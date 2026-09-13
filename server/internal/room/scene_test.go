package room

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
)

var sceneCellarAsset = testID(1009)

func sceneWorld(t *testing.T) *world {
	t.Helper()
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	w.apply(&TableSetLayerMap{
		Layer:   w.layer,
		AssetID: testAssetID,
		Map:     &MapRef{AssetID: testAssetID, Gen: testID(50), Width: 4096, Height: 4096, TileSize: 512, MaxZoom: 3},
	}, w.gm)
	w.apply(&TableSetLayerMap{
		Layer:   cellar,
		AssetID: sceneCellarAsset,
		Map:     &MapRef{AssetID: sceneCellarAsset, Gen: testID(51), Width: 2048, Height: 2048, TileSize: 512, MaxZoom: 2},
	}, w.gm)
	grid := w.s.Table.Grid
	grid.CellSize = 70
	grid.OffsetX, grid.OffsetY = -12, 8
	grid.FeetPerCell = 5
	w.apply(&TableSetGrid{Grid: grid}, w.gm)
	w.apply(&TableSetOptions{
		PawnLabels:         LabelsFull,
		PlayersCanDraw:     true,
		InitiativeGrouping: GroupMonsters,
		FogPrefill:         true,
	}, w.gm)
	ari := w.spawn(Pawn{
		Kind: PawnPlayer, Name: "Ari", X: 96, Y: 96, Visible: true,
		OwnerID: &testPlayerID, CharacterID: &testCharID,
		HP: intp(11), MaxHP: intp(14), AC: intp(16),
	})
	goblin := w.spawn(Pawn{
		Kind: PawnMonster, Name: "Goblin", X: 160, Y: 96, Visible: true,
		HP: intp(7), MaxHP: intp(7), AC: intp(15), MonsterID: idp(testID(60)),
	})
	w.spawn(Pawn{Kind: PawnObject, Name: "Wagon", Width: 128, Height: 256, X: 128, Y: 128, Visible: true})
	w.apply(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogHide, Points: []int{0, 0, 512, 512}}, w.gm)
	w.apply(&FogAdd{Layer: cellar, Kind: ShapePoly, Mode: FogReveal, Points: []int{64, 64, 192, 64, 192, 192}}, w.gm)
	w.apply(&StrokeBegin{ID: testID(800), Layer: w.layer, Kind: StrokeFree, Color: "#ff0000ff", Width: 4, Points: []int{0, 0, 8, 8}}, w.gm)
	w.apply(&StrokeEnd{ID: testID(800)}, w.gm)
	w.apply(&StrokeBegin{ID: testID(801), Layer: w.layer, Kind: StrokeFree, Color: "#00ff00ff", Width: 2, Points: []int{4, 4, 12, 12}}, w.pc)
	w.apply(&StrokeEnd{ID: testID(801)}, w.pc)
	w.apply(&StrokeBegin{ID: testID(802), Layer: cellar, Kind: StrokeFree, Color: "#0000ffff", Width: 2, Points: []int{4, 4}}, w.gm)
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Ari", PawnIDs: []ulid.ULID{ari}, Initiative: 18},
		{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}, Initiative: 12},
	}}, w.gm)
	w.apply(&DiceRoll{Expr: "1d20 + 7", Label: "Longsword"}, w.pc)
	w.apply(&MusicLoad{AssetID: testTrackID, Track: &TrackInfo{Name: "Tavern Brawl"}}, w.gm)
	w.apply(&RoomSetLocked{Locked: true}, w.gm)
	w.s.Seq = 42
	return w
}
func liveWorld(t *testing.T) *world {
	t.Helper()
	w := newWorld(t)
	w.apply(&TableSetOptions{
		PawnLabels:         LabelsNone,
		PlayersCanDraw:     false,
		InitiativeGrouping: GroupIndividual,
		FogPrefill:         false,
	}, w.gm)
	grid := w.s.Table.Grid
	grid.CellSize = 100
	grid.FeetPerCell = 10
	w.apply(&TableSetGrid{Grid: grid}, w.gm)
	w.spawn(Pawn{
		Kind: PawnPlayer, Name: "Rin", X: 32, Y: 32, Visible: true,
		OwnerID: &testOtherID, CharacterID: &testOtherChar,
		HP: intp(9), MaxHP: intp(9), AC: intp(14),
	})
	w.apply(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogHide, Points: []int{0, 0, 64, 64}}, w.gm)
	w.apply(&StrokeBegin{ID: testID(900), Layer: w.layer, Kind: StrokeFree, Color: "#ffffffff", Width: 2, Points: []int{1, 1, 2, 2}}, w.gm)
	w.apply(&StrokeEnd{ID: testID(900)}, w.gm)
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{{Name: "Rin", Initiative: 7}}}, w.gm)
	w.apply(&DiceRoll{Expr: "1d6", Label: "Sneak"}, w.other)
	w.apply(&MusicLoad{AssetID: testTrackID, Track: &TrackInfo{Name: "Rain on the roof"}}, w.gm)
	w.apply(&RoomSetName{Name: "Keep on the Borderlands"}, w.gm)
	return w
}
func TestASceneCarriesThePlaceIntoAnotherRoom(t *testing.T) {
	body, err := sceneWorld(t).s.ExportScene()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	scene, err := Unmarshal(body)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	layers := mustJSON(t, scene.Table.Layers)
	active := scene.Table.ActiveLayer
	grid := scene.Table.Grid
	fog := mustJSON(t, scene.Fog)
	strokes := mustJSON(t, scene.Strokes)
	pawns := mustJSON(t, scene.Pawns)

	live := liveWorld(t)
	players := mustJSON(t, live.s.Players)
	rolls := mustJSON(t, live.s.Rolls)
	music := mustJSON(t, live.s.Music)
	info := live.s.Room
	labels := live.s.Table.PawnLabels
	canDraw := live.s.Table.PlayersCanDraw
	grouping := live.s.Table.InitiativeGrouping
	prefill := live.s.Table.FogPrefill

	live.s.ImportScene(scene)

	if got := mustJSON(t, live.s.Table.Layers); got != layers {
		t.Errorf("the layers did not cross:\n got %s\nwant %s", got, layers)
	}
	if live.s.Table.ActiveLayer != active {
		t.Errorf("active layer = %s, want %s", live.s.Table.ActiveLayer, active)
	}
	if live.s.Table.Grid != grid {
		t.Errorf("grid = %+v, want %+v", live.s.Table.Grid, grid)
	}
	if got := mustJSON(t, live.s.Fog); got != fog {
		t.Errorf("the fog did not cross:\n got %s\nwant %s", got, fog)
	}
	if got := mustJSON(t, live.s.Strokes); got != strokes {
		t.Errorf("the drawing did not cross:\n got %s\nwant %s", got, strokes)
	}
	if got := mustJSON(t, live.s.Pawns); got != pawns {
		t.Errorf("the pawns did not cross:\n got %s\nwant %s", got, pawns)
	}
	for _, name := range []string{"Goblin", "Wagon"} {
		if !hasPawnNamed(live.s, name) {
			t.Errorf("the %s did not arrive", name)
		}
	}
	for _, p := range live.s.Pawns {
		if p.Kind == PawnPlayer || p.CharacterID != nil || p.OwnerID != nil {
			t.Errorf("%q is somebody's pawn and it is on the table after a load", p.Name)
		}
	}
	if got := mustJSON(t, live.s.Players); got != players {
		t.Errorf("the room's players changed:\n got %s\nwant %s", got, players)
	}
	if got := mustJSON(t, live.s.Rolls); got != rolls {
		t.Errorf("the room's dice log changed:\n got %s\nwant %s", got, rolls)
	}
	if got := mustJSON(t, live.s.Music); got != music {
		t.Errorf("the room's music changed:\n got %s\nwant %s", got, music)
	}
	if live.s.Table.PawnLabels != labels || live.s.Table.PlayersCanDraw != canDraw ||
		live.s.Table.InitiativeGrouping != grouping || live.s.Table.FogPrefill != prefill {
		t.Errorf("a scene load reset how the GM runs their table: %+v", live.s.Table.TableSettings)
	}
	if len(live.s.Initiative.Entries) != 0 || live.s.Initiative.Active != nil {
		t.Errorf("the turn order survived a scene load: %+v", live.s.Initiative)
	}
	if live.s.Room != info {
		t.Errorf("the room took on the name or the lock of the scene: %+v, want %+v", live.s.Room, info)
	}
	if got := mustJSON(t, scene.Table.Layers); got != layers {
		t.Errorf("importing a scene changed the scene it read from")
	}
}
func TestAnExportedSceneHoldsNothingAboutAnybody(t *testing.T) {
	w := sceneWorld(t)
	body, err := w.s.ExportScene()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	scene, err := Unmarshal(body)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(scene.Players) != 0 {
		t.Errorf("the scene names %d players", len(scene.Players))
	}
	for _, p := range scene.Pawns {
		if p.Kind == PawnPlayer {
			t.Errorf("%q is a character pawn and it is in the scene", p.Name)
		}
		if p.OwnerID != nil {
			t.Errorf("%q is owned by somebody in the scene", p.Name)
		}
		if p.CharacterID != nil {
			t.Errorf("%q names a character sheet in the scene", p.Name)
		}
	}
	for _, st := range scene.Strokes {
		if !st.By.IsZero() {
			t.Errorf("a stroke in the scene remembers %s drew it", st.By)
		}
		if !st.Done {
			t.Errorf("a stroke in the scene was never finished, so nobody can end it")
		}
	}
	if scene.Seq != 0 {
		t.Errorf("seq = %d, want 0", scene.Seq)
	}
	if scene.Room != (RoomInfo{}) {
		t.Errorf("the scene carries the room it was saved from: %+v", scene.Room)
	}
	if len(scene.Initiative.Entries) != 0 || scene.Initiative.Active != nil {
		t.Errorf("the scene carries a turn order: %+v", scene.Initiative)
	}
	if len(scene.Rolls) != 0 {
		t.Errorf("the scene carries %d rolls", len(scene.Rolls))
	}
	if scene.Music != (Music{}) {
		t.Errorf("the scene carries the music that was playing: %+v", scene.Music)
	}
	text := string(body)
	for who, id := range map[string]ulid.ULID{
		"the GM":            testGMID,
		"Ari":               testPlayerID,
		"Rin":               testOtherID,
		"Ari's character":   testCharID,
		"Rin's character":   testOtherChar,
		"the room it is in": testRoomID,
	} {
		if strings.Contains(text, id.String()) {
			t.Errorf("the scene body still mentions %s (%s)", who, id)
		}
	}
	if strings.Contains(text, w.s.Room.Name) {
		t.Errorf("the scene body still names the room it was saved from")
	}
}
func TestAnExportedSceneReadsBackThroughTheSnapshotReader(t *testing.T) {
	body, err := sceneWorld(t).s.ExportScene()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	var envelope struct {
		Schema int `json:"schema"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("the scene body is not an object: %v", err)
	}
	if envelope.Schema != Schema {
		t.Fatalf("the scene body is stamped schema %d, want %d, so the migration chain would skip it", envelope.Schema, Schema)
	}
	scene, err := Unmarshal(body)
	if err != nil {
		t.Fatalf("a scene body does not read back the way a room snapshot does: %v", err)
	}
	again, err := scene.ExportScene()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if string(again) != string(body) {
		t.Fatalf("a scene changed on the way through:\n%s\n%s", body, again)
	}
}
func TestASceneWithoutItsActiveLayerLandsOnTheFirstOne(t *testing.T) {
	body, err := sceneWorld(t).s.ExportScene()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	scene, err := Unmarshal(body)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	scene.Table.ActiveLayer = testID(7777)
	live := liveWorld(t)
	live.s.ImportScene(scene)
	if live.s.Table.ActiveLayer != scene.Table.Layers[0].ID {
		t.Fatalf("active layer = %s, want the first floor %s", live.s.Table.ActiveLayer, scene.Table.Layers[0].ID)
	}
}
func hasPawnNamed(s *State, name string) bool {
	for _, p := range s.Pawns {
		if p.Name == name {
			return true
		}
	}
	return false
}
