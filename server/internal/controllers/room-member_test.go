package controllers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

func memberRequest(method, target, pathID string, sess session.UserSession) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	if pathID != "" {
		r.SetPathValue("id", pathID)
	}
	return r.WithContext(session.NewContext(r.Context(), sess))
}
func TestARoomThatIsNotThereIsAnEmpty404ToAFetchAndAnAlertToAChange(t *testing.T) {
	sess := session.UserSession{UserID: testOwnerID}
	for _, c := range []struct {
		method string
		target string
		pathID string
		alert  bool
	}{
		{http.MethodGet, "/fragment/room/members?room=not-a-ulid", "", false},
		{http.MethodPost, "/rooms/not-a-ulid/dice", "not-a-ulid", true},
	} {
		rec := httptest.NewRecorder()
		app := newRoomApp(&roomDB{})
		if _, _, ok := app.member(rec, memberRequest(c.method, c.target, c.pathID, sess)); ok {
			t.Fatalf("%s %s found a room", c.method, c.target)
		}
		trigger := rec.Header().Get("HX-Trigger")
		if rec.Code != http.StatusNotFound || rec.Body.Len() != 0 {
			t.Errorf("%s answered %d with %q, want an empty 404", c.method, rec.Code, rec.Body.String())
		}
		if got := strings.Contains(trigger, "no longer exists"); got != c.alert {
			t.Errorf("%s: alert sent = %v, want %v (%q)", c.method, got, c.alert, trigger)
		}
	}
}
func TestOnlyTheRoomPageClearsAClosedRoomFromTheSession(t *testing.T) {
	roomID := ulid.Make()
	player := session.UserSession{UserID: ulid.Make(), Hash: []byte("hash"), RoomID: &roomID}
	closed := func() *roomDB {
		return &roomDB{rows: 1, answers: []roomAnswer{getRoomAnswer(roomID, testOwnerID, "Barovia", "", false, true)}}
	}
	cleared := func(db *roomDB) bool {
		for _, call := range db.recorded() {
			if strings.Contains(call.query, "UPDATE sessions") {
				return true
			}
		}
		return false
	}
	db := closed()
	rec := httptest.NewRecorder()
	if _, _, ok := newRoomApp(db).member(rec, memberRequest(http.MethodGet, "/fragment/room/members?room="+roomID.String(), "", player)); ok {
		t.Fatal("a player was let into a closed room")
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("the fragment answered %d, want 404", rec.Code)
	}
	if cleared(db) {
		t.Error("reading membership for a fragment wrote to the session")
	}
	db = closed()
	rec = httptest.NewRecorder()
	newRoomApp(db).RoomPage(rec, memberRequest(http.MethodGet, "/rooms/"+roomID.String(), roomID.String(), player))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/rooms/join" {
		t.Errorf("the page answered %d to %q, want a redirect to /rooms/join", rec.Code, rec.Header().Get("Location"))
	}
	if !cleared(db) {
		t.Error("the room page left the closed room on the session")
	}
}
func TestTheGMHelperTurnsAPlayerAway(t *testing.T) {
	roomID := ulid.Make()
	player := session.UserSession{UserID: ulid.Make(), RoomID: &roomID}
	db := &roomDB{answers: []roomAnswer{getRoomAnswer(roomID, testOwnerID, "Barovia", "", false, false)}}
	rec := httptest.NewRecorder()
	if _, ok := newRoomApp(db).gm(rec, memberRequest(http.MethodGet, "/fragment/room/grid?room="+roomID.String(), "", player)); ok {
		t.Fatal("a player passed the GM check")
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("a player asking for a GM fragment got %d, want 404", rec.Code)
	}
	db = &roomDB{answers: []roomAnswer{getRoomAnswer(roomID, testOwnerID, "Barovia", "", false, false)}}
	rec = httptest.NewRecorder()
	if _, ok := newRoomApp(db).gm(rec, memberRequest(http.MethodGet, "/fragment/room/grid?room="+roomID.String(), "", session.UserSession{UserID: testOwnerID})); !ok {
		t.Errorf("the GM was turned away with %d", rec.Code)
	}
}
