package controllers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/share"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

// The owner's half of sharing a monster: the dialog that opens from the Share
// button in the editor's bar, and the two mutations behind its buttons.
// character-share.go is the same three routes for a sheet, journal-share.go for
// an entry, share-form.go is what all three have in common, and share.go is the
// reader's half of every one of them.
//
// THE THREE ROUTES ARE THE CHARACTER'S THREE WITH A MONSTER UNDERNEATH THEM.
// That is the whole difference the resource makes here: one id, off a different
// table, and a create whose uniqueness -- one live link per monster -- falls out
// of the same key the other two use.
//
// WHAT IT SHARES IS THE WHOLE MONSTER, which is where it stops resembling the
// character's dialog. A sheet's link is deliberately narrow, because a sheet has
// four other tabs and a journal behind it; a monster is one screen and the link
// hands over that screen. The blurb says so, and it says the other thing an
// owner is entitled to know before pasting the URL into a chat window: anybody
// signed in who opens it can take a copy.

// MonsterShareFragment is the dialog, opened from the Share button in the
// editor's bar.
//
// IT LOADS NO MONSTER, for the reason CharacterShareFragment gives: owner_id
// goes into the query beside the id, so a request naming somebody else's
// monster matches nothing and renders the form -- an offer to share a monster
// that will then refuse to be shared, because the insert behind the form is
// scoped the same way. Confirming the monster exists first would be a round trip
// to produce a different flavour of nothing.
//
// A bad id in the query string is a 404 with an empty body rather than
// htmx.NotFound: it came off the page's own markup, so a request carrying a
// broken one is a bug here rather than a reader who has lost a monster.
func (a *App) MonsterShareFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	monsterID, err := ulid.Parse(r.URL.Query().Get("monster"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	data, err := a.monsterShareDialog(ctx, r, monsterID, sess.UserID)
	if err != nil {
		slog.Error("Failed to load monster share", "error", err)
		htmx.ServerError(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.ShareDialog(data))
}

// CreateMonsterShare mints the link.
//
// THE INSERT IS THE OWNERSHIP CHECK. InsertMonsterShare selects owner_id and the
// id itself off the monsters row rather than taking them from the request, so a
// monster that is not this user's matches nothing, inserts nothing, and is read
// here as a 404 -- with no window between a check and a write for the monster to
// be deleted in.
//
// THE TOKEN AND THE HASH ARE MADE BEFORE THE STATEMENT RUNS and thrown away if
// it matches nothing, for the reason the character's create gives: both are
// cheap next to a round trip, and computing them after would mean holding a row
// open across bcrypt's sixty milliseconds.
func (a *App) CreateMonsterShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	monsterID, ok := panelMonsterID(w, r)
	if !ok {
		return
	}

	if !parsePanelForm(w, r, pages.ShareDialogPanel) {
		return
	}

	input, problems := buildShareInput(r)
	if len(problems) > 0 {
		renderPanelBlock(w, r, pages.ShareDialogPanel, problems)
		return
	}

	token, err := share.NewToken()
	if err != nil {
		slog.Error("Failed to mint a share token", "error", err)
		htmx.ServerError(w)
		return
	}

	params := queries.InsertMonsterShareParams{
		ID:        ulid.Make(),
		Token:     token,
		MonsterID: monsterID,
		OwnerID:   sess.UserID,
	}
	if input.Password != "" {
		hash, err := share.HashPassword(input.Password)
		if err != nil {
			slog.Error("Failed to hash a share password", "error", err)
			htmx.ServerError(w)
			return
		}
		params.PasswordHash = sql.NullString{String: hash, Valid: true}
	}
	if input.Days > 0 {
		params.ExpiresAt = sql.NullTime{
			Time:  time.Now().Add(time.Duration(input.Days) * 24 * time.Hour),
			Valid: true,
		}
	}

	if _, err := a.Queries.InsertMonsterShare(ctx, params); err != nil {
		// THE LIKELY FAILURE IS THE UNIQUE KEY, which is the monster already
		// having a link -- the same dialog left open in another tab. The answer
		// is the link that already exists rather than an error about a
		// constraint, so the row is read back before anything is reported.
		// Finding one means somebody won the race and the reader gets what they
		// asked for; finding none means this was a real failure and it is
		// answered as one.
		if data, readErr := a.monsterShareDialog(ctx, r, monsterID, sess.UserID); readErr == nil && data.Link != "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			render(w, r, pages.ShareDialog(data))
			return
		}

		slog.Error("Failed to create monster share", "error", err)
		htmx.ServerError(w)
		return
	}

	// Read back rather than built from params, so the dialog has exactly one
	// definition of what it shows -- and so zero rows inserted, which is a
	// monster that is not this user's, is found here as an absent link.
	data, err := a.monsterShareDialog(ctx, r, monsterID, sess.UserID)
	if err != nil {
		slog.Error("Failed to load monster share after creating it", "error", err)
		htmx.ServerError(w)
		return
	}
	if data.Link == "" {
		htmx.NotFound(w, "monster")
		return
	}

	htmx.Toast(w, "Share link created.")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.ShareDialog(data))
}

// RevokeMonsterShare deletes the row, which is the whole of revoking: the link
// stops working because there is nothing left for GetShareByToken to find, and
// any browser holding an unlock cookie for it is holding a proof of a hash that
// no longer exists.
//
// COPIES ALREADY TAKEN ARE NOT REVOKED AND CANNOT BE. An import is a row in
// somebody else's manual from the moment it is made, and this deletes a link
// rather than reaching into another account -- which is what the dialog's blurb
// tells the owner before they hand the URL out.
//
// It answers with the form, so the dialog is immediately ready to mint a new
// link -- and 200 rather than 204, like every other delete in the app: the
// noSwap config lists 204, and a status in that list would stop the swap that
// puts the form back.
func (a *App) RevokeMonsterShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	monsterID, ok := panelMonsterID(w, r)
	if !ok {
		return
	}

	result, err := a.Queries.DeleteMonsterShare(ctx, queries.DeleteMonsterShareParams{
		MonsterID: monsterID,
		OwnerID:   sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to revoke monster share", "error", err)
		htmx.ServerError(w)
		return
	}
	if deleted, err := result.RowsAffected(); err == nil && deleted == 0 {
		htmx.NotFound(w, "share link")
		return
	}

	htmx.Toast(w, "Share link revoked.")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.ShareDialog(monsterShareDialogData(monsterID)))
}

// monsterShareDialogData is the dialog with no row behind it: the three strings
// that say which share this is, and nothing about whether it exists yet.
//
// THE BLURB IS WHERE THE SCOPE IS WRITTEN DOWN FOR THE PERSON DECIDING, and a
// monster's has a second sentence the other two do not need. A shared sheet can
// only be read; a shared monster can be taken, and an owner about to paste the
// URL into a table's chat is entitled to know that before they do rather than
// after.
func monsterShareDialogData(monsterID ulid.ULID) pages.ShareDialogData {
	return pages.ShareDialogData{
		Heading: "Share this monster",
		Blurb: "Anyone with the link can read this monster's full stat block, and it stays in step with it as you edit. " +
			"Anyone signed in can also copy it into their own manual, and a copy is theirs -- revoking the link later does not take it back.",
		Action: "/monsters/" + monsterID.String() + "/share",
	}
}

// monsterShareDialog reads whichever state the dialog is in. No row is the form
// state, which is monsterShareDialogData unchanged, so the miss needs no special
// handling anywhere it is called.
func (a *App) monsterShareDialog(ctx context.Context, r *http.Request, monsterID, ownerID ulid.ULID) (pages.ShareDialogData, error) {
	data := monsterShareDialogData(monsterID)

	row, err := a.Queries.GetMonsterShare(ctx, queries.GetMonsterShareParams{
		MonsterID: monsterID,
		OwnerID:   ownerID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return data, nil
	}
	if err != nil {
		return data, err
	}

	return describeShare(ctx, r, data, row), nil
}
