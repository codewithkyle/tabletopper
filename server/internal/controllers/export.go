package controllers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"tabletopper/internal/export"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/share"

	"github.com/oklog/ulid/v2"
)

// Downloading a monster or a character as Markdown, from four places: the two
// editors, and the two shared pages. internal/export is what writes the file;
// this is what decides whose it is and hands it over.
//
// THE FOUR ROUTES PRODUCE THREE BYTE-IDENTICAL PAIRS OF FILES, and that is the
// whole shape of this file. An owner exporting their own monster and a stranger
// exporting the same monster from a link go through one function with different
// ids -- so there is no version of the export that shows more to one of them,
// and no second definition of what a monster is in Markdown to drift.
//
// A MARKDOWN EXPORT IS A REPRESENTATION AND NOT A FRAGMENT. It is a GET that
// answers with a whole document rather than a piece of a page, so it keeps the
// resource's own URL: /monsters/{id}/export.md sits beside /monsters/{id}/edit
// the way /assets/maps/{id}/tiles/... sits beside the map. The extension is
// there because it is the thing a browser, an editor and a vault all read to
// decide what the file is.
//
// THE SHARED EXPORT IS BEHIND THE SAME PASSWORD THE PAGE IS. A link somebody was
// handed but never unlocked would otherwise give up the whole sheet as a file --
// the same hole the import would have been, and the same answer.
//
// NOTHING HERE IS SESSION-SHAPED. The reply is a file with a filename, not a
// swap: no htmx, no toast, no alert dialog. A failure the reader can do
// something about is a redirect to the page they came from, and one they cannot
// is the error page -- which is why every miss below answers with a navigation.

// ExportMonster is the button on the monster editor's bar: the owner taking
// their own stat block without sharing it first.
func (a *App) ExportMonster(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	monsterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		redirect(w, r, "/monsters")
		return
	}

	file, name, err := a.monsterMarkdown(ctx, monsterID, sess.UserID)
	// The read is owner-scoped, so a monster that is not this user's and one
	// that never existed are the same miss -- and the manual is where somebody
	// looking for a monster they cannot find should end up.
	if errors.Is(err, sql.ErrNoRows) {
		redirect(w, r, "/monsters")
		return
	}
	if err != nil {
		slog.Error("Failed to export monster", "error", err, "monsterID", monsterID.String())
		redirectToError(w, r)
		return
	}

	writeMarkdown(w, file, export.Filename(name, "monster"))
}

// ExportCharacter is the same button on the character editor's bar, and it is on
// the bar rather than on a tab for the reason the Share button is: it belongs to
// the character rather than to any one page of it.
func (a *App) ExportCharacter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		redirect(w, r, "/characters")
		return
	}

	file, name, err := a.characterMarkdown(ctx, characterID, sess.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		redirect(w, r, "/characters")
		return
	}
	if err != nil {
		slog.Error("Failed to export character", "error", err, "characterID", characterID.String())
		redirectToError(w, r)
		return
	}

	writeMarkdown(w, file, export.Filename(name, "character"))
}

// ExportShare is the button on a shared page, and it is one route for both kinds
// of share for the reason the portrait route is one route: the token names a row
// and the row says what it opens as. There is nothing in the path that could be
// edited into asking for the other.
//
// A JOURNAL SHARE IS REFUSED RATHER THAN LEFT TO MISS. An entry is already
// Markdown -- it is stored as the writer typed it -- so exporting one would be
// easy and is still wrong here: its pictures are share-scoped URLs that resolve
// for nobody once the file is in a vault, and a downloaded entry full of broken
// images is a worse artifact than no download. The shared entry page draws no
// button for this, so only a hand-built request arrives.
func (a *App) ExportShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	token := r.PathValue("token")
	if !share.ValidToken(token) {
		shareUnavailable(w, r)
		return
	}

	grant, err := a.Queries.GetShareByToken(ctx, token)
	if errors.Is(err, sql.ErrNoRows) {
		shareUnavailable(w, r)
		return
	}
	if err != nil {
		slog.Error("Failed to load share for export", "error", err)
		redirectToError(w, r)
		return
	}

	if grant.PasswordHash.Valid && !share.Unlocked(r, token, grant.PasswordHash.String) {
		shareUnavailable(w, r)
		return
	}

	var file []byte
	var name, fallback string

	switch grant.ResourceType {
	case queries.SharesResourceTypeMonster:
		fallback = "monster"
		file, name, err = a.monsterMarkdown(ctx, grant.ResourceID, grant.OwnerID)
	case queries.SharesResourceTypeCharacter:
		fallback = "character"
		file, name, err = a.characterMarkdown(ctx, grant.ResourceID, grant.OwnerID)
	default:
		shareUnavailable(w, r)
		return
	}

	// The thing was deleted after the link went out, which is the dead link
	// every other miss on a shared page is.
	if errors.Is(err, sql.ErrNoRows) {
		shareUnavailable(w, r)
		return
	}
	if err != nil {
		slog.Error("Failed to export a shared page", "error", err, "type", grant.ResourceType)
		redirectToError(w, r)
		return
	}

	writeMarkdown(w, file, export.Filename(name, fallback))
}

// monsterMarkdown is the file and the name to save it under. It is the editor's
// own two reads and the same block builder the page renders, so what comes out
// of here is what is on the screen.
func (a *App) monsterMarkdown(ctx context.Context, monsterID, ownerID ulid.ULID) ([]byte, string, error) {
	monster, err := a.Queries.GetMonster(ctx, queries.GetMonsterParams{ID: monsterID, OwnerID: ownerID})
	if err != nil {
		return nil, "", err
	}

	actions, err := a.Queries.ListMonsterActions(ctx, queries.ListMonsterActionsParams{
		MonsterID: monsterID,
		OwnerID:   ownerID,
	})
	if err != nil {
		return nil, "", err
	}

	block := monsterStatBlock(monster, actions, monsterDerived(monster, actions))

	return export.Monster(block, monster.Description), monster.Name, nil
}

// characterMarkdown is the same for a character, off the same loader the shared
// sheet is built by.
//
// WHAT IS EXPORTED IS THE SHARED SHEET AND NOT THE WHOLE ACCOUNT'S VIEW OF THE
// CHARACTER, for the owner as much as for a stranger. The scope was chosen for
// the shared page -- what is equipped rather than the whole pack, what is
// prepared rather than the whole spellbook, and no journal -- and it happens to
// be exactly the scope the export is for: a GM balancing tomorrow's fight wants
// what the party can bring to bear, not fifty feet of rope. One scope also means
// the file an owner exports and the file a player exports from their link are
// the same file, which is what makes it safe to say "use the link".
func (a *App) characterMarkdown(ctx context.Context, characterID, ownerID ulid.ULID) ([]byte, string, error) {
	sheet, _, err := a.loadCharacterSheet(ctx, characterID, ownerID)
	if err != nil {
		return nil, "", err
	}

	return export.Character(sheet), sheet.Header.Name, nil
}

// writeMarkdown is the reply all four routes make.
//
// Content-Disposition IS WHAT MAKES IT A DOWNLOAD rather than a page of plain
// text, and the filename in it is a slug -- see export.Filename -- so nothing a
// person typed can reach this header. A quote in a monster's name would
// otherwise end the parameter early and leave the rest of the name as syntax.
//
// no-store because two of the four routes are behind a session and a third is
// behind a password, and a shared cache holding one reader's answer would serve
// it to the next.
func writeMarkdown(w http.ResponseWriter, file []byte, filename string) {
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")

	if _, err := w.Write(file); err != nil {
		slog.Error("Failed to write a Markdown export", "error", err)
	}
}
