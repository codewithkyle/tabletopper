package controllers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/share"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

// What a shared monster link opens, and the one thing on any shared page that
// writes: the copy a signed-in reader takes into their own manual. share.go owns
// the token, the password and the miss; monster-share.go is the owner minting
// the link; this file is the monster and the import.
//
// IT IS THE WHOLE MONSTER, AND THAT IS THE DIFFERENCE FROM A SHARED SHEET. A
// character's link is deliberately narrow -- the editor has five tabs and the
// link hands over one of them -- because there is a journal behind it with its
// own links and its own passwords. A monster is one screen: the stat block, and
// the GM's own paragraph under it. Nothing is held back, because the import
// hands over a copy of all of it anyway, and a page that showed less than the
// button gives would be lying about what the button does.
//
// THE IMPORT IS THE ONLY WRITE ANY VISITOR CAN CAUSE, and every id it uses comes
// off the share row: the monster copied is grant.ResourceID, read as
// grant.OwnerID, and the copy is written under the session's own user. There is
// no id in the request but the token, so a link cannot be edited into copying a
// monster it does not name -- and nothing the visitor sends reaches a column,
// because CopyMonster selects every value off the source row.
//
// A COPY IS NOT A SUBSCRIPTION. It is a row in the importer's manual from the
// moment it is made: the owner editing theirs afterwards changes nothing here,
// deleting theirs takes nothing away, and revoking the link takes nothing back.
// The dialog's blurb tells the owner that before they hand the URL out.

// errImportedMonsterGone is a copy whose source vanished between the page
// rendering and the button being pressed -- the owner deleted the monster, and
// the share row went with it. It is the shape every other dead link takes.
var errImportedMonsterGone = errors.New("the shared monster no longer exists")

// sharedMonster renders the page. It runs past the password gate in SharePage
// and takes the grant rather than re-reading it, so the question is asked
// exactly once per request.
//
// EVERY ID IT QUERIES WITH COMES OFF THE GRANT, and the two reads are the
// editor's own -- both are already scoped by monster and owner, which is exactly
// the pair a share row carries.
func (a *App) sharedMonster(w http.ResponseWriter, r *http.Request, token string, grant queries.GetShareByTokenRow) {
	ctx := r.Context()

	monster, err := a.Queries.GetMonster(ctx, queries.GetMonsterParams{
		ID:      grant.ResourceID,
		OwnerID: grant.OwnerID,
	})
	// The monster was deleted after the link went out. Deleting one takes its
	// share with it, so this is a race rather than a steady state -- and it
	// reads as a dead link, which is what it is.
	if errors.Is(err, sql.ErrNoRows) {
		shareUnavailable(w, r)
		return
	}
	if err != nil {
		slog.Error("Failed to load shared monster", "error", err)
		redirectToError(w, r)
		return
	}

	actions, err := a.Queries.ListMonsterActions(ctx, queries.ListMonsterActionsParams{
		MonsterID: grant.ResourceID,
		OwnerID:   grant.OwnerID,
	})
	if err != nil {
		slog.Error("Failed to load shared monster actions", "error", err)
		redirectToError(w, r)
		return
	}

	block := monsterStatBlock(monster, actions, monsterDerived(monster, actions))

	// monsterStatBlock points at /assets/images/{id}, which needs a session and
	// would render as a broken picture for every reader. The share's own
	// portrait route is where this page's one picture comes from.
	block.Image = ""
	if monster.AssetID != nil {
		block.Image = sharePortraitURL(token)
	}

	shareHeaders(w)
	render(w, r, pages.SharedMonsterPage(pages.SharedMonsterData{
		Block:       block,
		Description: strings.TrimSpace(monster.Description),
		Actions:     sharedMonsterActions(session.FromContext(ctx).UserID, token, grant.OwnerID),
	}))
}

// sharedMonsterActions is the row above the block, and the import in it is the
// one thing on any shared page that is not the same for everybody.
//
// THE EXPORT IS UNCONDITIONAL AND THE IMPORT IS NOT. The file holds what the
// page holds, so there is nobody who can read this monster and should not be
// able to keep a copy of the text -- while taking it into a manual is a write,
// and needs somewhere to write it.
//
// THE IMPORT'S THREE STATES ARE THE THREE KINDS OF READER. Somebody signed in is
// offered the copy; somebody signed out is told where the copy comes from,
// because a button that answered with a sign-in page would be a worse way to say
// the same thing; the owner is offered nothing, since importing their own
// monster would hand them a duplicate they did not ask for and would be
// indistinguishable from the button having failed.
//
// EVERY STATE SAYS SOMETHING, INCLUDING THE OWNER'S. A row with one button
// floating against its right edge and nothing beside it reads as a panel that
// failed to load the rest of itself, so the left half of it is always a
// sentence -- what the reader can do here, in the order they would do it.
//
// THE SENTENCES ARE WRITTEN HERE RATHER THAN IN THE MARKUP, which is where
// prose belongs in this app: a .templ is a Tailwind source, and an ordinary
// English word in one that happens to be a component name emits that
// component's whole family into the stylesheet.
func sharedMonsterActions(viewer ulid.ULID, token string, ownerID ulid.ULID) pages.SharedActions {
	actions := pages.SharedActions{Export: shareExportURL(token)}

	switch {
	case viewer.IsZero():
		actions.Blurb = "Sign in to add this monster to your own manual, as a full copy you can run and edit -- or export it as Markdown for your own notes."
		actions.SignIn = "/sign-in"
	case viewer == ownerID:
		actions.Blurb = "This monster is yours. Export it as Markdown for your own notes."
	default:
		actions.Blurb = "Add this monster to your own manual and you get a full copy to run and edit however you like -- or export it as Markdown for your own notes."
		actions.Import = "/share/" + token + "/import"
	}

	return actions
}

// ImportSharedMonster copies a shared monster into the reader's own manual and
// sends them to their copy of it.
//
// IT IS A PLAIN FORM POST ANSWERED WITH A 303, like the password gate beside it,
// because the share layout loads no JavaScript at all. It is also what makes a
// refresh of the page afterwards harmless: the browser lands on the editor,
// which is a GET, rather than on a repeat of this.
//
// THE PASSWORD GATES THE COPY AS WELL AS THE PAGE, and this is the request where
// that matters most. A link somebody was handed but never unlocked would
// otherwise give up the whole monster on one POST -- permanently, and without
// ever having rendered it.
//
// THE SESSION IS THE ONLY THING TAKEN FROM THE REQUEST. Everything else comes
// off the share row, so there is nothing here to point at another monster with.
func (a *App) ImportSharedMonster(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

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
		slog.Error("Failed to load share for import", "error", err)
		redirectToError(w, r)
		return
	}

	if grant.PasswordHash.Valid && !share.Unlocked(r, token, grant.PasswordHash.String) {
		shareUnavailable(w, r)
		return
	}

	// A journal entry and a character sheet have nothing to import, and a stale
	// form posting here is the only way to arrive: neither page draws a button
	// that points at this route. The dead-link page is the answer for the same
	// reason every other miss gets it -- telling them apart would tell somebody
	// guessing which half of the guess was right.
	if grant.ResourceType != queries.SharesResourceTypeMonster {
		shareUnavailable(w, r)
		return
	}

	// The owner pressing their own button, which the page does not draw for
	// them. Their manual already has this monster, so the honest answer is the
	// one they were reaching for rather than a second copy of it.
	if sess.UserID == grant.OwnerID {
		redirect(w, r, "/monsters/"+grant.ResourceID.String()+"/edit")
		return
	}

	monsterID, err := a.importMonster(ctx, grant, sess.UserID)
	if errors.Is(err, errImportedMonsterGone) {
		shareUnavailable(w, r)
		return
	}
	if err != nil {
		slog.Error("Failed to import a shared monster", "error", err, "monsterID", grant.ResourceID.String())
		redirectToError(w, r)
		return
	}

	// THE PICTURE IS NOT WORTH THROWING THE COPY AWAY OVER, which is the same
	// call NewMonsterForm makes about the picture its dialog carried: the
	// monster is complete without it, the editor this redirect lands on carries
	// the upload control, and a reader who has just been sent to a stat block
	// they now own does not want to be told it failed because of a thumbnail.
	if err := a.copyMonsterPicture(ctx, grant, sess.UserID, monsterID); err != nil {
		slog.Error("Failed to copy an imported monster's picture", "error", err, "monsterID", monsterID.String())
	}

	redirect(w, r, "/monsters/"+monsterID.String()+"/edit")
}

// importMonster writes the row and its stat block rows, as one transaction.
//
// A HALF-COPIED MONSTER IS WORSE THAN NO COPY AT ALL, because nothing about it
// says so: it would sit in the reader's manual with its name, its armour class
// and none of its actions, looking exactly like a monster whose owner never
// wrote any. That used to be prevented by a compensating delete -- copy the
// monster, copy its actions, and if the second half failed run the monster
// purge over what the first half wrote. The transaction is the same guarantee
// with nothing to keep in step: a table added to the stat block is covered
// because the database is what undoes the write, not a list of statements
// somebody has to remember to extend.
//
// The picture is copied by the caller, after this returns, and is deliberately
// not part of it -- an object cannot be rolled back, and a monster with no
// picture is a monster, while a monster with no actions is a mistake.
func (a *App) importMonster(ctx context.Context, grant queries.GetShareByTokenRow, ownerID ulid.ULID) (ulid.ULID, error) {
	monsterID := ulid.Make()

	err := a.tx(ctx, func(q *queries.Queries) error {
		return copyMonster(ctx, q, grant, ownerID, monsterID)
	})
	if err != nil {
		return ulid.ULID{}, err
	}

	return monsterID, nil
}

// copyMonster is the two writes an import is, over whatever Queries it is
// given. It is separate from the transaction around it so that a test can drive
// the statements without one.
func copyMonster(ctx context.Context, q *queries.Queries, grant queries.GetShareByTokenRow, ownerID, monsterID ulid.ULID) error {
	result, err := q.CopyMonster(ctx, queries.CopyMonsterParams{
		ID:            monsterID,
		OwnerID:       ownerID,
		SourceID:      grant.ResourceID,
		SourceOwnerID: grant.OwnerID,
	})
	if err != nil {
		return fmt.Errorf("monster: %w", err)
	}
	// The SELECT half matched nothing, which is the monster having been deleted
	// between the page and the button. It is the only thing zero rows can mean:
	// the id was freshly minted, so a duplicate key is not on the table.
	//
	// Returning an error rolls the transaction back, which is what should
	// happen: there is nothing to keep, and the row the INSERT did not write is
	// not there to remove either.
	if copied, err := result.RowsAffected(); err == nil && copied == 0 {
		return errImportedMonsterGone
	}

	if err := copyMonsterActions(ctx, q, grant, ownerID, monsterID); err != nil {
		return fmt.Errorf("actions: %w", err)
	}

	return nil
}

// copyMonsterActions copies the seven sections a row at a time, because a fresh
// ULID cannot be minted inside a statement -- and the ids are not incidental.
//
// THEY ARE MINTED IN ORDER AND THAT IS THE COPY'S ORDER. ListMonsterActions
// sorts by kind then id, so id is what puts Multiattack in front of Bite; and
// ulid.Make takes fresh entropy on every call, which means two ids minted inside
// the same millisecond -- all of them, for a stat block this size -- sort in
// whatever order the random bytes happened to fall. ulid.Monotonic is the reader
// that guarantees each id is greater than the one before it within a
// millisecond, so the copy prints in the order the original does.
func copyMonsterActions(ctx context.Context, q *queries.Queries, grant queries.GetShareByTokenRow, ownerID, monsterID ulid.ULID) error {
	rows, err := q.ListMonsterActions(ctx, queries.ListMonsterActionsParams{
		MonsterID: grant.ResourceID,
		OwnerID:   grant.OwnerID,
	})
	if err != nil {
		return fmt.Errorf("reading the original: %w", err)
	}

	entropy := ulid.Monotonic(rand.Reader, 0)
	for _, row := range rows {
		id, err := ulid.New(ulid.Timestamp(time.Now()), entropy)
		if err != nil {
			return fmt.Errorf("minting an id: %w", err)
		}

		err = q.CopyMonsterAction(ctx, queries.CopyMonsterActionParams{
			ID:          id,
			OwnerID:     ownerID,
			MonsterID:   monsterID,
			Kind:        row.Kind,
			Name:        row.Name,
			Description: row.Description,
		})
		if err != nil {
			return fmt.Errorf("writing the copy: %w", err)
		}
	}

	return nil
}

// copyMonsterPicture gives the copy its own object rather than a pointer at the
// original's.
//
// THE COPY CANNOT SHARE THE ORIGINAL'S ASSET, which is why CopyMonster leaves
// asset_id out. The object lives under the source owner's own prefix and its row
// is theirs: deleting their monster deletes both, and a row in this account
// pointing there would render a broken picture from that moment on with nothing
// able to fix it.
//
// The bytes are read back and written through attachMonsterImage, the same
// function an upload goes through, so the copy's row is written before its
// object and rolled back if the object fails -- the ledger property every
// monster picture in the bucket has.
//
// NOTHING IS RE-ENCODED. What is being copied is already this app's own output:
// a square WebP at monsterImageSize, put there by the encoder every upload runs
// through. Decoding it to encode it again would cost a megabyte of image for a
// byte-identical result.
func (a *App) copyMonsterPicture(ctx context.Context, grant queries.GetShareByTokenRow, ownerID, monsterID ulid.ULID) error {
	picture, err := a.Queries.GetSharedMonsterImage(ctx, queries.GetSharedMonsterImageParams{
		MonsterID: grant.ResourceID,
		OwnerID:   grant.OwnerID,
	})
	// An INNER JOIN with no row is a monster with no picture, which is most of
	// them and is not a failure.
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("the original's asset row: %w", err)
	}

	body, _, err := a.Storage.Get(ctx, picture.FilePath)
	if err != nil {
		return fmt.Errorf("reading the object: %w", err)
	}
	defer body.Close()

	// maxUploadBytes is the cap on anything this process holds as an image, and
	// it is orders of magnitude above what is actually there -- a 256-pixel WebP
	// is a few kilobytes. It is here because the size comes off a bucket rather
	// than off a request, and a read with no ceiling is one bad row away from
	// being the whole heap.
	encoded, err := io.ReadAll(io.LimitReader(body, maxUploadBytes))
	if err != nil {
		return fmt.Errorf("reading the object: %w", err)
	}

	if _, err := a.attachMonsterImage(ctx, ownerID, monsterID, encoded, picture.FileName); err != nil {
		return fmt.Errorf("attaching the copy: %w", err)
	}

	return nil
}
