package controllers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

// testRoomID is the room every test in this file and its two neighbours works
// against.
var testRoomID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVT0")

// roomRequest drives a room handler that takes an id in its path, over a stub
// that can begin a transaction. sess is the caller; the zero value is enough
// for the handlers that only read UserID off it.
func roomRequest(t *testing.T, handler http.HandlerFunc, method string, path string, pathValues map[string]string, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(method, path, nil)
	for key, value := range pathValues {
		r.SetPathValue(key, value)
	}
	r = r.WithContext(session.NewContext(r.Context(), sess))

	rec := httptest.NewRecorder()
	handler(rec, r)

	return rec
}

// newRoomApp is newPanelApp for the handlers that need a pool they can begin a
// transaction on, plus a session store over the same stub.
func newRoomApp(db *roomDB) *App {
	pool := db.db()
	q := queries.New(pool)

	return &App{DB: pool, Queries: q, Sessions: session.NewStore(q, false)}
}

// toastFrom reads the queued toast off a response.
func toastFrom(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	header := rec.Header().Get("HX-Trigger")
	if header == "" {
		return ""
	}

	var events map[string]any
	if err := json.Unmarshal([]byte(header), &events); err != nil {
		t.Fatalf("HX-Trigger is not JSON: %v", err)
	}

	message, _ := events["flash:toast"].(string)

	return message
}

// The happy path. The reply is a redirect with no body, because the dialog the
// post came from is about to be navigated away from.
func TestCreateRoomMintsACodeAndRedirects(t *testing.T) {
	app, db := newPanelApp(1)

	rec := panelPost(t, db, app.NewRoomForm, url.Values{"name": {"  Curse of Strahd  "}}, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	call := db.only(t)
	if !strings.Contains(call.query, "INSERT INTO rooms") {
		t.Errorf("statement is not the create: %q", call.query)
	}
	if len(call.args) != 4 {
		t.Fatalf("statement took %d values, want 4 (id, owner, name, code)", len(call.args))
	}

	id, ok := boundID(call.args[0])
	if !ok {
		t.Fatalf("the id is not a ULID: %T", call.args[0])
	}
	if len(id) != 16 {
		t.Errorf("the id is %d bytes, want 16", len(id))
	}

	// The owner comes from the session, never from the form.
	if call.args[1] != testOwnerID {
		t.Errorf("owner = %v, want %v", call.args[1], testOwnerID)
	}

	// The name is trimmed before it is stored and before it is announced.
	if call.args[2] != "Curse of Strahd" {
		t.Errorf("stored name = %v, want %q", call.args[2], "Curse of Strahd")
	}

	code := boundCode(t, call.args[3])
	if !room.ValidCode(code) {
		t.Errorf("stored code = %q, which ValidCode refuses", code)
	}

	if want := "/rooms/" + id.String(); rec.Header().Get("HX-Redirect") != want {
		t.Errorf("HX-Redirect = %q, want %q", rec.Header().Get("HX-Redirect"), want)
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("body = %q, want empty", body)
	}
	if got := toastFrom(t, rec); got != "Curse of Strahd is ready." {
		t.Errorf("toast = %q", got)
	}
}

// A rejection has to be a 422 specifically. It is the only 4xx the dialog's
// form carries an hx-status route for -- every other code in the range is in
// the noSwap list in base.templ, so the reply would land nowhere and the dialog
// would look like it had done nothing.
func TestCreateRoomRejectsBadNamesWithoutWriting(t *testing.T) {
	for name, c := range map[string]struct{ value, want string }{
		"empty":           {"", "Name is required."},
		"whitespace only": {"   \t ", "Name is required."},
		"too long":        {strings.Repeat("a", 129), "Name must be 128 characters or fewer."},
	} {
		t.Run(name, func(t *testing.T) {
			app, db := newPanelApp(1)

			rec := panelPost(t, db, app.NewRoomForm, url.Values{"name": {c.value}}, nil)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}
			if len(db.calls) != 0 {
				t.Errorf("ran %d statements, want 0", len(db.calls))
			}
			if rec.Header().Get("HX-Redirect") != "" {
				t.Error("a rejected create still sent HX-Redirect")
			}
			if body := rec.Body.String(); !strings.Contains(body, c.want) {
				t.Errorf("body missing %q: %s", c.want, body)
			}
			// Into the block the form targets, not the form itself -- so the
			// name the GM typed is still in the field.
			if !strings.Contains(rec.Body.String(), `id="errors-new-room"`) {
				t.Errorf("body is not the error block: %s", rec.Body.String())
			}
		})
	}
}

// The column is varchar(128) and MySQL counts characters there. A byte-length
// check would reject this name at 128 letters the database would have taken.
func TestCreateRoomMeasuresTheNameInCharactersNotBytes(t *testing.T) {
	app, db := newPanelApp(1)

	rec := panelPost(t, db, app.NewRoomForm, url.Values{"name": {strings.Repeat("é", 128)}}, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(db.calls) != 1 {
		t.Fatalf("ran %d statements, want 1", len(db.calls))
	}
}

// The dialog's content is the same for every user, so the handler that serves
// it should not be reaching for a row to render it.
func TestNewRoomFragmentTouchesNoDatabase(t *testing.T) {
	app, db := newPanelApp(1)

	r := httptest.NewRequest(http.MethodGet, "/fragment/room/new", nil)
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
	rec := httptest.NewRecorder()
	app.NewRoomFragment(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(db.calls) != 0 {
		t.Errorf("ran %d statements, want 0", len(db.calls))
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
}

// THE OWNER-SCOPED STATEMENT RUNS FIRST AND THAT IS THE WHOLE OWNERSHIP CHECK
// ON THE OTHER ONE. ClearRoomSessions names a room and no owner -- it has to,
// since it clears every session in the room -- so a delete that emptied the
// table before checking who owned it would let anybody turn a room out.
func TestDeleteRoomRemovesTheRoomBeforeEmptyingIt(t *testing.T) {
	db := &roomDB{rows: 1}
	app := newRoomApp(db)

	rec := roomRequest(t, app.DeleteRoom, http.MethodDelete, "/rooms/"+testRoomID.String(),
		map[string]string{"id": testRoomID.String()}, session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	// 200 and not 204: noSwap lists 204, and a status in that list overrides
	// the hx-swap="delete" on the button, which would leave the card on screen
	// after the room was gone.
	if body := rec.Body.String(); body != "" {
		t.Errorf("body = %q, want empty", body)
	}

	if len(db.calls) != 2 {
		t.Fatalf("ran %d statements, want 2: %v", len(db.calls), db.queries())
	}
	if !strings.Contains(db.calls[0].query, "DELETE FROM rooms") {
		t.Errorf("the first statement is not the delete: %q", db.calls[0].query)
	}
	if !strings.Contains(db.calls[1].query, "UPDATE sessions") {
		t.Errorf("the second statement is not the session sweep: %q", db.calls[1].query)
	}

	assertBoundToRoom(t, db.calls[0], testRoomID)
	assertBoundToRoom(t, db.calls[1], testRoomID)

	// And the delete carries the owner, which is what makes the rollback below
	// reachable at all.
	if owner, ok := boundRoomID(db.calls[0].args[1]); !ok || owner != testOwnerID {
		t.Errorf("the delete is not owner-scoped: %v", db.calls[0].args)
	}
}

// A delete that matched no row is a room that is not there or not yours, and
// both are the same 404. The transaction rolls back, so the session sweep that
// would have run next never lands.
func TestDeletingSomebodyElsesRoomIsA404(t *testing.T) {
	db := &roomDB{rows: 0}
	app := newRoomApp(db)

	rec := roomRequest(t, app.DeleteRoom, http.MethodDelete, "/rooms/"+testRoomID.String(),
		map[string]string{"id": testRoomID.String()}, session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if len(db.calls) != 1 {
		t.Errorf("ran %d statements, want 1 -- the sweep should not have been reached: %v", len(db.calls), db.queries())
	}
}

// assertBoundToRoom checks that a recorded statement names the room under test.
func assertBoundToRoom(t *testing.T, call recordedCall, want ulid.ULID) {
	t.Helper()

	for _, arg := range call.args {
		if id, ok := boundRoomID(arg); ok && id == want {
			return
		}
	}

	t.Errorf("statement is not bound to %v: %q with %v", want, call.query, call.args)
}

// boundCode reads a room code out of a recorded argument, whichever of the two
// stubs recorded it.
//
// THE TWO LAYERS BIND IT DIFFERENTLY AND BOTH ARE CORRECT. recordingDB is a
// queries.DBTX and sees what sqlc passed, which is a sql.NullString -- the
// column is nullable, because a closed room has no code. roomDB is a
// driver.Connector and sees what database/sql handed the driver, by which point
// that has been unwrapped into a plain string. A test that accepted only one
// form would be asserting which stub it was using.
func boundCode(t *testing.T, arg any) string {
	t.Helper()

	switch code := arg.(type) {
	case sql.NullString:
		if !code.Valid {
			t.Fatal("the code bound as NULL, which is a closed room and not an open one")
		}

		return code.String
	case string:
		return code
	default:
		t.Fatalf("the code bound as %T, want a string or a sql.NullString", arg)

		return ""
	}
}
