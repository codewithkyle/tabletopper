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

// The owner's half of sharing a character sheet: the dialog that opens from the
// Share button in the editor's bar, and the two mutations behind its buttons.
// journal-share.go is the same three routes for one entry, share-form.go is
// what they have in common, and share.go is the reader's half of both.
//
// THE THREE ROUTES ARE THE JOURNAL'S THREE WITH ONE ID INSTEAD OF TWO, which is
// the whole difference the resource being a character makes here. A shared
// sheet's resource_id IS the character, so nothing in this file carries an
// entry, and the create's uniqueness -- one live link per character -- falls out
// of the same key the journal share uses.
//
// WHAT IT SHARES IS THE CHARACTER TAB, NOT THE CHARACTER. The dialog says so and
// share-character.go is what holds that line: no inventory beyond what is
// equipped, no spellbook beyond what is prepared, and no journal at all, since
// an entry is shared by its own link and gated by its own password.

// CharacterShareFragment is the dialog, opened from the Share button that every
// editor tab carries.
//
// IT LOADS NO CHARACTER, for the reason JournalShareFragment gives: owner_id
// goes into the query beside the id, so a request naming somebody else's
// character matches nothing and renders the form -- an offer to share a sheet
// that will then refuse to be shared, because the insert behind the form is
// scoped the same way. Confirming the character exists first would be a round
// trip to produce a different flavour of nothing.
//
// A bad id in the query string is a 404 with an empty body rather than
// htmx.NotFound: it came off the page's own markup, so a request carrying a
// broken one is a bug here rather than a reader who has lost a character.
func (a *App) CharacterShareFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, err := ulid.Parse(r.URL.Query().Get("character"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	data, err := a.characterShareDialog(ctx, r, characterID, sess.UserID)
	if err != nil {
		slog.Error("Failed to load character share", "error", err)
		htmx.ServerError(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.ShareDialog(data))
}

// CreateCharacterShare mints the link.
//
// THE INSERT IS THE OWNERSHIP CHECK. InsertCharacterShare selects owner_id and
// the id itself off the characters row rather than taking them from the request,
// so a character that is not this user's matches nothing, inserts nothing, and
// is read here as a 404 -- with no window between a check and a write for the
// character to be deleted in.
//
// THE TOKEN AND THE HASH ARE MADE BEFORE THE STATEMENT RUNS and thrown away if
// it matches nothing. That is the right way round: both are cheap next to a
// round trip, and computing them after would mean holding a row open across
// bcrypt's sixty milliseconds.
func (a *App) CreateCharacterShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
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

	params := queries.InsertCharacterShareParams{
		ID:          ulid.Make(),
		Token:       token,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
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

	if _, err := a.Queries.InsertCharacterShare(ctx, params); err != nil {
		// THE LIKELY FAILURE IS THE UNIQUE KEY, which is the character already
		// having a link -- a dialog left open on another tab of the same
		// editor, which is easier to arrive at here than on an entry, since
		// this button is on all five. The answer is the link that already
		// exists rather than an error about a constraint, so the row is read
		// back before anything is reported. Finding one means somebody won the
		// race and the reader gets what they asked for; finding none means this
		// was a real failure and it is answered as one.
		if data, readErr := a.characterShareDialog(ctx, r, characterID, sess.UserID); readErr == nil && data.Link != "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			render(w, r, pages.ShareDialog(data))
			return
		}

		slog.Error("Failed to create character share", "error", err)
		htmx.ServerError(w)
		return
	}

	// Read back rather than built from params, so the dialog has exactly one
	// definition of what it shows -- and so zero rows inserted, which is a
	// character that is not this user's, is found here as an absent link.
	data, err := a.characterShareDialog(ctx, r, characterID, sess.UserID)
	if err != nil {
		slog.Error("Failed to load character share after creating it", "error", err)
		htmx.ServerError(w)
		return
	}
	if data.Link == "" {
		htmx.NotFound(w, "character")
		return
	}

	htmx.Toast(w, "Share link created.")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.ShareDialog(data))
}

// RevokeCharacterShare deletes the row, which is the whole of revoking: the link
// stops working because there is nothing left for GetShareByToken to find, and
// any browser holding an unlock cookie for it is holding a proof of a hash that
// no longer exists.
//
// IT TAKES THE SHEET'S LINK AND NOTHING ELSE. DeleteCharacterShare pins
// resource_type, so every journal link this character handed out is still live
// afterwards -- revoking the sheet is not revoking the diary.
//
// It answers with the form, so the dialog is immediately ready to mint a new
// link -- and 200 rather than 204, like every other delete in the app: the
// noSwap config lists 204, and a status in that list would stop the swap that
// puts the form back.
func (a *App) RevokeCharacterShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}

	result, err := a.Queries.DeleteCharacterShare(ctx, queries.DeleteCharacterShareParams{
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to revoke character share", "error", err)
		htmx.ServerError(w)
		return
	}
	if deleted, err := result.RowsAffected(); err == nil && deleted == 0 {
		htmx.NotFound(w, "share link")
		return
	}

	htmx.Toast(w, "Share link revoked.")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.ShareDialog(characterShareDialogData(characterID)))
}

// characterShareDialogData is the dialog with no row behind it: the three
// strings that say which share this is, and nothing about whether it exists yet.
//
// THE BLURB IS WHERE THE SCOPE IS WRITTEN DOWN FOR THE PERSON DECIDING. What the
// link shows is a decision made in share-character.go, and an owner handing the
// URL to a table of strangers is entitled to know it before they paste it --
// particularly that the journal is not in it, which is the one thing they might
// reasonably assume goes along with a character.
func characterShareDialogData(characterID ulid.ULID) pages.ShareDialogData {
	return pages.ShareDialogData{
		Heading: "Share this character",
		Blurb: "Anyone with the link can read this character sheet, and it stays in step with the sheet as you edit it. " +
			"Journal entries are not included, and neither is the full inventory or spellbook -- only what is equipped and prepared.",
		Action: "/characters/" + characterID.String() + "/share",
	}
}

// characterShareDialog reads whichever state the dialog is in. No row is the
// form state, which is characterShareDialogData unchanged, so the miss needs no
// special handling anywhere it is called.
func (a *App) characterShareDialog(ctx context.Context, r *http.Request, characterID, ownerID ulid.ULID) (pages.ShareDialogData, error) {
	data := characterShareDialogData(characterID)

	row, err := a.Queries.GetCharacterShare(ctx, queries.GetCharacterShareParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return data, nil
	}
	if err != nil {
		return data, err
	}

	return describeShare(ctx, r, data, row), nil
}
