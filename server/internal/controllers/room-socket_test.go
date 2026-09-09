package controllers

import (
	"context"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tabletopper/internal/hub"
	"tabletopper/internal/room"
	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

// getSocket drives RoomSocket against a room the stub answers with.
func getSocket(t *testing.T, app *App, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()

	return roomRequest(t, app.RoomSocket, http.MethodGet, "/socket/room/"+testRoomID.String(),
		map[string]string{"id": testRoomID.String()}, sess)
}

// EVERY REFUSAL ON THIS ROUTE IS A 404 AND NONE OF THEM SAYS WHY. A stranger who
// could tell "that room exists but is not yours" from "there is no such room"
// could enumerate rooms, and a WebSocket client cannot render an explanation
// anyway -- all it sees is a handshake that failed.
func TestTheSocketRefusesEverybodyItShould(t *testing.T) {
	for _, c := range []struct {
		name string
		db   *roomDB
		sess session.UserSession
	}{
		{
			name: "somebody whose session points at no room",
			db: &roomDB{rows: 1, answers: []roomAnswer{
				getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false),
			}},
			sess: session.UserSession{UserID: testMemberID, Hash: []byte("h")},
		},
		{
			name: "a player whose session points at another room",
			db: &roomDB{rows: 1, answers: []roomAnswer{
				getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false),
			}},
			sess: memberSession(ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVT9")),
		},
		{
			// The GM's page still renders for a closed room so they can reopen
			// it, but there is no room to run and so nothing to connect to.
			name: "the GM of a closed room",
			db: &roomDB{rows: 1, answers: []roomAnswer{
				getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "", false, true),
			}},
			sess: session.UserSession{UserID: testOwnerID},
		},
		{
			name: "an id that is not a ULID at all",
			db:   &roomDB{rows: 1},
			sess: session.UserSession{UserID: testOwnerID},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			app := newRoomApp(c.db)
			app.Hub = hub.New(nil, hub.Options{Store: emptyRoomStore{}})

			rec := getSocket(t, app, c.sess)

			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
		})
	}
}

// The routes test proves the socket sits behind the 404 wrapper; this proves
// the handler does not panic when the hub is absent, which is what every test
// in this package that builds a bare App relies on.
func TestTheSocketAnswers404WithoutAHub(t *testing.T) {
	app := newRoomApp(&roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false),
	}})

	if rec := getSocket(t, app, session.UserSession{UserID: testOwnerID}); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// The character a person is holding belongs to the room they joined, not to
// their account. Carrying the column across would seat a GM at this table with
// a character from somebody else's party.
func TestOnlyTheCharacterFromThisRoomComesAlong(t *testing.T) {
	character := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVT5")
	elsewhere := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVT9")

	here := session.UserSession{UserID: testMemberID, RoomID: &testRoomID, CharacterID: &character}
	if got := roomCharacter(here, testRoomID); got == nil || *got != character {
		t.Errorf("character = %v, want the one they joined with", got)
	}

	stale := session.UserSession{UserID: testMemberID, RoomID: &elsewhere, CharacterID: &character}
	if got := roomCharacter(stale, testRoomID); got != nil {
		t.Errorf("character = %v, want nothing: that session is at another table", got)
	}
}

// The window is behind the same membership rule as the page, and its refusal is
// an empty 404 rather than a page-shaped one -- htmx leaves the target untouched
// for every 4xx, which is what keeps a stale panel on screen instead of
// replacing it with a message about a room.
func TestTheMembersWindowIsGatedOnMembership(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false),
	}}
	app := newRoomApp(db)

	rec := membersRequest(t, app, session.UserSession{UserID: testMemberID, Hash: []byte("h")})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("body = %q, want it empty", body)
	}
}

// With no live room the window falls back to the session rows, and says so.
// That window is the one between the page rendering and its first frame, and
// the honest thing to show in it is the membership rather than a connected
// state nobody could have.
func TestTheMembersWindowFallsBackToTheSessionRows(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false),
		{
			columns: []string{"user_id", "username", "profile_image_url", "avatar_asset_id", "character_name"},
			values:  []driver.Value{testMemberID.Bytes(), "ari", "/images/default-avatar.webp", nil, "Ilyana"},
		},
	}}
	app := newRoomApp(db)

	rec := membersRequest(t, app, session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	// THE CHARACTER LEADS AND THE ACCOUNT FOLLOWS IN BRACKETS, in the fallback
	// exactly as in the live list -- a window that changed what it was naming
	// depending on where the answer came from would be a window nobody could
	// read across a reconnect.
	if !strings.Contains(body, "Ilyana") || !strings.Contains(body, "(ari)") {
		t.Errorf("the fallback list does not name the character and the account:\n%s", body)
	}
	if !strings.Contains(body, "Waiting for the connection.") {
		t.Error("the fallback list does not say that it is not the live one")
	}
	if !strings.Contains(body, `hx-trigger="room:players from:window"`) {
		t.Error("the swapped-in list does not listen for the next player event, so it would refetch once and stop")
	}
	// The strategy matters and is easy to lose. Without it a burst of player
	// events is a burst of GETs whose answers can land in either order; with
	// "replace" instead, htmx cancels the request in flight and reports the
	// cancellation as an error on every page load.
	if !strings.Contains(body, `hx-sync="this:queue last"`) {
		t.Error("the swapped-in list does not serialise its refetches")
	}
}

// And with a live room it is the room's own answer, connected states and all.
func TestTheMembersWindowPrefersTheLiveRoom(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false),
	}}
	app := newRoomApp(db)
	app.Hub = hub.New(nil, hub.Options{Store: emptyRoomStore{}})

	// Seating somebody is what makes the room live. This is the command the
	// socket sends on connect, sent here without one.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := app.Hub.Dispatch(ctx, testRoomID, room.Actor{ID: testOwnerID, Role: room.RoleGM}, &room.PlayerJoin{
		Player: room.Player{ID: testOwnerID, Name: "kyle", Avatar: "/images/default-avatar.webp", Role: room.RoleGM},
	})
	if err != nil {
		t.Fatalf("seating the GM: %v", err)
	}

	rec := membersRequest(t, app, session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	// THE GM BRINGS NO CHARACTER, so their line is the title and their account,
	// not a blank followed by brackets.
	if !strings.Contains(body, "Game Master") || !strings.Contains(body, "(kyle)") {
		t.Errorf("the live list does not name the GM and their account:\n%s", body)
	}
	if !strings.Contains(body, ">GM<") {
		t.Error("the live list does not say who is running the room")
	}
	if strings.Contains(body, "Waiting for the connection.") {
		t.Error("the live list claims it is the fallback")
	}
}

func membersRequest(t *testing.T, app *App, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()

	return roomRequest(t, app.RoomMembersFragment, http.MethodGet,
		"/fragment/room/members?room="+testRoomID.String(), nil, sess)
}

// emptyRoomStore is a rooms row with no snapshot, which is what a room nobody
// has opened looks like. Nothing in these tests reads it back.
type emptyRoomStore struct{}

func (emptyRoomStore) Load(context.Context, ulid.ULID) (hub.Loaded, error) {
	return hub.Loaded{Name: "Curse of Strahd"}, nil
}
func (emptyRoomStore) Save(context.Context, ulid.ULID, []byte, uint64) error { return nil }
func (emptyRoomStore) ClearMembership(context.Context, ulid.ULID, ulid.ULID) error {
	return nil
}

// THE KICK IS THE GM'S ONE MODERATION TOOL, and every refusal it can produce
// belongs to internal/room rather than to the handler -- so what these check is
// that the handler establishes the right actor and hands the rest over, and
// that the refusals arrive as the alert modal rather than as a swap.

// kickRequest posts the kick for one player, as sess.
func kickRequest(t *testing.T, app *App, target ulid.ULID, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()

	return roomRequest(t, app.KickPlayer, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/players/"+target.String()+"/kick",
		map[string]string{"id": testRoomID.String(), "player": target.String()}, sess)
}

// liveRoomApp is an App over a room the hub has loaded, with the GM and one
// player seated -- which is the state every kick starts from.
func liveRoomApp(t *testing.T, db *roomDB) *App {
	t.Helper()

	app := newRoomApp(db)
	app.Hub = hub.New(nil, hub.Options{Store: emptyRoomStore{}})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	gm := room.Actor{ID: testOwnerID, Role: room.RoleGM}
	seat := func(p room.Player) {
		if err := app.Hub.Dispatch(ctx, testRoomID, gm, &room.PlayerJoin{Player: p}); err != nil {
			t.Fatalf("seating %s: %v", p.Name, err)
		}
	}

	seat(room.Player{ID: testOwnerID, Name: "kyle", Role: room.RoleGM})
	seat(room.Player{ID: testMemberID, Name: "ari", CharacterName: "Ilyana", Role: room.RolePlayer})

	return app
}

func TestTheGMCanRemoveAPlayerAndGetsTheListBack(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false),
	}}
	app := liveRoomApp(t, db)

	rec := kickRequest(t, app, testMemberID, session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	body := rec.Body.String()
	// The reply is the member list it just changed, which is what keeps a GM
	// whose own socket has dropped from watching a row that is no longer there.
	if !strings.Contains(body, `id="room-members"`) {
		t.Errorf("the reply is not the member list:\n%s", body)
	}
	if strings.Contains(body, "Ilyana") {
		t.Errorf("the removed player is still in the list:\n%s", body)
	}
	if !strings.Contains(body, "Game Master") {
		t.Errorf("the GM went with them:\n%s", body)
	}
}

// A PLAYER POSTING THIS IS REFUSED BY THE COMMAND, NOT BY THE HANDLER, and the
// difference is visible: they are a member of the room, so they get the
// protocol's own sentence in the alert modal rather than the 404 a non-member
// gets. The room is left alone.
func TestAPlayerCannotRemoveAnybody(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false),
	}}
	app := liveRoomApp(t, db)

	sess := session.UserSession{UserID: testMemberID, RoomID: &testRoomID}
	rec := kickRequest(t, app, testOwnerID, sess)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	// 403 is in the noSwap list, so the body stays empty and the caller's
	// target is left alone; the message rides the HX-Trigger.
	if body := rec.Body.String(); body != "" {
		t.Errorf("body = %q, want it empty", body)
	}
	if trigger := rec.Header().Get("HX-Trigger"); !strings.Contains(trigger, "alert") {
		t.Errorf("HX-Trigger = %q, want an alert", trigger)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	players, ok := app.Hub.Players(ctx, testRoomID)
	if !ok || len(players) != 2 {
		t.Errorf("the room holds %d players, want both still seated", len(players))
	}
}

// THE GM CANNOT REMOVE THEMSELVES, because a room with nobody who can unlock,
// close or reopen it is a room that has to be abandoned. The refusal is
// PlayerKick.Authorize's, and it reaches the GM as its own sentence.
func TestTheGMCannotRemoveThemselves(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false),
	}}
	app := liveRoomApp(t, db)

	rec := kickRequest(t, app, testOwnerID, session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if trigger := rec.Header().Get("HX-Trigger"); !strings.Contains(trigger, "Close it instead") {
		t.Errorf("HX-Trigger = %q, want the command's own sentence", trigger)
	}
}

// A player id that is not one cannot name anybody, so it is refused before the
// room is asked -- the same shape every other malformed id in this package
// takes.
func TestAMalformedPlayerIsRefused(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false),
	}}
	app := liveRoomApp(t, db)

	rec := roomRequest(t, app.KickPlayer, http.MethodPost, "/rooms/x/players/nonsense/kick",
		map[string]string{"id": testRoomID.String(), "player": "nonsense"},
		session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
