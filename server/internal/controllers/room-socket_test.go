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


func getSocket(t *testing.T, app *App, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()

	return roomRequest(t, app.RoomSocket, http.MethodGet, "/socket/room/"+testRoomID.String(),
		map[string]string{"id": testRoomID.String()}, sess)
}





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




func TestTheSocketAnswers404WithoutAHub(t *testing.T) {
	app := newRoomApp(&roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false),
	}})

	if rec := getSocket(t, app, session.UserSession{UserID: testOwnerID}); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}




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
	
	
	
	
	if !strings.Contains(body, "Ilyana") || !strings.Contains(body, "(ari)") {
		t.Errorf("the fallback list does not name the character and the account:\n%s", body)
	}
	if !strings.Contains(body, "Waiting for the connection.") {
		t.Error("the fallback list does not say that it is not the live one")
	}
	if !strings.Contains(body, `hx-trigger="room:players from:window"`) {
		t.Error("the swapped-in list does not listen for the next player event, so it would refetch once and stop")
	}
	
	
	
	
	if !strings.Contains(body, `hx-sync="this:queue last"`) {
		t.Error("the swapped-in list does not serialise its refetches")
	}
}


func TestTheMembersWindowPrefersTheLiveRoom(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false),
	}}
	app := newRoomApp(db)
	app.Hub = hub.New(nil, hub.Options{Store: emptyRoomStore{}})

	
	
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



type emptyRoomStore struct{}

func (emptyRoomStore) Load(context.Context, ulid.ULID) (hub.Loaded, error) {
	return hub.Loaded{Name: "Curse of Strahd"}, nil
}
func (emptyRoomStore) Save(context.Context, ulid.ULID, []byte, uint64) error { return nil }
func (emptyRoomStore) ClearMembership(context.Context, ulid.ULID, ulid.ULID) error {
	return nil
}
func (emptyRoomStore) Preserve(context.Context, ulid.ULID, []byte) error { return nil }







func kickRequest(t *testing.T, app *App, target ulid.ULID, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()

	return roomRequest(t, app.KickPlayer, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/players/"+target.String()+"/kick",
		map[string]string{"id": testRoomID.String(), "player": target.String()}, sess)
}



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
	
	
	if !strings.Contains(body, `id="room-members"`) {
		t.Errorf("the reply is not the member list:\n%s", body)
	}
	if strings.Contains(body, "Ilyana") {
		t.Errorf("the removed player is still in the list:\n%s", body)
	}
	if !strings.Contains(body, "Game Master") {
		t.Errorf("the GM went with them:\n%s", body)
	}

	
	
	
	
	
	cleared := -1
	for i, call := range db.calls {
		if strings.Contains(call.query, "UPDATE sessions") && strings.Contains(call.query, "user_id = ?") {
			cleared = i
		}
	}
	if cleared < 0 {
		t.Fatalf("the kick did not clear the player's session rows: %v", db.queries())
	}
	assertBoundToRoom(t, db.calls[cleared], testRoomID)
	if user, ok := boundRoomID(db.calls[cleared].args[1]); !ok || user != testMemberID {
		t.Errorf("the clear is not scoped to the kicked player: %v", db.calls[cleared].args)
	}
}





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
