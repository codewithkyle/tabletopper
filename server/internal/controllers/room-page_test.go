package controllers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

// testMemberID is somebody who is not the owner of testRoomID.
var testMemberID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVT1")

// memberSession is a player whose session is pointed at the room under test.
func memberSession(roomID ulid.ULID) session.UserSession {
	return session.UserSession{UserID: testMemberID, Hash: []byte("session-hash"), RoomID: &roomID}
}

// getRoomPage drives RoomPage against a room the stub answers with.
func getRoomPage(t *testing.T, db *roomDB, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()

	app := newRoomApp(db)

	return roomRequest(t, app.RoomPage, http.MethodGet, "/rooms/"+testRoomID.String(),
		map[string]string{"id": testRoomID.String()}, sess)
}

// The owner reaches the page whatever their session says about rooms: it is
// ownership that admits them, and the GM is never a member.
func TestTheOwnerReachesTheRoomPageAsGM(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false),
		noMembersAnswer(),
	}}

	rec := getRoomPage(t, db, session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	// The three controls only a GM gets. The code is the one that matters
	// most: it is the single thing on this page that admits somebody else.
	for _, want := range []string{"AB2C", "/close", "/lock"} {
		if !strings.Contains(body, want) {
			t.Errorf("the GM's page is missing %q", want)
		}
	}
	if strings.Contains(body, "/leave") {
		t.Error("the GM's page offers Leave, which is a member's control")
	}
}

// A player is admitted by their session's room_id, and gets the same shell
// without the GM's controls.
func TestAMemberReachesTheRoomPageAsAPlayer(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false),
		noMembersAnswer(),
	}}

	rec := getRoomPage(t, db, memberSession(testRoomID))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "/leave") {
		t.Error("the player's page has no way out of the room")
	}
	// THE CODE IS THE GM'S. A player who is already in the room has no use for
	// it, and it is what admits somebody else.
	if strings.Contains(body, "AB2C") {
		t.Error("the player's page prints the room code")
	}
	for _, forbidden := range []string{"/close", "/lock", "/unlock"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the player's page carries the GM control %q", forbidden)
		}
	}
}

// Somebody who is neither the owner nor a member is not told the room exists.
// They get the join page, which is where they would go next anyway.
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
	// The members were never read: the refusal happens before the page has
	// anything to draw.
	if len(db.calls) != 1 {
		t.Errorf("ran %d statements, want 1: %v", len(db.calls), db.queries())
	}
}

// A CLOSED ROOM IS OVER FOR A PLAYER AND NOT FOR THE GM. The player's session
// is cleared rather than left pointing at a room they cannot rejoin -- the
// alternative is a homepage that goes on offering "return to your table" for a
// table that is gone.
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

// The GM keeps the closed room, because they are the one who has to reopen it.
func TestTheOwnerOfAClosedRoomStillGetsThePage(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "", false, true),
		noMembersAnswer(),
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

// A `room` that will not parse came off the page's own markup, so there is
// nobody on the other end to tell -- and the body has to be empty, because
// noSwap leaves the caller's panel untouched for a 4xx and a page-shaped body
// swapped into it would be the wreckage instead.
func TestTheMembersFragmentRefusesAMalformedRoom(t *testing.T) {
	db := &roomDB{rows: 1}
	app := newRoomApp(db)

	rec := roomRequest(t, app.RoomMembersFragment, http.MethodGet, "/fragment/room/members?room=nonsense", nil,
		session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("body = %q, want empty", body)
	}
	if len(db.calls) != 0 {
		t.Errorf("ran %d statements, want 0: %v", len(db.calls), db.queries())
	}
}

// The same refusal for a room that is somebody else's, and it is a 404 rather
// than the page's redirect: a fragment is a piece of a page that is already
// open, and there is nowhere for it to navigate to.
func TestTheMembersFragmentRefusesARoomThatIsNotYours(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false),
	}}
	app := newRoomApp(db)

	rec := roomRequest(t, app.RoomMembersFragment, http.MethodGet,
		"/fragment/room/members?room="+testRoomID.String(), nil, session.UserSession{UserID: testMemberID})

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("body = %q, want empty", body)
	}
}

// CLOSING IS TWO STATEMENTS IN ONE TRANSACTION AND THE ORDER IS THE OWNERSHIP
// CHECK. ClearRoomSessions names a room and no owner, so the owner-scoped close
// runs first and a caller who owns nothing rolls back before it.
func TestClosingARoomClosesItThenEmptiesIt(t *testing.T) {
	db := &roomDB{rows: 1}
	app := newRoomApp(db)

	rec := roomRequest(t, app.CloseRoom, http.MethodPost, "/rooms/"+testRoomID.String()+"/close",
		map[string]string{"id": testRoomID.String()}, session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(db.calls) != 2 {
		t.Fatalf("ran %d statements, want 2: %v", len(db.calls), db.queries())
	}
	if !strings.Contains(db.calls[0].query, "UPDATE rooms") || !strings.Contains(db.calls[0].query, "closed_at = NOW()") {
		t.Errorf("the first statement is not the close: %q", db.calls[0].query)
	}
	if !strings.Contains(db.calls[1].query, "UPDATE sessions") {
		t.Errorf("the second statement is not the session sweep: %q", db.calls[1].query)
	}

	assertBoundToRoom(t, db.calls[0], testRoomID)
	assertBoundToRoom(t, db.calls[1], testRoomID)

	if got := rec.Header().Get("HX-Redirect"); got != "/rooms" {
		t.Errorf("HX-Redirect = %q, want %q", got, "/rooms")
	}
	if got := toastFrom(t, rec); got != "The room is closed." {
		t.Errorf("toast = %q", got)
	}
}

// A close that matched no row is a room that is not there or not yours, and the
// transaction rolls back before the sweep.
func TestClosingSomebodyElsesRoomIsA404(t *testing.T) {
	db := &roomDB{rows: 0}
	app := newRoomApp(db)

	rec := roomRequest(t, app.CloseRoom, http.MethodPost, "/rooms/"+testRoomID.String()+"/close",
		map[string]string{"id": testRoomID.String()}, session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if len(db.calls) != 1 {
		t.Errorf("ran %d statements, want 1 -- the sweep should not have been reached: %v", len(db.calls), db.queries())
	}
}

// The lock pair answer with the control they just changed, which is the
// mutation case the fragment rules name -- and they answer it without a read,
// because the statement carried the new value and matched a row.
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
			// Owner-scoped, which is the whole of the check: there is no read
			// in front of this statement.
			if owner, ok := boundRoomID(call.args[2]); !ok || owner != testOwnerID {
				t.Errorf("the lock write is not owner-scoped: %v", call.args)
			}

			// The reply is the control in its new state, so the button now
			// offers the opposite action.
			if !strings.Contains(rec.Body.String(), c.want) {
				t.Errorf("the reply does not offer %q: %s", c.want, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `id="room-lock"`) {
				t.Errorf("the reply is not the lock control: %s", rec.Body.String())
			}
		})
	}
}

// Leaving touches the session row and nothing else, and only for the room the
// session is actually in -- a POST naming another room is a stale page, and
// clearing whatever room they happened to be in would take somebody out of a
// game because a tab was old.
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

// Reopening mints a new code, and it is a new one rather than the old one --
// that went back into circulation when the room closed.
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
