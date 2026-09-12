package controllers

import (
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

var testEntryID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS1")

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
func TestSaveJournalEntryWritesOnlyItsOwnColumns(t *testing.T) {
	app, db := newPanelApp(1)
	rec := journalRequest(t, app.SaveJournalEntry, http.MethodPost, journalForm(), testEntryID.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
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
	scope := call.args[len(call.args)-3:]
	for i, want := range []ulid.ULID{testEntryID, testCharacterID, testOwnerID} {
		if got, ok := scope[i].(ulid.ULID); !ok || got != want {
			t.Errorf("scope[%d] = %v, want %v", i, scope[i], want)
		}
	}
}
func TestSaveJournalEntryIsSilent(t *testing.T) {
	app, _ := newPanelApp(1)
	rec := journalRequest(t, app.SaveJournalEntry, http.MethodPost, journalForm(), testEntryID.String())
	if got := rec.Header().Get("HX-Trigger"); got != "" {
		t.Errorf("HX-Trigger = %q, want none", got)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `<div id="errors-journal" hidden></div>` {
		t.Errorf("body = %q, want the cleared error block", body)
	}
}
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
	if len(db.calls) != 2 {
		t.Fatalf("statements run = %d, want 2", len(db.calls))
	}
	if !strings.Contains(db.calls[0].query, "UPDATE journals") {
		t.Errorf("the announced save ran something else:\n%s", db.calls[0].query)
	}
}
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
func TestJournalTitleIsMeasuredInCharacters(t *testing.T) {
	app, db := newPanelApp(1)
	form := url.Values{"title": {strings.Repeat("é", journalTitleLimit)}, "body": {"fine"}}
	rec := journalRequest(t, app.SaveJournalEntry, http.MethodPost, form, testEntryID.String())
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(db.calls) != 2 {
		t.Errorf("ran %d statements, want 2", len(db.calls))
	}
}
func TestJournalBodyIsStoredUntrimmed(t *testing.T) {
	app, db := newPanelApp(1)
	const body = "    fireball\n\nand then we ran.\n"
	journalRequest(t, app.SaveJournalEntry, http.MethodPost, url.Values{"title": {"x"}, "body": {body}}, testEntryID.String())
	call := db.calls[0]
	if got, ok := call.args[1].(string); !ok || got != body {
		t.Errorf("body = %q, want %q", call.args[1], body)
	}
}
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
func TestDeleteJournalEntryAnswers200(t *testing.T) {
	app, db := newPanelApp(1)
	rec := journalRequest(t, app.DeleteJournalEntry, http.MethodDelete, nil, testEntryID.String())
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Header().Get("HX-Trigger"), "Entry deleted.") {
		t.Errorf("no toast in HX-Trigger: %q", rec.Header().Get("HX-Trigger"))
	}
	if len(db.calls) != 3 {
		t.Fatalf("statements run = %d, want 3", len(db.calls))
	}
	if !strings.Contains(db.calls[2].query, "DELETE FROM journals") {
		t.Errorf("did not delete from journals:\n%s", db.calls[2].query)
	}
}
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
func journalSearch(t *testing.T, app *App, character, term string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/fragment/character/journal-entries?character=" + url.QueryEscape(character) + "&q=" + url.QueryEscape(term)
	r := httptest.NewRequest(http.MethodGet, target, nil)
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
	rec := httptest.NewRecorder()
	app.JournalEntriesFragment(rec, r)
	return rec
}
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

func journalWindowApp(answers ...roomAnswer) *App {
	return newRoomApp(&roomDB{rows: 1, answers: append([]roomAnswer{tableRoomAnswer()}, answers...)})
}
func journalFragment(t *testing.T, app *App, query string, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/fragment/character/journal?"+query, nil)
	r = r.WithContext(session.NewContext(r.Context(), sess))
	rec := httptest.NewRecorder()
	app.CharacterJournalFragment(rec, r)
	return rec
}
func journalCreate(t *testing.T, app *App, form url.Values, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/characters/"+testCharacterID.String()+"/journal", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetPathValue("id", testCharacterID.String())
	r = r.WithContext(session.NewContext(r.Context(), sess))
	rec := httptest.NewRecorder()
	app.CreateJournalEntry(rec, r)
	return rec
}
func journalRowAnswer(title string) roomAnswer {
	return roomAnswer{
		columns: []string{"id", "title", "created_at", "updated_at"},
		values:  []driver.Value{testEntryID.Bytes(), title, time.Now(), time.Now()},
	}
}
func journalEntryAnswer(title string, body string) roomAnswer {
	return roomAnswer{
		columns: []string{"id", "owner_id", "character_id", "title", "body", "created_at", "updated_at"},
		values:  []driver.Value{testEntryID.Bytes(), testOwnerID.Bytes(), testCharacterID.Bytes(), title, body, time.Now(), time.Now()},
	}
}
func strangerSession() session.UserSession {
	sess := sheetSession(true)
	sess.UserID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS8")
	elsewhere := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS9")
	sess.RoomID = &elsewhere
	return sess
}
func TestTheJournalWindowIsRefusedWithAnEmptyBody(t *testing.T) {
	inRoom := sheetSession(true)
	noCharacter := sheetSession(true)
	noCharacter.CharacterID = nil
	for name, c := range map[string]struct {
		query string
		sess  session.UserSession
	}{
		"no character on the session":  {"room=" + testRoomID.String(), noCharacter},
		"a room the session is not in": {"room=" + testRoomID.String(), strangerSession()},
		"a room that is not a ulid":    {"room=nonsense", inRoom},
		"no room at all":               {"", inRoom},
		"an entry that is not a ulid":  {"room=" + testRoomID.String() + "&entry=nonsense", inRoom},
		"a term longer than the box":   {"room=" + testRoomID.String() + "&q=" + strings.Repeat("a", 256), inRoom},
	} {
		rec := journalFragment(t, journalWindowApp(), c.query, c.sess)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", name, rec.Code)
		}
		if body := strings.TrimSpace(rec.Body.String()); body != "" {
			t.Errorf("%s: the refusal carries a body: %q", name, body)
		}
	}
}
func TestTheJournalWindowOpensOnEverythingWritten(t *testing.T) {
	app := journalWindowApp(journalRowAnswer("Session 12"))
	rec := journalFragment(t, app, "room="+testRoomID.String(), sheetSession(true))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`id="` + pages.JournalBodyID + `"`, "Session 12", "New Entry", `type="search"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the window is missing %q\n%s", want, body)
		}
	}
}
func TestSearchingTheJournalWindowRedrawsOnlyTheList(t *testing.T) {
	app := journalWindowApp(journalRowAnswer("Session 12"))
	rec := journalFragment(t, app, "room="+testRoomID.String()+"&q=", sheetSession(true))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Session 12") {
		t.Errorf("the results are missing\n%s", body)
	}
	if strings.Contains(body, `id="`+pages.JournalBodyID+`"`) {
		t.Errorf("a search redrew the whole window, so the box it was typed in was replaced\n%s", body)
	}
	if strings.Contains(body, `type="search"`) {
		t.Errorf("a search redrew the search box\n%s", body)
	}
}
func TestAnEntryOpenedAtTheTableComesBackAsItsEditor(t *testing.T) {
	app := journalWindowApp(journalEntryAnswer("Session 12", "We went back to the marsh."))
	rec := journalFragment(t, app, "room="+testRoomID.String()+"&entry="+testEntryID.String(), sheetSession(true))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "data-journal-editor-src") {
		t.Errorf("the entry cannot load the editor\n%s", body)
	}
	if !strings.Contains(body, "We went back to the marsh.") {
		t.Errorf("the entry came back empty\n%s", body)
	}
	if strings.Contains(body, `href="/characters/`) {
		t.Errorf("the entry offers a link that leaves the room\n%s", body)
	}
}
func TestANewEntryAtTheTableComesBackAsItsEditor(t *testing.T) {
	app := journalWindowApp(journalEntryAnswer("", ""))
	rec := journalCreate(t, app, url.Values{"room": {testRoomID.String()}}, sheetSession(true))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Errorf("the window was sent to %q instead of being handed the entry", got)
	}
	if !strings.Contains(rec.Body.String(), `id="`+pages.JournalBodyID+`"`) {
		t.Errorf("what came back is not the window's body\n%s", rec.Body.String())
	}
}
func TestANewEntryOutsideARoomStillOpensItsPage(t *testing.T) {
	rec := journalCreate(t, journalWindowApp(), url.Values{}, sheetSession(false))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body: %s", rec.Code, rec.Body.String())
	}
	want := "/characters/" + testCharacterID.String() + "/edit/journal/"
	if got := rec.Header().Get("Location"); !strings.HasPrefix(got, want) {
		t.Errorf("Location = %q, want a new entry under %q", got, want)
	}
}
func TestANewEntryRefusesARoomYouAreNotIn(t *testing.T) {
	rec := journalCreate(t, journalWindowApp(), url.Values{"room": {testRoomID.String()}}, strangerSession())
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "" {
		t.Errorf("the refusal carries a body: %q", body)
	}
}
