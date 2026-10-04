package room

import (
	"slices"
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
)

func TestADeletedCharacterLeavesItsSeatAndTakesItsPawn(t *testing.T) {
	w := newWorld(t)
	ari := w.spawn(Pawn{
		Kind: PawnPlayer, Name: "Ari", X: 96, Y: 96, Visible: true,
		OwnerID: &testPlayerID, CharacterID: &testCharID,
		HP: intp(11), MaxHP: intp(14), AC: intp(16),
	})
	goblin := w.spawn(Pawn{
		Kind: PawnMonster, Name: "Goblin", X: 160, Y: 96, Visible: true,
		HP: intp(7), MaxHP: intp(7), AC: intp(15), MonsterID: idp(testMonsterID),
	})
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Ari", PawnIDs: []ulid.ULID{ari}, Initiative: 18},
		{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}, Initiative: 12},
	}}, w.gm)
	ch := w.change(&CharacterGone{ID: testCharID}, w.gm)
	if w.s.Pawn(ari) != nil {
		t.Fatal("Ari's pawn is still on the table")
	}
	if w.s.Pawn(goblin) == nil {
		t.Fatal("the goblin went too")
	}
	seat := w.s.Player(testPlayerID)
	if seat == nil || seat.CharacterID != nil || seat.CharacterName != "" {
		t.Errorf("Ari's seat is %+v, want one with no character", seat)
	}
	if rin := w.s.Player(testOtherID); rin == nil || rin.CharacterID == nil || *rin.CharacterID != testOtherChar {
		t.Error("Rin lost her character too")
	}
	for _, e := range w.s.Initiative.Entries {
		if e.Name == "Ari" {
			t.Error("Ari's turn survived her pawn")
		}
	}
	want := []string{"players.upserted", "pawns.removed", "initiative.updated"}
	if got := changeTypesOf(ch.changes(RoleGM)); !slices.Equal(got, want) {
		t.Errorf("the GM was told %v, want %v", got, want)
	}
	if got := changeTypesOf(ch.changes(RolePlayer)); !slices.Equal(got, want) {
		t.Errorf("the players were told %v, want %v", got, want)
	}
}
func TestADeletedCharacterNobodyBroughtChangesNothing(t *testing.T) {
	w := newWorld(t)
	before := mustJSON(t, w.s)
	ch := w.change(&CharacterGone{ID: testID(4242)}, w.gm)
	if got := mustJSON(t, w.s); got != before {
		t.Errorf("the room changed:\n got %s\nwant %s", got, before)
	}
	if got := ch.changes(RoleGM); len(got) != 0 {
		t.Errorf("the GM was told %v about a character nobody brought", changeTypesOf(got))
	}
}
func TestADeletedMonsterLeavesItsPawnsWithoutAStatBlock(t *testing.T) {
	w := newWorld(t)
	goblin := w.spawn(Pawn{
		Kind: PawnMonster, Name: "Goblin", X: 160, Y: 96, Visible: true,
		HP: intp(7), MaxHP: intp(7), AC: intp(15), MonsterID: idp(testMonsterID), Image: ImageURL(testAssetID),
	})
	hiding := w.spawn(Pawn{
		Kind: PawnMonster, Name: "Goblin", X: 260, Y: 96, Visible: false,
		HP: intp(7), MaxHP: intp(7), AC: intp(15), MonsterID: idp(testMonsterID),
	})
	wolf := w.spawn(Pawn{
		Kind: PawnMonster, Name: "Wolf", X: 360, Y: 96, Visible: true,
		HP: intp(11), MaxHP: intp(11), AC: intp(13), MonsterID: idp(testID(61)),
	})
	ch := w.change(&MonsterGone{ID: testMonsterID}, w.gm)
	for _, id := range []ulid.ULID{goblin, hiding} {
		p := w.s.Pawn(id)
		if p == nil {
			t.Fatal("a goblin left the table with its stat block")
		}
		if p.MonsterID != nil {
			t.Error("a goblin still names a monster that is gone")
		}
		if p.Name != "Goblin" || *p.HP != 7 || *p.AC != 15 {
			t.Errorf("the goblin lost more than its stat block: %+v", p)
		}
	}
	if p := w.s.Pawn(goblin); p.Image != ImageURL(testAssetID) {
		t.Errorf("the goblin's picture is %q; the monster going is not the picture going", p.Image)
	}
	if p := w.s.Pawn(wolf); p == nil || p.MonsterID == nil {
		t.Error("the wolf lost its stat block too")
	}
	if got := changeTypesOf(ch.changes(RoleGM)); !slices.Equal(got, []string{"pawns.upserted"}) {
		t.Errorf("the GM was told %v, want the two goblins upserted", got)
	}
	if got := changeTypesOf(ch.changes(RolePlayer)); !slices.Equal(got, []string{"pawns.upserted"}) {
		t.Errorf("the players were told %v, want the visible goblin upserted", got)
	}
}
func TestADeletedAssetIsForgottenEverywhereItWasUsed(t *testing.T) {
	w := sceneWorld(t)
	cellar := floorCalled(w.s, "Cellar").ID
	w.apply(&TableSetLayerMap{
		Layer:   cellar,
		AssetID: testAssetID,
		GM:      true,
		Map:     &MapRef{AssetID: testAssetID, Gen: testID(50), Width: 4096, Height: 4096, TileSize: 512, MaxZoom: 3},
	}, w.gm)
	token := testID(1300)
	crate := w.spawn(Pawn{Kind: PawnObject, Name: "Crate", Width: 64, Height: 64, X: 10, Y: 10, Visible: true, Image: ImageURL(token)})
	barrel := w.spawn(Pawn{Kind: PawnObject, Name: "Barrel", Width: 64, Height: 64, X: 90, Y: 10, Visible: true, Image: ImageURL(testID(1301))})
	pine := w.s.Table.Palette[0].ID

	ch := w.change(&AssetGone{ID: testAssetID}, w.gm)
	if l := floorCalled(w.s, DefaultLayerName); l.Map != nil {
		t.Error("the ground floor still shows a map that is gone")
	}
	if l := floorCalled(w.s, "Cellar"); l.GMMap != nil {
		t.Error("the cellar's GM map still points at a map that is gone")
	} else if l.Map == nil {
		t.Error("the cellar lost a map that is still in the library")
	}
	if got := changeTypesOf(ch.changes(RoleGM)); !slices.Equal(got, []string{"layers.updated"}) {
		t.Errorf("the GM was told %v, want the layers", got)
	}

	ch = w.change(&AssetGone{ID: testTerrainID}, w.gm)
	if len(w.s.Table.Palette) != 1 || w.s.Table.Palette[0].Name != "Rolling hills" {
		t.Errorf("the palette holds %+v, want the hills alone", w.s.Table.Palette)
	}
	for _, tile := range w.s.Tiles {
		if tile.Art == pine {
			t.Error("a tile still names the pines, so it draws nothing")
		}
	}
	if len(w.s.Tiles) != 1 {
		t.Errorf("%d tiles survived, want the one hill", len(w.s.Tiles))
	}
	if got := changeTypesOf(ch.changes(RoleGM)); !slices.Equal(got, []string{"tiles.erased", "palette.updated"}) {
		t.Errorf("the GM was told %v, want the tiles and the palette", got)
	}

	ch = w.change(&AssetGone{ID: token}, w.gm)
	if p := w.s.Pawn(crate); p.Image != "" {
		t.Errorf("the crate still shows %q", p.Image)
	}
	if p := w.s.Pawn(barrel); p.Image != ImageURL(testID(1301)) {
		t.Errorf("the barrel lost its picture too: %q", p.Image)
	}
	if got := changeTypesOf(ch.changes(RolePlayer)); !slices.Equal(got, []string{"pawns.upserted"}) {
		t.Errorf("the players were told %v, want the crate upserted", got)
	}

	w.apply(&MusicSetLoop{Loop: true}, w.gm)
	ch = w.change(&AssetGone{ID: testTrackID}, w.gm)
	if w.s.Music.TrackID != nil || w.s.Music.Playing || w.s.Music.Name != "" {
		t.Errorf("the music is %+v after its track was deleted", w.s.Music)
	}
	if !w.s.Music.Loop {
		t.Error("the repeat setting was lost with the track")
	}
	if got := changeTypesOf(ch.changes(RoleGM)); !slices.Equal(got, []string{"music.updated"}) {
		t.Errorf("the GM was told %v, want the music", got)
	}
}
func TestTheGoneCommandsAreServerSideOnly(t *testing.T) {
	for _, name := range []string{"character.gone", "monster.gone", "asset.gone"} {
		if _, hub := HubCommandPrototypes()[name]; !hub {
			t.Fatalf("%s is not in the hub registry", name)
		}
		_, _, err := DecodeCommand([]byte(`{"type":"` + name + `","cid":"1","id":"` + testCharID.String() + `"}`))
		e, ok := err.(*Error)
		if !ok || !strings.Contains(e.Message, "does not know") {
			t.Fatalf("a socket asked for %s and got %v; a player could empty the table of anything by naming it", name, err)
		}
	}
}
func TestImageURLsRoundTrip(t *testing.T) {
	id, ok := imageAsset(ImageURL(testAssetID))
	if !ok || id != testAssetID {
		t.Fatalf("imageAsset(ImageURL(x)) = %v, %v", id, ok)
	}
	if id, ok := imageAsset(ImageURL(testAssetID) + "?fresh"); !ok || id != testAssetID {
		t.Errorf("a cache-busting query hid the asset: %v, %v", id, ok)
	}
	for _, url := range []string{"", DefaultAvatar, "/assets/images/", "/assets/images/not-a-ulid", "/images/goblin.webp"} {
		if _, ok := imageAsset(url); ok {
			t.Errorf("%q was read as a library picture", url)
		}
	}
}
