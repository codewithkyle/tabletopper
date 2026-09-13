package controllers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tabletopper/internal/hub"
	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

var testMemberID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVT1")

func memberSession(roomID ulid.ULID) session.UserSession {
	return session.UserSession{UserID: testMemberID, Hash: []byte("session-hash"), RoomID: &roomID}
}
func getRoomPage(t *testing.T, db *roomDB, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()
	app := newRoomApp(db)
	return roomRequest(t, app.RoomPage, http.MethodGet, "/rooms/"+testRoomID.String(),
		map[string]string{"id": testRoomID.String()}, sess)
}
func TestTheOwnerReachesTheRoomPageAsGM(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false),
	}}
	rec := getRoomPage(t, db, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{"AB2C", "/close", "/lock"} {
		if !strings.Contains(body, want) {
			t.Errorf("the GM's page is missing %q", want)
		}
	}
	if strings.Contains(body, "/leave") {
		t.Error("the GM's page offers Leave, which is a member's control")
	}
}
func TestAMemberReachesTheRoomPageAsAPlayer(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false),
	}}
	rec := getRoomPage(t, db, memberSession(testRoomID))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/leave") {
		t.Error("the player's page has no way out of the room")
	}
	if !strings.Contains(body, "AB2C") {
		t.Error("the player's page does not carry the room code")
	}
	for _, forbidden := range []string{"/close", "/lock", "/unlock"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the player's page carries the GM control %q", forbidden)
		}
	}
}
func TestANonMemberIsSentToTheJoinPage(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false),
	}}
	rec := getRoomPage(t, db, session.UserSession{UserID: testMemberID})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/rooms/join" {
		t.Errorf("Location = %q, want %q", got, "/rooms/join")
	}
	if len(db.calls) != 1 {
		t.Errorf("ran %d statements, want 1: %v", len(db.calls), db.queries())
	}
}
func TestAMemberOfAClosedRoomIsTurnedOut(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "", false, true),
	}}
	rec := getRoomPage(t, db, memberSession(testRoomID))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/rooms/join" {
		t.Errorf("Location = %q, want %q", got, "/rooms/join")
	}
	if len(db.calls) != 2 {
		t.Fatalf("ran %d statements, want the read and the clear: %v", len(db.calls), db.queries())
	}
	if !strings.Contains(db.calls[1].query, "room_id = NULL") {
		t.Errorf("the session was not cleared: %q", db.calls[1].query)
	}
}
func TestTheOwnerOfAClosedRoomStillGetsThePage(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "", false, true),
	}}
	rec := getRoomPage(t, db, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "/open") {
		t.Error("the closed room offers no way to reopen it")
	}
	if strings.Contains(rec.Body.String(), "/close") {
		t.Error("the closed room still offers Close")
	}
}
func TestClosingARoomClosesItThenEmptiesIt(t *testing.T) {
	db := &roomDB{rows: 1}
	app := newRoomApp(db)
	rec := roomRequest(t, app.CloseRoom, http.MethodPost, "/rooms/"+testRoomID.String()+"/close",
		map[string]string{"id": testRoomID.String()}, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(db.calls) != 3 {
		t.Fatalf("ran %d statements, want 3: %v", len(db.calls), db.queries())
	}
	if !strings.Contains(db.calls[0].query, "GetRoom") {
		t.Errorf("the first statement is not the read that finds the open scene: %q", db.calls[0].query)
	}
	if !strings.Contains(db.calls[1].query, "UPDATE rooms") || !strings.Contains(db.calls[1].query, "closed_at = NOW()") {
		t.Errorf("the second statement is not the close: %q", db.calls[1].query)
	}
	if !strings.Contains(db.calls[2].query, "UPDATE sessions") {
		t.Errorf("the third statement is not the session sweep: %q", db.calls[2].query)
	}
	assertBoundToRoom(t, db.calls[1], testRoomID)
	assertBoundToRoom(t, db.calls[1], testRoomID)
	if got := rec.Header().Get("HX-Redirect"); got != "/rooms" {
		t.Errorf("HX-Redirect = %q, want %q", got, "/rooms")
	}
	if got := toastFrom(t, rec); got != "The room is closed." {
		t.Errorf("toast = %q", got)
	}
}
func TestClosingSomebodyElsesRoomIsA404(t *testing.T) {
	db := &roomDB{rows: 0}
	app := newRoomApp(db)
	rec := roomRequest(t, app.CloseRoom, http.MethodPost, "/rooms/"+testRoomID.String()+"/close",
		map[string]string{"id": testRoomID.String()}, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if len(db.calls) != 2 {
		t.Errorf("ran %d statements, want 2 -- the sweep should not have been reached: %v", len(db.calls), db.queries())
	}
}
func TestLockingARoomAnswersWithTheControl(t *testing.T) {
	for name, c := range map[string]struct {
		handler func(*App) http.HandlerFunc
		want    string
		absent  string
	}{
		"lock":   {func(a *App) http.HandlerFunc { return a.LockRoom }, "/unlock", "Unlock"},
		"unlock": {func(a *App) http.HandlerFunc { return a.UnlockRoom }, "/lock", "Lock"},
	} {
		t.Run(name, func(t *testing.T) {
			db := &roomDB{rows: 1}
			app := newRoomApp(db)
			rec := roomRequest(t, c.handler(app), http.MethodPost, "/rooms/"+testRoomID.String()+"/"+name,
				map[string]string{"id": testRoomID.String()}, session.UserSession{UserID: testOwnerID})
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			call := db.only(t)
			if !strings.Contains(call.query, "is_locked = ?") {
				t.Errorf("statement is not the lock write: %q", call.query)
			}
			if owner, ok := boundRoomID(call.args[2]); !ok || owner != testOwnerID {
				t.Errorf("the lock write is not owner-scoped: %v", call.args)
			}
			if !strings.Contains(rec.Body.String(), c.want) {
				t.Errorf("the reply does not offer %q: %s", c.want, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `id="room-lock"`) {
				t.Errorf("the reply is not the lock control: %s", rec.Body.String())
			}
		})
	}
}
func TestLeavingRefusesARoomTheSessionIsNotIn(t *testing.T) {
	other := ulid.Make()
	for name, sess := range map[string]session.UserSession{
		"in no room":         {UserID: testMemberID, Hash: []byte("session-hash")},
		"in a different one": memberSession(other),
	} {
		t.Run(name, func(t *testing.T) {
			db := &roomDB{rows: 1}
			app := newRoomApp(db)
			rec := roomRequest(t, app.LeaveRoom, http.MethodPost, "/rooms/"+testRoomID.String()+"/leave",
				map[string]string{"id": testRoomID.String()}, sess)
			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
			if len(db.calls) != 0 {
				t.Errorf("ran %d statements, want 0: %v", len(db.calls), db.queries())
			}
		})
	}
}
func TestLeavingClearsTheSessionAndGoesHome(t *testing.T) {
	db := &roomDB{rows: 1}
	app := newRoomApp(db)
	rec := roomRequest(t, app.LeaveRoom, http.MethodPost, "/rooms/"+testRoomID.String()+"/leave",
		map[string]string{"id": testRoomID.String()}, memberSession(testRoomID))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	call := db.only(t)
	if !strings.Contains(call.query, "room_id = NULL") {
		t.Errorf("statement is not the clear: %q", call.query)
	}
	if got := rec.Header().Get("HX-Redirect"); got != "/" {
		t.Errorf("HX-Redirect = %q, want %q", got, "/")
	}
}
func TestReopeningMintsANewCode(t *testing.T) {
	db := &roomDB{rows: 1}
	app := newRoomApp(db)
	rec := roomRequest(t, app.OpenRoom, http.MethodPost, "/rooms/"+testRoomID.String()+"/open",
		map[string]string{"id": testRoomID.String()}, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	call := db.only(t)
	if !strings.Contains(call.query, "closed_at = NULL") {
		t.Errorf("statement is not the reopen: %q", call.query)
	}
	if code := boundCode(t, call.args[0]); len(code) != 4 {
		t.Errorf("the reopen wrote %q, which is not a code", code)
	}
	if want := "/rooms/" + testRoomID.String(); rec.Header().Get("HX-Redirect") != want {
		t.Errorf("HX-Redirect = %q, want %q", rec.Header().Get("HX-Redirect"), want)
	}
}
func TestTheRoomPageCarriesWhatTheClientNeedsToConnect(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false),
	}}
	app := newRoomApp(db)
	app.Hub = hub.New(nil, hub.Options{Store: emptyRoomStore{}, Version: "abc123"})
	rec := roomRequest(t, app.RoomPage, http.MethodGet, "/rooms/"+testRoomID.String(),
		map[string]string{"id": testRoomID.String()}, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`data-room="` + testRoomID.String() + `"`,
		`data-role="gm"`,
		`data-version="abc123"`,
		`data-socket="/socket/room/` + testRoomID.String() + `"`,
		`src="/static/room.js?v=abc123"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the page is missing %s", want)
		}
	}
}
func TestAClosedRoomTellsTheClientNotToConnect(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "", false, true),
	}}
	app := newRoomApp(db)
	app.Hub = hub.New(nil, hub.Options{Store: emptyRoomStore{}, Version: "abc123"})
	rec := roomRequest(t, app.RoomPage, http.MethodGet, "/rooms/"+testRoomID.String(),
		map[string]string{"id": testRoomID.String()}, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if strings.Contains(rec.Body.String(), "data-socket") {
		t.Error("a closed room renders a socket path")
	}
}
