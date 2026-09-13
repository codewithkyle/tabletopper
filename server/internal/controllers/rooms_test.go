package controllers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"tabletopper/internal/hub"
	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

var testRoomID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVT0")

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
func newRoomApp(db *roomDB) *App {
	pool := db.db()
	q := queries.New(pool)
	return &App{DB: pool, Queries: q, Sessions: session.NewStore(q, false)}
}
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
	if call.args[1] != testOwnerID {
		t.Errorf("owner = %v, want %v", call.args[1], testOwnerID)
	}
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
			if !strings.Contains(rec.Body.String(), `id="errors-new-room"`) {
				t.Errorf("body is not the error block: %s", rec.Body.String())
			}
		})
	}
}
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
func TestDeleteRoomRemovesTheRoomBeforeEmptyingIt(t *testing.T) {
	db := &roomDB{rows: 1}
	app := newRoomApp(db)
	rec := roomRequest(t, app.DeleteRoom, http.MethodDelete, "/rooms/"+testRoomID.String(),
		map[string]string{"id": testRoomID.String()}, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("body = %q, want empty", body)
	}
	if len(db.calls) != 3 {
		t.Fatalf("ran %d statements, want 3: %v", len(db.calls), db.queries())
	}
	if !strings.Contains(db.calls[0].query, "GetRoom") {
		t.Errorf("the first statement is not the read that finds the open scene: %q", db.calls[0].query)
	}
	if !strings.Contains(db.calls[1].query, "DELETE FROM rooms") {
		t.Errorf("the second statement is not the delete: %q", db.calls[1].query)
	}
	if !strings.Contains(db.calls[2].query, "UPDATE sessions") {
		t.Errorf("the third statement is not the session sweep: %q", db.calls[2].query)
	}
	assertBoundToRoom(t, db.calls[1], testRoomID)
	assertBoundToRoom(t, db.calls[2], testRoomID)
	if owner, ok := boundRoomID(db.calls[1].args[1]); !ok || owner != testOwnerID {
		t.Errorf("the delete is not owner-scoped: %v", db.calls[1].args)
	}
}
func TestDeleteRoomClosesTheLiveRoom(t *testing.T) {
	db := &roomDB{rows: 1}
	app := newRoomApp(db)
	app.Hub = hub.New(nil, hub.Options{Store: emptyRoomStore{}})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	gm := room.Actor{ID: testOwnerID, Role: room.RoleGM}
	if err := app.Hub.Dispatch(ctx, testRoomID, gm, &room.PlayerJoin{Player: room.Player{ID: testOwnerID, Name: "kyle", Role: room.RoleGM}}); err != nil {
		t.Fatalf("loading the room: %v", err)
	}
	if _, live := app.Hub.Players(ctx, testRoomID); !live {
		t.Fatal("the room is not live before the delete")
	}
	rec := roomRequest(t, app.DeleteRoom, http.MethodDelete, "/rooms/"+testRoomID.String(),
		map[string]string{"id": testRoomID.String()}, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if _, live := app.Hub.Players(ctx, testRoomID); live {
		t.Error("the room is still live after the delete")
	}
}
func TestDeletingSomebodyElsesRoomIsA404(t *testing.T) {
	db := &roomDB{rows: 0}
	app := newRoomApp(db)
	rec := roomRequest(t, app.DeleteRoom, http.MethodDelete, "/rooms/"+testRoomID.String(),
		map[string]string{"id": testRoomID.String()}, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if len(db.calls) != 2 {
		t.Errorf("ran %d statements, want 2 -- the sweep should not have been reached: %v", len(db.calls), db.queries())
	}
}
func assertBoundToRoom(t *testing.T, call recordedCall, want ulid.ULID) {
	t.Helper()
	for _, arg := range call.args {
		if id, ok := boundRoomID(arg); ok && id == want {
			return
		}
	}
	t.Errorf("statement is not bound to %v: %q with %v", want, call.query, call.args)
}
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
