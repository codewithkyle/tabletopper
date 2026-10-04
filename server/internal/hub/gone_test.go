package hub

import (
	"context"
	"testing"

	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)

type bareLibrary struct{}

func nothingThere(heading string) error {
	return &room.Error{Code: room.CodeNotFound, Heading: heading, Message: "That is no longer in your library."}
}
func (bareLibrary) Map(context.Context, ulid.ULID) (room.MapRef, error) {
	return room.MapRef{}, nothingThere("Map gone")
}
func (bareLibrary) Monster(context.Context, ulid.ULID) (room.MonsterInfo, error) {
	return room.MonsterInfo{}, nothingThere("Monster gone")
}
func (bareLibrary) Picture(context.Context, ulid.ULID, room.PictureKind) (room.PictureInfo, error) {
	return room.PictureInfo{}, nothingThere("Picture gone")
}
func (bareLibrary) Track(context.Context, ulid.ULID) (room.TrackInfo, error) {
	return room.TrackInfo{}, nothingThere("Track gone")
}
func (bareLibrary) Character(context.Context, ulid.ULID) (room.CharacterInfo, error) {
	return room.CharacterInfo{}, nothingThere("Character gone")
}
func (bareLibrary) Image(context.Context, ulid.ULID) error {
	return nothingThere("Picture gone")
}
func (tb *tabletop) dirty() bool {
	tb.t.Helper()
	reply := make(chan any, 1)
	if err := tb.actor().post(tb.ctx(), ask{fn: func(a *actor) any { return a.dirty }, reply: reply}); err != nil {
		tb.t.Fatalf("the room would not answer: %v", err)
	}
	dirty, _ := (<-reply).(bool)
	return dirty
}
func TestADeletedCharacterLeavesEveryLiveTable(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	layer := activeLayer(t, only(t, gm, "snapshot")[0])
	tb.seat(playerID, charID, "Ari")
	frames(t, gm)
	spawned := seatPawn(charID, "Ari", 11, 14, 16, room.SizeMedium)
	tb.send(gm, "1", &room.PawnSpawn{
		Kind: room.PawnPlayer, Layer: layer, X: 64, Y: 64, Visible: true,
		CharacterID: &charID, Pawn: &spawned,
	})
	frames(t, gm)
	tb.ForgetCharacter(tb.ctx(), charID)
	tb.settle()
	if _, seated := tb.CharacterPawn(tb.ctx(), roomID, charID); seated {
		t.Error("the character's pawn is still on the table")
	}
	players, _ := tb.Players(tb.ctx(), roomID)
	for _, p := range players {
		if p.ID == playerID && (p.CharacterID != nil || p.CharacterName != "") {
			t.Errorf("Ari's seat still holds %+v", p)
		}
	}
	only(t, gm, "players.upserted", "pawns.removed")
	if !tb.dirty() {
		t.Error("the room was not marked for saving, so a reload brings the character back")
	}
}
func TestForgettingReachesNoRoomThatIsNotLive(t *testing.T) {
	tb := newTabletop(t, Options{})
	tb.ForgetCharacter(tb.ctx(), charID)
	tb.ForgetMonster(tb.ctx(), charID)
	tb.ForgetAsset(tb.ctx(), charID)
	if tb.Loaded() != 0 {
		t.Errorf("forgetting loaded %d rooms; there is nobody at those tables to tell", tb.Loaded())
	}
}
func TestLoadingARoomDropsWhatTheLibraryNoLongerHolds(t *testing.T) {
	s := room.NewState(roomID, "The Sunless Citadel", room.Env{})
	s.Table.Layers[0].Map = &room.MapRef{AssetID: mapAssetID, Gen: mapGenID, Width: 4096, Height: 4096, TileSize: 512, MaxZoom: 3}
	monster, track := testID(72), testID(73)
	s.Pawns = append(s.Pawns, room.Pawn{
		ID: testID(70), Kind: room.PawnMonster, LayerID: s.Table.ActiveLayer, Name: "Goblin",
		Image: room.ImageURL(testID(71)), Size: room.SizeSmall, Visible: true, MonsterID: &monster,
	})
	s.Music = room.Music{TrackID: &track, Name: "Tavern Brawl"}
	s.Normalize()
	blob, err := room.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	store := &memStore{loaded: Loaded{Owner: gmID, Name: "The Sunless Citadel", Snapshot: blob}}
	tb := newTabletop(t, Options{Store: store, Library: func(ulid.ULID) room.Library { return bareLibrary{} }})
	table, ok := tb.Table(tb.ctx(), roomID)
	if !ok {
		t.Fatal("the room would not load")
	}
	if table.Table.Layers[0].Map != nil {
		t.Error("the floor still shows a map that is gone")
	}
	p, ok := tb.Pawn(tb.ctx(), roomID, testID(70), room.RoleGM)
	if !ok {
		t.Fatal("the goblin left the table over its stat block")
	}
	if p.MonsterID != nil || p.Image != "" {
		t.Errorf("the goblin is %+v, want no stat block and no picture", p)
	}
	if m, ok := tb.Music(tb.ctx(), roomID); !ok || m.Music.TrackID != nil {
		t.Error("the music still names a track that is gone")
	}
	if !tb.dirty() {
		t.Error("the pruned room was not marked for saving, so the next load prunes it all over again")
	}
}
func TestLoadingARoomKeepsItWhenTheLibraryIsUnavailable(t *testing.T) {
	s := room.NewState(roomID, "The Sunless Citadel", room.Env{})
	s.Table.Layers[0].Map = &room.MapRef{AssetID: mapAssetID, Gen: mapGenID, Width: 4096, Height: 4096, TileSize: 512, MaxZoom: 3}
	s.Normalize()
	blob, err := room.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	store := &memStore{loaded: Loaded{Owner: gmID, Name: "The Sunless Citadel", Snapshot: blob}}
	tb := newTabletop(t, Options{Store: store})
	table, ok := tb.Table(tb.ctx(), roomID)
	if !ok {
		t.Fatal("the room would not load")
	}
	if table.Table.Layers[0].Map == nil {
		t.Error("a hub with no library pruned the map")
	}
	if tb.dirty() {
		t.Error("a room nothing was done to is marked for saving")
	}
}
func TestASheetSaveForACharacterThatIsGoneClearsItsSeat(t *testing.T) {
	tb := stubbedTabletop(t, noRows())
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	tb.seat(playerID, charID, "Ari")
	frames(t, gm)
	tb.SyncCharacter(tb.ctx(), roomID, playerID, charID)
	tb.settle()
	players, _ := tb.Players(tb.ctx(), roomID)
	for _, p := range players {
		if p.ID == playerID && p.CharacterID != nil {
			t.Errorf("Ari's seat still holds a character the roster no longer has: %+v", p)
		}
	}
	only(t, gm, "players.upserted")
}
