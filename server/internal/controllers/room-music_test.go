package controllers

import (
	"context"
	"database/sql/driver"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"tabletopper/internal/hub"
	"tabletopper/internal/room"
	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

var testTrackID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVY0")

func musicApp(t *testing.T, answers ...roomAnswer) *App {
	t.Helper()
	app := newRoomApp(&roomDB{rows: 1, answers: answers})
	app.Hub = hub.New(app.Queries, hub.Options{Store: emptyRoomStore{}})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	gm := room.Actor{ID: testOwnerID, Role: room.RoleGM}
	for _, p := range []room.Player{
		{ID: testOwnerID, Name: "kyle", Role: room.RoleGM},
		{ID: testMemberID, Name: "ari", CharacterName: "Ilyana", Role: room.RolePlayer},
	} {
		if err := app.Hub.Dispatch(ctx, testRoomID, gm, &room.PlayerJoin{Player: p}); err != nil {
			t.Fatalf("seating %s: %v", p.Name, err)
		}
	}
	return app
}
func theRoom() roomAnswer {
	return getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false)
}
func aTrack(name string) roomAnswer {
	return roomAnswer{
		columns: []string{
			"id", "owner_id", "journal_id", "file_path", "preview_path", "type",
			"file_name", "size_bytes", "name", "detached_at", "width", "height",
			"tile_size", "max_zoom", "tile_gen", "tile_state", "tile_attempts",
			"tile_lease", "tile_leased_at", "tiled_at", "uploaded_at",
			"created_at", "updated_at",
		},
		values: []driver.Value{
			testTrackID.Bytes(), testOwnerID.Bytes(), nil, "users/music/x", nil, "music",
			"tavern.mp3", int64(4 << 20), name, nil, nil, nil,
			nil, nil, nil, nil, int64(0),
			nil, nil, nil, time.Unix(0, 0),
			time.Unix(0, 0), time.Unix(0, 0),
		},
	}
}
func musicWindowFor(t *testing.T, app *App, sess session.UserSession) string {
	t.Helper()
	rec := tableRequest(t, app.RoomMusicFragment, http.MethodGet,
		"/fragment/room/music?room="+testRoomID.String(), nil, nil, sess)
	if rec.Code != http.StatusOK {
		t.Fatalf("the window returned %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}
func musicPost(t *testing.T, app *App, handler http.HandlerFunc, path string, form url.Values, sess session.UserSession) int {
	t.Helper()
	rec := tableRequest(t, handler, http.MethodPost, "/rooms/"+testRoomID.String()+path,
		map[string]string{"id": testRoomID.String()}, form, sess)
	return rec.Code
}

func TestTheWindowIsEverybodysAndTheLibraryIsTheGMsAlone(t *testing.T) {
	app := musicApp(t, theRoom(), aTrack("Tavern Brawl"), theRoom())
	gm := musicWindowFor(t, app, session.UserSession{UserID: testOwnerID})
	if !strings.Contains(gm, "data-music-search") {
		t.Errorf("the GM was not sent the library:\n%s", gm)
	}
	player := musicWindowFor(t, app, memberSession(testRoomID))
	if !strings.Contains(player, "data-music-bar") {
		t.Errorf("a player was not sent the progress bar:\n%s", player)
	}
	if strings.Contains(player, "data-music-search") || strings.Contains(player, "hx-post") {
		t.Errorf("a player was sent controls that are the GM's:\n%s", player)
	}
}
func TestTheGMPutsATrackOnAndTheWindowSaysSo(t *testing.T) {
	app := musicApp(t, theRoom(), aTrack("Tavern Brawl"), theRoom(), aTrack("Tavern Brawl"))
	form := url.Values{"track": {testTrackID.String()}}
	if code := musicPost(t, app, app.LoadRoomMusic, "/music", form, session.UserSession{UserID: testOwnerID}); code != http.StatusNoContent {
		t.Fatalf("loading the track returned %d, want 204", code)
	}
	markup := musicWindowFor(t, app, session.UserSession{UserID: testOwnerID})
	if !strings.Contains(markup, "Tavern Brawl") {
		t.Errorf("the window does not name what is playing:\n%s", markup)
	}
	if !strings.Contains(markup, "/music/pause") {
		t.Errorf("a track that was just put on is not offering a pause:\n%s", markup)
	}
}
func TestOnlyTheGMRunsTheMusic(t *testing.T) {
	app := musicApp(t, theRoom(), theRoom())
	for name, post := range map[string]func(http.ResponseWriter, *http.Request){
		"play":  app.PlayRoomMusic,
		"pause": app.PauseRoomMusic,
	} {
		code := musicPost(t, app, post, "/music/"+name, nil, memberSession(testRoomID))
		if code != http.StatusForbidden {
			t.Errorf("a player's %s returned %d, want 403", name, code)
		}
	}
}
func TestStartingWithNothingLoadedIsRefusedWithTheReasonWhy(t *testing.T) {
	app := musicApp(t, theRoom())
	rec := tableRequest(t, app.PlayRoomMusic, http.MethodPost, "/rooms/"+testRoomID.String()+"/music/play",
		map[string]string{"id": testRoomID.String()}, nil, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("HX-Trigger"), "No track") {
		t.Errorf("the refusal does not say what is missing: %q", rec.Header().Get("HX-Trigger"))
	}
}
func TestTheAudioIsServedForTheLoadedTrackAndNoOther(t *testing.T) {
	app := musicApp(t, theRoom())
	for name, query := range map[string]string{
		"nothing loaded": "?track=" + testTrackID.String(),
		"no track named": "",
		"not a ulid":     "?track=nonsense",
	} {
		rec := tableRequest(t, app.RoomMusicAudio, http.MethodGet,
			"/rooms/"+testRoomID.String()+"/music/audio"+query,
			map[string]string{"id": testRoomID.String()}, nil, session.UserSession{UserID: testOwnerID})
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s answered %d, want 404", name, rec.Code)
		}
	}
}
