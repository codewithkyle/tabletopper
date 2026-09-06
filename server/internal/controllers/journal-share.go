package controllers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
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
const (
	// shareMinDays and shareMaxDays bound the expiry the form collects. The
	// floor is a day because the box counts in days and zero of them is a link
	// that is dead before it is pasted; the ceiling is a year because a share
	// meant to outlast one is a share with no expiry, which the toggle already
	// offers.
	shareMinDays = 1
	shareMaxDays = 365
)

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
	render(w, r, pages.JournalShareFragment(data))
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

	if !parsePanelForm(w, r, pages.JournalSharePanel) {
		return
	}

	input, problems := buildShareInput(r)
	if len(problems) > 0 {
		renderPanelBlock(w, r, pages.JournalSharePanel, problems)
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
			render(w, r, pages.JournalShareFragment(data))
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
	render(w, r, pages.JournalShareFragment(data))
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
	render(w, r, pages.JournalShareFragment(pages.JournalShareData{
		CharacterID: characterID.String(),
		EntryID:     entryID.String(),
	}))
}

// journalShareDialog reads whichever state the dialog is in. No row is the form
// state, which is the zero value plus the two ids, so the miss needs no special
// handling anywhere it is called.
func (a *App) journalShareDialog(ctx context.Context, r *http.Request, characterID, entryID, ownerID ulid.ULID) (pages.JournalShareData, error) {
	data := pages.JournalShareData{
		CharacterID: characterID.String(),
		EntryID:     entryID.String(),
	}

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

	data.Link = shareLink(r, row.Token)
	data.Protected = row.PasswordHash.Valid
	if row.ExpiresAt.Valid {
		data.Expires = journalTimestamp(row.ExpiresAt.Time)
		// GetJournalShare deliberately does not filter on the expiry -- an
		// expired share is still a row its owner has to be shown -- so the
		// comparison the query skipped is made here, once, for the sentence
		// the dialog prints.
		data.Expired = !row.ExpiresAt.Time.After(time.Now())
	}

	return data, nil
}

// shareInput is a validated create request: how many days until the link dies,
// zero for never, and the password, empty for none. Both toggles collapse into
// their field's zero value, so nothing downstream has to consult a flag and a
// value that could disagree.
type shareInput struct {
	Days     int
	Password string
}

// buildShareInput reads the dialog's two toggles and the field under each.
//
// A TOGGLE THAT IS OFF DISCARDS THE FIELD BESIDE IT WITHOUT LOOKING AT IT. The
// days box always posts a value -- it holds a default so that flipping the
// toggle is a complete answer -- so reading it regardless would give every link
// an expiry nobody asked for.
func buildShareInput(r *http.Request) (shareInput, []string) {
	var input shareInput
	var problems []string

	if r.PostFormValue("expiry") != "" {
		days, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue("days")))
		if err != nil || days < shareMinDays || days > shareMaxDays {
			problems = append(problems, "Choose between 1 and 365 days, or turn the expiry off.")
		} else {
			input.Days = days
		}
	}

	if r.PostFormValue("protect") != "" {
		// NOT TRIMMED. A password is a secret rather than a name, and a space
		// at either end of it is a character the person who chose it typed --
		// trimming here would silently store something they could not then
		// type back in.
		password := r.PostFormValue("password")
		switch {
		case len(password) < share.PasswordMin:
			problems = append(problems, "A password must be at least 6 characters.")
		case len(password) > share.PasswordMax:
			problems = append(problems, "A password must be 72 characters or fewer.")
		default:
			input.Password = password
		}
	}

	return input, problems
}

// shareLink is the absolute URL of a share, which is what the dialog puts in
// front of someone to copy. It is the one URL in the app built with a scheme
// and a host, because it is the only one that leaves the app: everything else
// is a path the browser resolves against the page it is already on.
//
// THE HOST COMES FROM THE REQUEST, which is the only thing that knows it. There
// is no configured base URL, and adding one would be a variable to keep in step
// with wherever this is deployed for a value the request already carries.
//
// The scheme is the header the proxy in front sets, and only two values are
// accepted from it -- it is a client-supplied header when nothing is in front,
// and the worst a forged one can do is make a copied link say https where the
// deployment is plain http. Falling back to the connection is what makes local
// development produce http://localhost:3000 without configuring anything.
func shareLink(r *http.Request, token string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded == "http" || forwarded == "https" {
		scheme = forwarded
	}

	return scheme + "://" + r.Host + "/share/" + token
}
