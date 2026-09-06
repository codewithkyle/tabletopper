package controllers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

var testEntryID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS1")

// journalRequest drives one journal handler. It is inventoryRequest with a
// different path value, and it exists for the same reason: three of these
// routes are not POSTs.
func journalRequest(t *testing.T, handler http.HandlerFunc, method string, form url.Values, entryID string) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(method, "/characters/journal", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetPathValue("id", testCharacterID.String())
	if entryID != "" {
		r.SetPathValue("entryId", entryID)
	}
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))

	rec := httptest.NewRecorder()
	handler(rec, r)

	return rec
}

func journalForm() url.Values {
	return url.Values{
		"title": {"Session 12"},
		"body":  {"We went back to the marsh.\n\n## The hag\n\nShe wanted the ring."},
	}
}

// A save writes the entry's own two columns. The character's columns are what it
// must not be able to reach: the sheet's parse helpers return their fallback on
// an empty string, so a handler wide enough to touch them would write 10 over
// every ability score and report success.
func TestSaveJournalEntryWritesOnlyItsOwnColumns(t *testing.T) {
	app, db := newPanelApp(1)

	rec := journalRequest(t, app.SaveJournalEntry, http.MethodPost, journalForm(), testEntryID.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// TWO STATEMENTS NOW, and the write is the first of them: the save
	// reconciles the entry's images afterwards. That one is asserted in
	// journal-images_test.go; here it is only in the way.
	if len(db.calls) != 2 {
		t.Fatalf("statements run = %d, want 2", len(db.calls))
	}
	call := db.calls[0]
	if !strings.Contains(call.query, "UPDATE journals") {
		t.Fatalf("did not update journals:\n%s", call.query)
	}
	if got, want := setColumns(t, call.query), []string{"title", "body"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("wrote %v, want %v", got, want)
	}

	// All three of the scoping values, not just the owner. The entry id and the
	// character id both arrive in the URL, so the statement has to carry both or
	// an entry could be written through a character it does not belong to.
	scope := call.args[len(call.args)-3:]
	for i, want := range []ulid.ULID{testEntryID, testCharacterID, testOwnerID} {
		if got, ok := scope[i].(ulid.ULID); !ok || got != want {
			t.Errorf("scope[%d] = %v, want %v", i, scope[i], want)
		}
	}
}

// A JOURNAL SAVE DOES NOT TOAST, and this is the only place that says so.
// finishInventoryRow announces every save because an inventory field is a few
// words; a journal save is a pause between two sentences, and toast.js stacks
// its messages for five seconds each, so announcing them would bury the page in
// a writing session's worth of "saved."
//
// The body still has to be the cleared error block rather than a 204, for the
// reason every panel answers that way: it is what wipes a message the previous
// save left there.
func TestSaveJournalEntryIsSilent(t *testing.T) {
	app, _ := newPanelApp(1)

	rec := journalRequest(t, app.SaveJournalEntry, http.MethodPost, journalForm(), testEntryID.String())

	if got := rec.Header().Get("HX-Trigger"); got != "" {
		t.Errorf("HX-Trigger = %q, want none", got)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `<div id="errors-journal"></div>` {
		t.Errorf("body = %q, want the cleared error block", body)
	}
}

// The Save button's post is the debounce's post plus one field, and the field is
// the whole difference: the same statement runs and the reply carries a toast.
//
// The pair of these is the contract. A save that announced itself either way
// would bury a writing session in toasts; one that never announced would leave
// the button looking like it did nothing.
func TestAnAnnouncedSaveToasts(t *testing.T) {
	app, db := newPanelApp(1)

	form := journalForm()
	form.Set("announce", "1")
	rec := journalRequest(t, app.SaveJournalEntry, http.MethodPost, form, testEntryID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Header().Get("HX-Trigger"), "Entry saved.") {
		t.Errorf("no toast in HX-Trigger: %q", rec.Header().Get("HX-Trigger"))
	}

	// The same two statements in the same order: the button is not a second
	// save path, it is the debounce's post with one more field on it.
	if len(db.calls) != 2 {
		t.Fatalf("statements run = %d, want 2", len(db.calls))
	}
	if !strings.Contains(db.calls[0].query, "UPDATE journals") {
		t.Errorf("the announced save ran something else:\n%s", db.calls[0].query)
	}
}

// Both limits are refused with a message rather than reaching the driver. MySQL
// runs strict, so an overlong value comes back as an error and would surface as
// a 500 on a field the writer was entitled to overfill.
func TestJournalLimitsAreRejectedBeforeTheWrite(t *testing.T) {
	for _, c := range []struct {
		name  string
		form  url.Values
		wants string
	}{
		{
			name:  "title",
			form:  url.Values{"title": {strings.Repeat("a", journalTitleLimit+1)}, "body": {"fine"}},
			wants: "Title must be 255 characters or fewer.",
		},
		{
			name:  "body",
			form:  url.Values{"title": {"Session 12"}, "body": {strings.Repeat("a", journalBodyLimit+1)}},
			wants: "too long to save",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, db := newPanelApp(1)

			rec := journalRequest(t, app.SaveJournalEntry, http.MethodPost, c.form, testEntryID.String())

			// 422 is the one 4xx the panel has an hx-status route for; every
			// other one is in base.templ's noSwap list and would replace
			// nothing, so the writer would see no difference.
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}
			if !strings.Contains(rec.Body.String(), c.wants) {
				t.Errorf("no %q in the reply:\n%s", c.wants, rec.Body.String())
			}
			if len(db.calls) != 0 {
				t.Errorf("ran %d statements, want 0", len(db.calls))
			}
		})
	}
}

// A title of 255 accented letters fits the column, because VARCHAR(255) counts
// characters and so does the limit. len() on the string would have rejected it.
func TestJournalTitleIsMeasuredInCharacters(t *testing.T) {
	app, db := newPanelApp(1)

	form := url.Values{"title": {strings.Repeat("é", journalTitleLimit)}, "body": {"fine"}}
	rec := journalRequest(t, app.SaveJournalEntry, http.MethodPost, form, testEntryID.String())

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	// The write and the reconciliation that follows it.
	if len(db.calls) != 2 {
		t.Errorf("ran %d statements, want 2", len(db.calls))
	}
}

// The body is stored exactly as it arrives. Trimming it would eat the leading
// spaces of an indented code block, and markdown is a format where leading
// whitespace is content.
func TestJournalBodyIsStoredUntrimmed(t *testing.T) {
	app, db := newPanelApp(1)

	const body = "    fireball\n\nand then we ran.\n"
	journalRequest(t, app.SaveJournalEntry, http.MethodPost, url.Values{"title": {"x"}, "body": {body}}, testEntryID.String())

	call := db.calls[0]
	if got, ok := call.args[1].(string); !ok || got != body {
		t.Errorf("body = %q, want %q", call.args[1], body)
	}
}

// An entry id that is not a ULID never reaches a query on either mutation. Both
// answer htmx.NotFound, which raises the alert dialog rather than swapping
// something into the page.
func TestJournalMutationsRejectAnUnparseableEntryID(t *testing.T) {
	for _, c := range []struct {
		name    string
		handler func(*App) http.HandlerFunc
		method  string
	}{
		{name: "save", handler: func(a *App) http.HandlerFunc { return a.SaveJournalEntry }, method: http.MethodPost},
		{name: "delete", handler: func(a *App) http.HandlerFunc { return a.DeleteJournalEntry }, method: http.MethodDelete},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, db := newPanelApp(1)

			rec := journalRequest(t, c.handler(app), c.method, journalForm(), "not-a-ulid")

			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
			if !strings.Contains(rec.Header().Get("HX-Trigger"), "alert") {
				t.Errorf("no alert in HX-Trigger: %q", rec.Header().Get("HX-Trigger"))
			}
			if len(db.calls) != 0 {
				t.Errorf("ran %d statements, want 0", len(db.calls))
			}
		})
	}
}

// Zero matched rows means the entry is not this user's or is already gone --
// found-rows semantics are on, so it cannot mean "the save changed nothing".
// Reporting success there would be a save that never happened.
func TestJournalMutationsAnswer404WhenNothingMatched(t *testing.T) {
	for _, c := range []struct {
		name    string
		handler func(*App) http.HandlerFunc
		method  string
	}{
		{name: "save", handler: func(a *App) http.HandlerFunc { return a.SaveJournalEntry }, method: http.MethodPost},
		{name: "delete", handler: func(a *App) http.HandlerFunc { return a.DeleteJournalEntry }, method: http.MethodDelete},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, _ := newPanelApp(0)

			rec := journalRequest(t, c.handler(app), c.method, journalForm(), testEntryID.String())

			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
		})
	}
}

// The delete answers 200 and not 204. base.templ's noSwap config lists 204, and
// a status in that list overrides the hx-swap="delete" on the button -- so a 204
// would leave the entry on screen after the database had dropped it.
func TestDeleteJournalEntryAnswers200(t *testing.T) {
	app, db := newPanelApp(1)

	rec := journalRequest(t, app.DeleteJournalEntry, http.MethodDelete, nil, testEntryID.String())

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Header().Get("HX-Trigger"), "Entry deleted.") {
		t.Errorf("no toast in HX-Trigger: %q", rec.Header().Get("HX-Trigger"))
	}
	// Three statements: the share is revoked, the images are detached, then
	// the entry goes. The order is asserted in journal-images_test.go; what
	// matters here is that the delete is still one of them and still last.
	if len(db.calls) != 3 {
		t.Fatalf("statements run = %d, want 3", len(db.calls))
	}
	if !strings.Contains(db.calls[2].query, "DELETE FROM journals") {
		t.Errorf("did not delete from journals:\n%s", db.calls[2].query)
	}
}

// Creation carries no fields, and the statement it runs cannot carry any: it
// names three columns and takes both the owner and the character from the
// characters row, so a character that is not this user's inserts nothing.
func TestCreateJournalEntryCannotCarryEntryData(t *testing.T) {
	app, db := newPanelApp(1)

	rec := journalRequest(t, app.CreateJournalEntry, http.MethodPost, journalForm(), "")

	call := db.only(t)
	if !strings.Contains(call.query, "INSERT INTO journals (id, owner_id, character_id)") {
		t.Errorf("the insert is not three columns:\n%s", call.query)
	}
	if !strings.Contains(call.query, "FROM characters") {
		t.Errorf("the insert does not select from characters:\n%s", call.query)
	}
	if len(call.args) != 3 {
		t.Errorf("insert takes %d values, want 3", len(call.args))
	}

	// A browser posted this, so the reply is a 303 it follows into the editor
	// rather than an HX-Redirect header.
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	prefix := "/characters/" + testCharacterID.String() + "/edit/journal/"
	location := rec.Header().Get("Location")
	if !strings.HasPrefix(location, prefix) {
		t.Fatalf("Location = %q, want a new entry under %q", location, prefix)
	}
	if _, err := ulid.Parse(strings.TrimPrefix(location, prefix)); err != nil {
		t.Errorf("Location does not end in a ULID: %q", location)
	}
}

// The insert matched nothing, so the character is not this user's. That is a
// page request, so it is a redirect and not an alert -- nothing is open to show
// an alert in.
func TestCreateJournalEntryForAStrangersCharacterRedirects(t *testing.T) {
	app, _ := newPanelApp(0)

	rec := journalRequest(t, app.CreateJournalEntry, http.MethodPost, nil, "")

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/characters" {
		t.Errorf("Location = %q, want /characters", got)
	}
}

// THE PAGE REDIRECTS, IT DOES NOT ALERT. htmx.NotFound writes an empty 404 body
// and a header, which is a blank screen for a navigation.
//
// It also runs no statement, which is the ordering this pins: the entry id is
// parsed before the character is loaded, so a last segment that is not a ULID
// costs no query at all. recordingDB panics on a read, so a page that queried
// first would fail here rather than pass quietly.
func TestJournalEntryPageRedirectsOnAnUnparseableEntryID(t *testing.T) {
	app, db := newPanelApp(1)

	rec := journalRequest(t, app.CharacterJournalEntryPage, http.MethodGet, nil, "not-a-ulid")

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got, want := rec.Header().Get("Location"), "/characters/"+testCharacterID.String()+"/edit/journal"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if len(db.calls) != 0 {
		t.Errorf("ran %d statements, want 0", len(db.calls))
	}
}

// A character id that is not a ULID cannot be reflected back into a Location
// header, so the fallback is the character list.
func TestJournalEntryPageRedirectsToTheListWhenTheCharacterIDIsJunk(t *testing.T) {
	app, _ := newPanelApp(1)

	r := httptest.NewRequest(http.MethodGet, "/characters/x/edit/journal/y", nil)
	r.SetPathValue("id", "not-a-ulid")
	r.SetPathValue("entryId", "not-a-ulid")
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
	rec := httptest.NewRecorder()
	app.CharacterJournalEntryPage(rec, r)

	if got := rec.Header().Get("Location"); got != "/characters" {
		t.Errorf("Location = %q, want /characters", got)
	}
}

// The link dialog takes nothing off the request. A query string is a 404 with an
// empty body rather than something to validate -- and not http.NotFound, which
// writes a page-shaped body into a dialog.
func TestJournalLinkFragmentRefusesAQueryString(t *testing.T) {
	app, db := newPanelApp(1)

	r := httptest.NewRequest(http.MethodGet, "/fragment/character/journal-link?href=javascript:alert(1)", nil)
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
	rec := httptest.NewRecorder()
	app.JournalLinkFragment(rec, r)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("body = %q, want empty", body)
	}

	// And with no query it is the dialog, built from no database at all.
	clean := httptest.NewRequest(http.MethodGet, "/fragment/character/journal-link", nil)
	clean = clean.WithContext(session.NewContext(clean.Context(), session.UserSession{UserID: testOwnerID}))
	rec = httptest.NewRecorder()
	app.JournalLinkFragment(rec, clean)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `name="href"`) {
		t.Errorf("no href field in the dialog:\n%s", rec.Body.String())
	}
	if len(db.calls) != 0 {
		t.Errorf("ran %d statements, want 0", len(db.calls))
	}
}

// journalSearch drives the search fragment the way the box does: a GET with the
// character in the path it was rendered with and the term appended beside it.
func journalSearch(t *testing.T, app *App, character, term string) *httptest.ResponseRecorder {
	t.Helper()

	target := "/fragment/character/journal-entries?character=" + url.QueryEscape(character) + "&q=" + url.QueryEscape(term)
	r := httptest.NewRequest(http.MethodGet, target, nil)
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))

	rec := httptest.NewRecorder()
	app.JournalEntriesFragment(rec, r)

	return rec
}

// THE SEARCH IS SCOPED IN THE STATEMENT, and that is the whole of its access
// control -- this route is the one journal handler that never loads the
// character first. owner_id sits beside character_id in the WHERE, so a
// character belonging to somebody else matches nothing and comes back as an
// empty list, which is the same reply an empty journal gives. Drop the owner
// from the statement and the route hands one user another user's journal.
func TestJournalSearchIsScopedToTheCharacterAndTheOwner(t *testing.T) {
	app, db := newPanelApp(0)

	journalSearch(t, app, testCharacterID.String(), "hag")

	call := db.only(t)
	if !strings.Contains(call.query, "FROM journals") || !strings.Contains(call.query, "LIKE") {
		t.Fatalf("did not run the search:\n%s", call.query)
	}

	want := []any{testCharacterID, testOwnerID, "%hag%", "%hag%"}
	if len(call.args) != len(want) {
		t.Fatalf("args = %v, want %v", call.args, want)
	}
	for i, want := range want {
		if call.args[i] != want {
			t.Errorf("args[%d] = %v, want %v", i, call.args[i], want)
		}
	}
}

// An empty box is the whole list, and it has to read the LIST query to get it.
// Searching for "%%" would return the same rows, and would scan every body to
// do it -- on the one path that runs most often, because clearing the box is how
// every search ends.
func TestAnEmptySearchReadsTheUnfilteredList(t *testing.T) {
	for _, term := range []string{"", "   "} {
		app, db := newPanelApp(0)

		journalSearch(t, app, testCharacterID.String(), term)

		call := db.only(t)
		if strings.Contains(call.query, "LIKE") {
			t.Errorf("a blank term %q ran a search:\n%s", term, call.query)
		}
		if strings.Contains(call.query, "body") {
			t.Errorf("a blank term %q read bodies:\n%s", term, call.query)
		}
	}
}

// `%` and `_` are wildcards to LIKE and ordinary characters to somebody typing.
// Unescaped, a single `%` matches every entry the character has, and `_` matches
// any character at all -- so a search for "d_ce" would find "dice" and "dance".
// The backslash goes first, because escaping it after the other two would arm
// the escapes just written.
func TestJournalSearchEscapesLikeWildcards(t *testing.T) {
	for _, tc := range []struct{ term, want string }{
		{"hag", "%hag%"},
		{"100%", `%100\%%`},
		{"d_ce", `%d\_ce%`},
		{`C:\`, `%C:\\%`},
		{`50%_off`, `%50\%\_off%`},
	} {
		if got := journalSearchPattern(tc.term); got != tc.want {
			t.Errorf("journalSearchPattern(%q) = %q, want %q", tc.term, got, tc.want)
		}
	}
}

// Both parameters are checked before anything is queried. The character id comes
// off the page's own markup and the term is capped at the box's maxlength, so
// neither failure is reachable from the control that sends them -- which is why
// the reply is a bare 404 rather than the alert dialog. There is no reader here
// to tell anything to.
func TestJournalSearchRefusesWhatTheBoxCannotSend(t *testing.T) {
	for _, tc := range []struct {
		name      string
		character string
		term      string
	}{
		{"no character", "", "hag"},
		{"character is not a ULID", "the-marsh", "hag"},
		{"term is longer than the box", testCharacterID.String(), strings.Repeat("a", journalSearchLimit+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, db := newPanelApp(0)

			rec := journalSearch(t, app, tc.character, tc.term)

			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("body = %q, want empty", rec.Body.String())
			}
			if len(db.calls) != 0 {
				t.Errorf("reached the database: %v", db.calls)
			}
		})
	}
}
