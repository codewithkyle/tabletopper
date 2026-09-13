package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"tabletopper/internal/hub"
	"tabletopper/internal/room"
	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

type seededRoomStore struct{ snapshot json.RawMessage }

func (s seededRoomStore) Load(context.Context, ulid.ULID) (hub.Loaded, error) {
	return hub.Loaded{Name: "Curse of Strahd", Snapshot: s.snapshot}, nil
}
func (seededRoomStore) Save(context.Context, ulid.ULID, []byte, uint64) error { return nil }
func (seededRoomStore) ClearMembership(context.Context, ulid.ULID, ulid.ULID) error {
	return nil
}
func (seededRoomStore) Preserve(context.Context, ulid.ULID, []byte) error { return nil }
func (seededRoomStore) AutosaveScene(context.Context, ulid.ULID, []byte, *ulid.ULID) error {
	return nil
}
func stampingApp(t *testing.T, on bool) *App {
	t.Helper()
	s := room.NewState(testRoomID, "Curse of Strahd", room.Env{})
	s.Table.PlayersCanStamp = on
	s.Table.Palette = []room.TileArt{
		{ID: testArtID, AssetID: testAssetID, Name: "Pine forest", Image: "/assets/images/" + testAssetID.String()},
	}
	body, err := room.Marshal(s)
	if err != nil {
		t.Fatalf("seeding the room: %v", err)
	}
	app := newRoomApp(&roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer(), tableRoomAnswer()}})
	app.Hub = hub.New(app.Queries, hub.Options{Store: seededRoomStore{snapshot: body}})
	return app
}
func TestTheRingIsThePaletteInTheOrderTheGMBuiltIt(t *testing.T) {
	palette := []room.TileArt{
		{ID: testArtID, AssetID: testAssetID, Name: "Pine forest", Image: "/assets/images/a"},
		{ID: testRoomID, AssetID: testOwnerID, Name: "Rolling hills", Image: "/assets/images/b"},
	}
	ring := tableMenuRing(palette)
	if len(ring) != 2 {
		t.Fatalf("the ring carries %d pictures, want 2", len(ring))
	}
	for i, art := range ring {
		if art.Index != i || art.Count != 2 {
			t.Errorf("picture %d is %d of %d, want %d of 2", i, art.Index, art.Count, i)
		}
	}
	if ring[0].Name != "Pine forest" || ring[1].Image != "/assets/images/b" {
		t.Errorf("the ring is not the palette: %+v", ring)
	}
	if got := tableMenuRing(nil); len(got) != 0 {
		t.Errorf("an empty palette made a ring of %d", len(got))
	}
}
func TestAPlayersRingIsTheGMsOwnAndOnlyOnceStampingIsOn(t *testing.T) {
	table := room.Table{Palette: []room.TileArt{
		{ID: testArtID, Name: "Pine forest", Image: "/assets/images/a"},
		{ID: testRoomID, Name: "Rolling hills", Image: "/assets/images/b"},
	}}
	if got := tableMenuRingFor(false, table); len(got) != 0 {
		t.Errorf("a player was handed %d pictures while stamping is off", len(got))
	}
	if got := tableMenuRingFor(true, table); len(got) != 2 {
		t.Errorf("the GM's ring is %d pictures while stamping is off; the setting is the players'", len(got))
	}
	table.PlayersCanStamp = true
	got, want := tableMenuRingFor(false, table), tableMenuRingFor(true, table)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("a player's ring is %+v, want the GM's own %+v", got, want)
	}
}
func TestTheWheelAPlayerIsServedFollowsTheSetting(t *testing.T) {
	for name, tc := range map[string]struct {
		on    bool
		sess  session.UserSession
		stamp bool
	}{
		"a player while stamping is off": {false, memberSession(testRoomID), false},
		"a player while stamping is on":  {true, memberSession(testRoomID), true},
		"the GM while stamping is off":   {false, session.UserSession{UserID: testOwnerID}, true},
	} {
		t.Run(name, func(t *testing.T) {
			app := stampingApp(t, tc.on)
			rec := tableRequest(t, app.RoomTableMenuFragment, http.MethodGet,
				"/fragment/room/table-menu?room="+testRoomID.String(), nil, nil, tc.sess)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if got := strings.Contains(body, "data-table-menu-art="); got != tc.stamp {
				t.Errorf("the ring is there: %v, want %v:\n%s", got, tc.stamp, body)
			}
			if got := strings.Contains(body, "data-table-menu-erase"); got != tc.stamp {
				t.Errorf("the eraser is there: %v, want %v:\n%s", got, tc.stamp, body)
			}
			isGM := tc.sess.UserID == testOwnerID
			if got := strings.Contains(body, "data-table-menu-party"); got != isGM {
				t.Errorf("the party start is there: %v, want %v:\n%s", got, isGM, body)
			}
		})
	}
}
