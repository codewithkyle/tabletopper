package controllers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

// The journal tab. An entry is a row of its own, like an inventory item, and
// the editor page saves it on a debounce the way every panel on the sheet does.
//
// TWO OF THESE FIVE ANSWER A BROWSER AND THREE ANSWER HTMX, and the difference
// decides how a miss is reported. htmx.NotFound writes an HX-Trigger header and
// an empty 404 body, which raises the alert dialog for a request the page made
// and is a blank screen for a navigation. So the two page routes and the create
// post -- which is a plain form submission -- redirect, and only the save and
// the delete answer with htmx.NotFound.
//
// The limits are enforced here, in the units the columns count, because MySQL
// runs in strict mode: an overlong value comes back from the driver as an error
// and would reach the writer as a 500 on a field they were entitled to overfill.
// The title is measured in characters, which is what VARCHAR counts; the body is
// measured in bytes, which is what MEDIUMTEXT counts.
//
// 256 KB of body is roughly forty pages of prose. MEDIUMTEXT holds 16 MB and
// ParseForm takes 10 MB, so neither is the ceiling here -- the cap is low
// because the whole body is posted on every debounce, and a runaway paste should
// be refused with a message rather than shipped over the wire once a second.
//
// The search term is capped at the title's length for no deeper reason than
// that nothing longer is a search. The box carries the same number as a
// maxlength, so the cap is reachable only by a request nobody's browser made.
const (
	journalTitleLimit  = 255
	journalBodyLimit   = 262144
	journalSearchLimit = 255
)

func (a *App) CharacterJournalPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	_, characterID, ok := a.loadCharacter(w, r)
	if !ok {
		return
	}

	// The page always renders the whole list. A ?q= on it would have to come
	// from a bookmark, because the box never puts one there -- see the search
	// route for why the search stays out of the URL.
	entries, err := a.journalEntries(ctx, characterID, sess.UserID, "")
	if err != nil {
		slog.Error("Failed to load journal entries", "error", err)
		redirectToError(w, r)
		return
	}

	render(w, r, pages.EditCharacterJournal(pages.JournalPageData{
		CharacterID: characterID.String(),
		Entries:     entries,
	}))
}

// JournalEntriesFragment is the list under the search box, filtered by ?q=. It
// is a GET returning the same component the page renders, which is what the
// /fragment/ prefix promises.
//
// IT DOES NOT LOAD THE CHARACTER. Every other journal route does, because every
// other one needs the row or needs to redirect somewhere sensible without it.
// This one needs neither: owner_id goes into the query beside character_id, so
// an id belonging to somebody else matches nothing and comes back as an empty
// list. That reply is indistinguishable from an empty journal, which is the
// point -- it says nothing about whether the character exists. A GetCharacter
// before the search would be a second round trip to learn something the search
// already enforces.
//
// THE SEARCH IS NOT IN THE URL, and hx-push-url is deliberately absent from the
// box. htmx pushes on every swap, and the swaps here are on a 250ms debounce, so
// pushing would file a history entry per pause in typing and leave Back walking
// the term backwards a few characters at a time. A filter on a list is worth
// less than a working Back button.
//
// A bad character id is a 404 with an empty body, not htmx.NotFound: the id came
// off the page's own markup, so a request carrying a broken one is not a reader
// who has lost a character and has nothing to be told.
func (a *App) JournalEntriesFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	params := r.URL.Query()

	characterID, err := ulid.Parse(params.Get("character"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Trimmed, so a term someone is still typing a space into does not stop
	// matching, and so a box holding nothing but spaces is the whole list
	// rather than a search for a space.
	term := strings.TrimSpace(params.Get("q"))
	if len([]rune(term)) > journalSearchLimit {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	entries, err := a.journalEntries(ctx, characterID, sess.UserID, term)
	if err != nil {
		slog.Error("Failed to search journal entries", "error", err)
		htmx.ServerError(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.JournalEntriesFragment(pages.JournalPageData{
		CharacterID: characterID.String(),
		Entries:     entries,
		Query:       term,
	}))
}

// CharacterJournalEntryPage is the editor for one entry, and the only read in
// the app that carries a body.
//
// THE ENTRY ID IS PARSED BEFORE THE CHARACTER IS LOADED. A page whose last
// segment is not a ULID has no row to fetch no matter who owns the character, so
// asking the database first would be a query run to reach a redirect that was
// already decided.
func (a *App) CharacterJournalEntryPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	entryID, err := ulid.Parse(r.PathValue("entryId"))
	if err != nil {
		redirectToJournal(w, r)
		return
	}

	_, characterID, ok := a.loadCharacter(w, r)
	if !ok {
		return
	}

	entry, err := a.Queries.GetJournalEntry(ctx, queries.GetJournalEntryParams{
		ID:          entryID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		redirectToJournal(w, r)
		return
	}
	if err != nil {
		slog.Error("Failed to load journal entry", "error", err)
		redirectToError(w, r)
		return
	}

	// THE ONLY PAGE IN THE APP THAT SENDS A CSP, and it sends one line of it.
	// The entry body is markdown the writer typed, an image in it is an <img>
	// the editor renders, and every legitimate one is served by this origin --
	// so a remote URL in there can only have got in by being typed into the
	// textarea fallback or pasted as HTML the editor did not catch. Refusing to
	// load it makes that a broken image rather than a request telling a third
	// party which of this user's entries was open and when.
	//
	// It is the backstop rather than the rule: foreign URLs are stripped when a
	// read view renders, and the read view will have to send this header too.
	// img-src alone, because nothing else about this page is being constrained
	// here and a default-src would be a policy for the whole app written in the
	// one handler that needed a line of it.
	w.Header().Set("Content-Security-Policy", "img-src 'self'")

	render(w, r, pages.EditCharacterJournalEntry(pages.JournalEntryPageData{
		CharacterID: characterID.String(),
		EntryID:     entry.ID.String(),
		Title:       entry.Title,
		Body:        entry.Body,
	}))
}

// CreateJournalEntry inserts a blank entry and sends the browser into its
// editor. That is the whole of creation: there is nothing to collect, because
// the title is a field on the page this redirects to.
//
// IT IS A PLAIN FORM POST, not htmx, which is why it answers with a 303 rather
// than an HX-Redirect. The character dialog is htmx because it carries a field
// that can be rejected without leaving the page; a button with nothing to
// validate has no such state, and a form the browser submits itself needs no
// JavaScript at all.
func (a *App) CreateJournalEntry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		redirect(w, r, "/characters")
		return
	}

	entryID := ulid.Make()
	// The statement selects from characters, so a character that is not this
	// user's matches nothing and inserts nothing. Zero rows is that, and it is
	// the only thing it can be: the id is freshly minted, so a duplicate key is
	// not on the table.
	result, err := a.Queries.InsertJournalEntry(ctx, queries.InsertJournalEntryParams{
		ID:          entryID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to create journal entry", "error", err)
		redirectToError(w, r)
		return
	}
	if inserted, err := result.RowsAffected(); err == nil && inserted == 0 {
		redirect(w, r, "/characters")
		return
	}

	redirect(w, r, "/characters/"+characterID.String()+"/edit/journal/"+entryID.String())
}

// SaveJournalEntry is the editor's autosave. It writes the two columns the page
// renders and nothing else.
func (a *App) SaveJournalEntry(w http.ResponseWriter, r *http.Request) {
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

	if !parsePanelForm(w, r, pages.JournalEntryPanel) {
		return
	}

	input, problems := buildJournalInput(r)
	if len(problems) > 0 {
		renderPanelBlock(w, r, pages.JournalEntryPanel, problems)
		return
	}

	result, err := a.Queries.UpdateJournalEntry(ctx, queries.UpdateJournalEntryParams{
		Title:       input.Title,
		Body:        input.Body,
		ID:          entryID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	// `announce` arrives only from the Save button in the page header, which
	// carries it in hx-vals; the debounce on the form never sends it. Reading a
	// field the form does not render is the shape the panel handlers avoid, and
	// it is safe here for the one reason that matters: absent means silent,
	// which is the behaviour the autosave wants, and the worst a misread can do
	// is a missing or an extra toast.
	finishJournalEntry(w, r, result, err, r.PostFormValue("announce") != "", func() {
		a.reconcileJournalImages(ctx, characterID, entryID, sess.UserID, input.Body)
	})
}

// DeleteJournalEntry drops one row. The reply carries no body, and it MUST be a
// 200: base.templ's noSwap config lists 204, and a status in that list sets the
// swap to "none", which overrides the hx-swap="delete" on the button and leaves
// the entry sitting on screen after the database has dropped it. Every other
// delete in the app is the same shape for the same reason.
//
// THE ENTRY'S IMAGES ARE DETACHED, NOT DELETED, and that is two statements
// rather than one plus a loop over R2. An entry holding forty pictures deletes
// in the same time as an entry holding none, the sweeper takes the objects a
// day later, and until it does an undo still finds them.
func (a *App) DeleteJournalEntry(w http.ResponseWriter, r *http.Request) {
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

	// FIRST, because it finds the images through journal_id and the row that
	// column points at is about to be gone. The statement is scoped by the
	// owner, so it matches nothing on a stranger's entry and the delete's zero
	// rows is still what answers them. If the delete fails after this, the next
	// save of the entry re-attaches whatever it still references -- the order
	// heals itself in the direction that matters.
	err := a.Queries.DetachJournalImages(ctx, queries.DetachJournalImagesParams{
		JournalID: &entryID,
		OwnerID:   sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to detach journal images", "error", err)
		htmx.ServerError(w)
		return
	}

	result, err := a.Queries.DeleteJournalEntry(ctx, queries.DeleteJournalEntryParams{
		ID:          entryID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to delete journal entry", "error", err)
		htmx.ServerError(w)
		return
	}
	if deleted, err := result.RowsAffected(); err == nil && deleted == 0 {
		htmx.NotFound(w, "journal entry")
		return
	}

	htmx.Toast(w, "Entry deleted.")
}

// finishJournalEntry is finishInventoryRow with the toast made conditional, and
// that condition is the whole reason it is its own function.
//
// An inventory field is a few words and a save there is an event worth
// announcing. A journal save is a pause between two sentences, and toast.js
// stacks its messages for five seconds each -- so a writing session would end
// with a column of "Entry saved." down the side of the page and the writer's own
// prose behind it. Silence is the correct report for a save nobody asked for.
//
// A SAVE SOMEONE ASKED FOR IS DIFFERENT, and it is why the Save button exists at
// all: the entry was always being saved, and a writer with nothing to read that
// from has to take it on faith. So the button posts the same form to the same
// route and asks to be told, and this is where being told happens -- after the
// write, on the response that carries it, rather than from the client guessing
// off a status code.
//
// saved runs once the write is known to have landed and before anything is put
// on the response. It is the entry's images being reconciled against the body
// that was just stored, and it is a parameter rather than a line in the caller
// because the check that guards it is here: an entry deleted in another tab
// matched nothing, has no body to reconcile against, and gets the 404 below.
func finishJournalEntry(w http.ResponseWriter, r *http.Request, result sql.Result, err error, announce bool, saved func()) {
	if err != nil {
		slog.Error("Failed to save journal entry", "error", err)
		htmx.ServerError(w)
		return
	}

	// Zero matched rows is the entry being gone -- deleted in another tab, most
	// likely -- rather than the character not being this user's, which is why
	// this does not say "character" the way finishPanel does.
	if matched, err := result.RowsAffected(); err == nil && matched == 0 {
		htmx.NotFound(w, "journal entry")
		return
	}

	saved()

	if announce {
		htmx.Toast(w, "Entry saved.")
	}

	renderPanelBlock(w, r, pages.JournalEntryPanel, nil)
}

// JournalLinkFragment is the editor's link dialog: a heading, one field and two
// buttons. It is the only fragment in the app with no server resource behind it
// -- nothing is posted, and journal-editor.js takes the submit, fills the field
// from the href under the cursor and applies the result to the document.
//
// It exists because window.prompt is as banned here as window.confirm, and a
// dialog has to come from somewhere. Rendering it server-side keeps it looking
// like every other dialog for free.
//
// IT TAKES NOTHING, so a query string is a 404 with an empty body rather than
// something to validate. http.NotFound is wrong here: it writes a page-shaped
// body into a dialog.
func (a *App) JournalLinkFragment(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.JournalLinkFragment())
}

type journalInput struct {
	Title string
	Body  string
}

// buildJournalInput reads the editor's two fields. Neither is required: an entry
// is created blank and named afterwards, so a required title would mean the
// browser refused to post the entry that most needs posting.
//
// THE BODY IS STORED EXACTLY AS IT ARRIVES. Trimming it the way the title is
// trimmed would eat the leading spaces of an indented code block, and markdown
// is a format where leading whitespace is content rather than noise.
func buildJournalInput(r *http.Request) (journalInput, []string) {
	var problems []string

	title := strings.TrimSpace(r.PostFormValue("title"))
	if len([]rune(title)) > journalTitleLimit {
		problems = append(problems, "Title must be 255 characters or fewer.")
	}

	body := r.PostFormValue("body")
	if len(body) > journalBodyLimit {
		problems = append(problems, "This entry is too long to save. Split it into two.")
	}

	return journalInput{Title: title, Body: body}, problems
}

func journalEntryID(w http.ResponseWriter, r *http.Request) (ulid.ULID, bool) {
	entryID, err := ulid.Parse(r.PathValue("entryId"))
	if err != nil {
		htmx.NotFound(w, "journal entry")
		return ulid.ULID{}, false
	}

	return entryID, true
}

// redirectToJournal is where a page request that names an entry it cannot have
// goes. The character id is reparsed rather than passed through, so a path
// segment that is not a ULID lands on the character list instead of being
// reflected back into a Location header.
func redirectToJournal(w http.ResponseWriter, r *http.Request) {
	characterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		redirect(w, r, "/characters")
		return
	}

	redirect(w, r, "/characters/"+characterID.String()+"/edit/journal")
}

// journalEntries reads one character's list, filtered when there is a term to
// filter by. Both branches select the same four columns in the same order and
// build the same rows, so the caller cannot tell a search from a list and does
// not need to.
//
// AN EMPTY TERM TAKES THE UNFILTERED QUERY rather than searching for "%%",
// which would match every row and be the same answer. The difference is what
// gets read to produce it: the list query never names body, and the search one
// has to scan it. Clearing the box is the common case -- it happens at the end
// of every search -- and it should cost what the page load costs.
func (a *App) journalEntries(ctx context.Context, characterID, ownerID ulid.ULID, term string) ([]pages.JournalEntry, error) {
	if term == "" {
		rows, err := a.Queries.ListCharacterJournals(ctx, queries.ListCharacterJournalsParams{
			CharacterID: characterID,
			OwnerID:     ownerID,
		})
		if err != nil {
			return nil, err
		}

		entries := make([]pages.JournalEntry, 0, len(rows))
		for _, row := range rows {
			entries = append(entries, journalPageEntry(row.ID, row.Title, row.CreatedAt, row.UpdatedAt))
		}

		return entries, nil
	}

	rows, err := a.Queries.SearchCharacterJournals(ctx, queries.SearchCharacterJournalsParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
		Term:        journalSearchPattern(term),
	})
	if err != nil {
		return nil, err
	}

	entries := make([]pages.JournalEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, journalPageEntry(row.ID, row.Title, row.CreatedAt, row.UpdatedAt))
	}

	return entries, nil
}

// journalSearchWildcards escapes what LIKE reads as a pattern. `%` and `_` are
// wildcards to MySQL and ordinary punctuation to a person, and `\` is what
// escapes them, so it has to be doubled first or escaping the other two would
// arm it. Without this, typing a single `%` matches every entry the character
// has and `_` quietly matches any character at all.
//
// MySQL's default LIKE escape is `\` and the values arrive as bound parameters,
// so this is the only place the string is ever read as a pattern.
var journalSearchWildcards = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)

// journalSearchPattern turns a term into a substring match, which is what a box
// above a list is read as doing. Anchoring it instead -- `term%` -- would find
// "marsh" in "marshalling" and miss it in "the marsh", and the second is the one
// somebody searching their own prose is looking for.
func journalSearchPattern(term string) string {
	return "%" + journalSearchWildcards.Replace(term) + "%"
}

func journalPageEntry(id ulid.ULID, title string, created, updated time.Time) pages.JournalEntry {
	return pages.JournalEntry{
		ID:      id.String(),
		Title:   title,
		Created: journalTimestamp(created),
		Updated: journalTimestamp(updated),
	}
}

// journalTimestamp renders one date twice, in UTC both times.
//
// The server cannot do better than UTC and should not try: MySQL runs at +00:00
// and the DSN parses times, so this holds an instant and nothing about where the
// reader is. The browser is the only thing that knows that, and public/js/
// local-time.js is what rewrites the text once it does. What is written here is
// the RFC 3339 value that rewrite reads, and the rendering that stands until it
// runs -- with JavaScript off, and in a test.
func journalTimestamp(at time.Time) pages.Timestamp {
	utc := at.UTC()

	return pages.Timestamp{
		ISO:  utc.Format(time.RFC3339),
		Text: utc.Format("2 Jan 2006, 15:04") + " UTC",
	}
}
