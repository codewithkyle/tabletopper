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

// The owner's half of sharing: the dialog that opens over the entry editor, and
// the two mutations behind its buttons. The reader's half is share.go, and the
// two files share nothing but the row.
//
// ALL THREE ANSWER WITH THE DIALOG IN WHATEVER STATE THE ENTRY IS NOW IN, which
// is why there is one component and not two. Opening it renders the form or the
// link depending on what is there; creating renders the link it just made;
// revoking renders the form again, so an owner who revoked by mistake can make
// a new link without closing anything.
//
// THE CREATE IS A POST TO THE ENTRY'S OWN URL AND NOT TO /fragment/. It returns
// partial HTML, which is the exception the fragment rules name -- a mutation
// answering with the thing it just made -- and the alternative is a POST that
// returns nothing followed by a GET to fetch the link it created. The prefix
// marks GET-shaped representations; this is not one.
//
// REVOKING IS NOT GATED BY THE CONFIRM DIALOG, and that is a decision rather
// than an omission. hx-confirm would open #confirm-modal on top of the content
// modal the button is sitting in, which is two <dialog>s in the top layer for a
// loss that takes one click to undo: revoking mints nothing, destroys nothing
// the entry needs, and the next button in the same dialog makes a new link.

// JournalShareFragment is the dialog, opened from the Share button in the entry
// editor's header.
//
// IT LOADS NO CHARACTER AND NO ENTRY, for the reason JournalEntriesFragment
// gives: owner_id goes into the query beside the other two ids, so a request
// naming somebody else's entry matches nothing and renders the form -- an offer
// to share an entry that will then refuse to be shared, because the insert
// behind the form is scoped the same way. Confirming the entry exists first
// would be a round trip to produce a different flavour of nothing.
//
// A bad id in the query string is a 404 with an empty body rather than
// htmx.NotFound: both ids came off the page's own markup, so a request carrying
// a broken one is a bug here rather than a reader who has lost an entry.
func (a *App) JournalShareFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	params := r.URL.Query()

	characterID, err := ulid.Parse(params.Get("character"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	entryID, err := ulid.Parse(params.Get("entry"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	data, err := a.journalShareDialog(ctx, r, characterID, entryID, sess.UserID)
	if err != nil {
		slog.Error("Failed to load journal share", "error", err)
		htmx.ServerError(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.ShareDialog(data))
}

// CreateJournalShare mints the link.
//
// THE INSERT IS THE OWNERSHIP CHECK. InsertJournalShare selects owner_id and
// character_id off the journals row rather than taking them from the request,
// so an entry that is not this user's matches nothing, inserts nothing, and is
// read here as a 404 -- with no window between a check and a write for the
// entry to be deleted in.
//
// THE TOKEN AND THE HASH ARE MADE BEFORE THE STATEMENT RUNS and thrown away if
// it matches nothing. That is the right way round: both are cheap next to a
// round trip, and computing them after would mean holding a row open across
// bcrypt's sixty milliseconds.
func (a *App) CreateJournalShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	entryID, ok := journalEntryID(w, r)
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

	params := queries.InsertJournalShareParams{
		ID:          ulid.Make(),
		Token:       token,
		EntryID:     entryID,
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

	if _, err := a.Queries.InsertJournalShare(ctx, params); err != nil {
		// THE LIKELY FAILURE IS THE UNIQUE KEY, which is the entry already
		// having a link -- a dialog left open in another tab, or a double
		// click. The answer to that is the link that already exists rather
		// than an error about a constraint, so the row is read back before
		// anything is reported. Finding one means somebody won the race and
		// the reader gets what they asked for; finding none means this was a
		// real failure and it is answered as one.
		if data, readErr := a.journalShareDialog(ctx, r, characterID, entryID, sess.UserID); readErr == nil && data.Link != "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			render(w, r, pages.ShareDialog(data))
			return
		}

		slog.Error("Failed to create journal share", "error", err)
		htmx.ServerError(w)
		return
	}

	// Read back rather than built from params, so the dialog has exactly one
	// definition of what it shows -- and so zero rows inserted, which is an
	// entry that is not this user's, is found here as an absent link.
	data, err := a.journalShareDialog(ctx, r, characterID, entryID, sess.UserID)
	if err != nil {
		slog.Error("Failed to load journal share after creating it", "error", err)
		htmx.ServerError(w)
		return
	}
	if data.Link == "" {
		htmx.NotFound(w, "journal entry")
		return
	}

	htmx.Toast(w, "Share link created.")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.ShareDialog(data))
}

// RevokeJournalShare deletes the row, which is the whole of revoking: the link
// stops working because there is nothing left for GetShareByToken to find, and
// any browser holding an unlock cookie for it is holding a proof of a hash that
// no longer exists.
//
// It answers with the form, so the dialog is immediately ready to mint a new
// link -- and 200 rather than 204, like every other delete in the app: the
// noSwap config lists 204, and a status in that list would stop the swap that
// puts the form back.
func (a *App) RevokeJournalShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	entryID, ok := journalEntryID(w, r)
	if !ok {
		return
	}

	result, err := a.Queries.DeleteJournalShare(ctx, queries.DeleteJournalShareParams{
		EntryID:     entryID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to revoke journal share", "error", err)
		htmx.ServerError(w)
		return
	}
	if deleted, err := result.RowsAffected(); err == nil && deleted == 0 {
		htmx.NotFound(w, "share link")
		return
	}

	htmx.Toast(w, "Share link revoked.")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.ShareDialog(journalShareDialogData(characterID, entryID)))
}

// journalShareDialogData is the dialog with no row behind it: the three strings
// that say which share this is, and nothing about whether it exists yet. Every
// state starts from here -- the form is exactly this, and the link state is this
// with what the row says added -- so the heading, the sentence and the URL are
// written once for all three routes.
func journalShareDialogData(characterID, entryID ulid.ULID) pages.ShareDialogData {
	return pages.ShareDialogData{
		Heading: "Share this entry",
		Blurb:   "Anyone with the link can read this entry. It stays in step with the entry, so later edits show up for them too.",
		Action:  "/characters/" + characterID.String() + "/journal/" + entryID.String() + "/share",
	}
}

// journalShareDialog reads whichever state the dialog is in. No row is the form
// state, which is journalShareDialogData unchanged, so the miss needs no special
// handling anywhere it is called.
func (a *App) journalShareDialog(ctx context.Context, r *http.Request, characterID, entryID, ownerID ulid.ULID) (pages.ShareDialogData, error) {
	data := journalShareDialogData(characterID, entryID)

	row, err := a.Queries.GetJournalShare(ctx, queries.GetJournalShareParams{
		EntryID:     entryID,
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
