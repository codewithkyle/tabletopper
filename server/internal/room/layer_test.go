package room

import (
	"testing"

	"github.com/oklog/ulid/v2"
)

func TestRemovingALayerEmptiesItInOrder(t *testing.T) {
	w := newWorld(t)
	ground := w.layer
	cellar := w.addLayer("Cellar")
	w.apply(&TableSetActiveLayer{Layer: cellar}, w.gm)
	w.spawn(Pawn{Name: "Upstairs", LayerID: ground, Visible: true})
	goblin := w.spawn(Pawn{Name: "Goblin", LayerID: cellar, Visible: true})
	w.spawn(Pawn{Name: "Ambusher", LayerID: cellar, Visible: false})
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}}}}, w.gm)
	w.apply(&FogAdd{Layer: cellar, Kind: ShapeRect, Mode: FogHide, Points: []int{0, 0, 64, 64}}, w.gm)
	w.apply(&StrokeBegin{ID: testID(700), Layer: cellar, Kind: StrokeFree, Color: "#ffffff", Width: 2, Points: []int{0, 0}}, w.gm)
	ch := w.change(&TableRemoveLayer{Layer: cellar}, w.gm)
	equalStrings(t, "the GM", eventTypesOf(ch.events(RoleGM)), []string{
		"fog.removed",
		"stroke.erased",
		"pawn.removed",
		"pawn.removed",
		"initiative.updated",
		"table.updated",
	})
	equalStrings(t, "the players", eventTypesOf(ch.events(RolePlayer)), []string{
		"fog.removed",
		"stroke.erased",
		"pawn.removed",
		"pawn.spawned",
		"initiative.updated",
		"table.updated",
	})
	if w.s.Table.ActiveLayer != ground {
		t.Fatal("the active layer did not fall back to the remaining floor")
	}
	if len(w.s.Pawns) != 1 || w.s.Pawns[0].Name != "Upstairs" {
		t.Fatalf("the room holds %d pawns after the removal", len(w.s.Pawns))
	}
	if len(w.s.Fog) != 0 || len(w.s.Strokes) != 0 {
		t.Fatal("the removed layer's fog or strokes outlived it")
	}
	if len(w.s.Initiative.Entries) != 0 {
		t.Fatal("the tracker still names a pawn that was deleted with its layer")
	}
}
func TestRemovingAnInactiveLayerDoesNotMoveAnybody(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	w.spawn(Pawn{Name: "Upstairs", Visible: true})
	ch := w.change(&TableRemoveLayer{Layer: cellar}, w.gm)
	equalStrings(t, "the GM", eventTypesOf(ch.events(RoleGM)), []string{"table.updated"})
	if w.s.Table.ActiveLayer != w.layer {
		t.Fatal("removing another floor moved the active layer")
	}
}
func TestTheLastLayerCannotBeRemoved(t *testing.T) {
	w := newWorld(t)
	w.refuse(&TableRemoveLayer{Layer: w.layer}, w.gm, CodeInvalid)
	w.refuse(&TableRemoveLayer{Layer: testID(999)}, w.gm, CodeNotFound)
}
func TestClearingTheTabletopEmptiesEveryFloor(t *testing.T) {
	w := newWorld(t)
	ground := w.layer
	cellar := w.addLayer("Cellar")
	goblin := w.spawn(Pawn{Name: "Goblin", LayerID: ground, Visible: true})
	w.spawn(Pawn{Name: "Ambusher", LayerID: ground, Visible: false})
	w.spawn(Pawn{Name: "Downstairs", LayerID: cellar, Visible: true})
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}}}}, w.gm)
	w.apply(&FogAdd{Layer: ground, Kind: ShapeRect, Mode: FogHide, Points: []int{0, 0, 64, 64}}, w.gm)
	w.apply(&StrokeBegin{ID: testID(701), Layer: cellar, Kind: StrokeFree, Color: "#ffffff", Width: 2, Points: []int{0, 0}}, w.gm)
	w.apply(&TableSetLayerMap{
		Layer:   ground,
		AssetID: testAssetID,
		Map:     &MapRef{AssetID: testAssetID, Gen: testID(50), Width: 2048, Height: 2048, TileSize: 512, MaxZoom: 2},
	}, w.gm)
	cell := w.s.Table.Grid.CellSize
	ch := w.change(&TableClear{}, w.gm)
	equalStrings(t, "the GM", eventTypesOf(ch.events(RoleGM)), []string{
		"fog.removed",
		"stroke.erased",
		"pawn.removed",
		"pawn.removed",
		"pawn.removed",
		"initiative.updated",
		"table.updated",
	})
	equalStrings(t, "the players", eventTypesOf(ch.events(RolePlayer)), []string{
		"fog.removed",
		"stroke.erased",
		"pawn.removed",
		"initiative.updated",
		"table.updated",
	})
	if len(w.s.Pawns) != 0 || len(w.s.Fog) != 0 || len(w.s.Strokes) != 0 {
		t.Fatalf("the table still holds %d pawns, %d fog shapes and %d strokes",
			len(w.s.Pawns), len(w.s.Fog), len(w.s.Strokes))
	}
	if len(w.s.Initiative.Entries) != 0 {
		t.Fatal("the tracker outlived the pawns it named")
	}
	if len(w.s.Table.Layers) != 2 {
		t.Fatalf("the room has %d layers after a clear, want both", len(w.s.Table.Layers))
	}
	for _, l := range w.s.Table.Layers {
		if l.Map != nil {
			t.Errorf("the %s layer kept its map", l.Name)
		}
	}
	if w.s.Table.Grid.CellSize != cell {
		t.Error("clearing the table changed the grid")
	}
}
func TestOnlyTheGMClearsTheTabletop(t *testing.T) {
	w := newWorld(t)
	w.refuse(&TableClear{}, w.pc, CodeForbidden)
}
func TestAPlayerCannotActOnAnotherLayer(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	w.refuse(&Ping{Layer: cellar, X: 10, Y: 10}, w.pc, CodeForbidden)
	w.refuse(&StrokeBegin{ID: testID(710), Layer: cellar, Kind: StrokeFree, Color: "#ffffff", Width: 2, Points: []int{0, 0}}, w.pc, CodeForbidden)
	w.refuse(&FogAdd{Layer: cellar, Kind: ShapeRect, Mode: FogHide, Points: []int{0, 0, 1, 1}}, w.pc, CodeForbidden)
	w.apply(&Ping{Layer: cellar, X: 10, Y: 10}, w.gm)
	w.apply(&StrokeBegin{ID: testID(711), Layer: cellar, Kind: StrokeFree, Color: "#ffffff", Width: 2, Points: []int{0, 0}}, w.gm)
}
func TestOnlyTheGMMovesPawnsBetweenLayers(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	mine := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true, OwnerID: &testPlayerID})
	w.refuse(&PawnSetLayer{IDs: []ulid.ULID{mine}, Layer: cellar}, w.pc, CodeForbidden)
	ch := w.change(&PawnSetLayer{IDs: []ulid.ULID{mine}, Layer: cellar}, w.gm)
	equalStrings(t, "the GM", eventTypesOf(ch.events(RoleGM)), []string{"pawn.updated"})
	equalStrings(t, "the players", eventTypesOf(ch.events(RolePlayer)), []string{"pawn.removed"})
}
func TestTheTrackerKeepsCombatantsOnOtherFloors(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	here := w.spawn(Pawn{Name: "Ogre", Visible: true})
	fallen := w.spawn(Pawn{Name: "Goblin", LayerID: cellar, Visible: true})
	hidden := w.spawn(Pawn{Name: "Ambusher", Visible: false})
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Ogre", PawnIDs: []ulid.ULID{here}},
		{Name: "Goblin", PawnIDs: []ulid.ULID{fallen}},
		{Name: "Ambusher", PawnIDs: []ulid.ULID{hidden}},
	}}, w.gm)
	entries := w.s.Project(RolePlayer).Initiative.Entries
	if len(entries) != 2 {
		t.Fatalf("the players' tracker holds %d entries, want the two they may know about", len(entries))
	}
	if entries[0].Name != "Ogre" || entries[1].Name != "Goblin" {
		t.Fatalf("the players' tracker holds %v", []string{entries[0].Name, entries[1].Name})
	}
}
func TestALayersMapIsResolvedBeforeItIsSet(t *testing.T) {
	w := newWorld(t)
	w.refuse(&TableSetLayerMap{Layer: w.layer, AssetID: testAssetID}, w.gm, CodeInvalid)
	w.apply(&TableSetLayerMap{
		Layer:   w.layer,
		AssetID: testAssetID,
		Map:     &MapRef{AssetID: testAssetID, Gen: testID(50), Width: 4096, Height: 4096, TileSize: 512, MaxZoom: 3},
	}, w.gm)
	if w.s.Layer(w.layer).Map == nil {
		t.Fatal("the resolved map was not stored")
	}
	w.refuse(&TableSetLayerMap{Layer: w.layer, AssetID: testAssetID, Map: &MapRef{}}, w.gm, CodeInvalid)
	w.apply(&TableClearLayerMap{Layer: w.layer}, w.gm)
	if w.s.Layer(w.layer).Map != nil {
		t.Fatal("the map was not cleared")
	}
}
func TestLayersReorderWithinTheList(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	w.apply(&TableMoveLayer{Layer: cellar, Index: 0}, w.gm)
	if w.s.Table.Layers[0].ID != cellar {
		t.Fatal("the layer did not move to the bottom")
	}
	w.refuse(&TableMoveLayer{Layer: cellar, Index: 2}, w.gm, CodeInvalid)
	w.refuse(&TableMoveLayer{Layer: cellar, Index: -1}, w.gm, CodeInvalid)
}
