package room

import (
	"testing"

	"github.com/oklog/ulid/v2"
)

// CHANGING THE ACTIVE LAYER IS A CROSSFADE FOR THE GM AND A WHOLE NEW TABLE FOR
// THE PLAYERS. The GM sees one table.updated, because to them nothing appeared
// or disappeared; the players lose every pawn on the old floor and gain every
// pawn on the new one, through the same two events visibility uses.
func TestChangingTheActiveLayerRebuildsThePlayersTable(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")

	upstairs := w.spawn(Pawn{Name: "Upstairs", Visible: true})
	downstairs := w.spawn(Pawn{Name: "Downstairs", LayerID: cellar, Visible: true})
	w.spawn(Pawn{Name: "Hidden downstairs", LayerID: cellar, Visible: false})

	ems := w.apply(&TableSetActiveLayer{Layer: cellar}, w.gm)

	// The table first: a client that received the pawns first would spawn them
	// onto a floor it still believed was the old one.
	equalStrings(t, "emissions", summary(ems), []string{
		"table.updated to all",
		"pawn.removed to players",
		"pawn.spawned to players",
	})

	if got := ems[1].Event.(*PawnRemoved).ID; got != upstairs {
		t.Fatal("the wrong pawn was taken off the players' table")
	}
	if got := ems[2].Event.(*PawnSpawned).Pawn.ID; got != downstairs {
		t.Fatal("the wrong pawn was put on the players' table")
	}

	if got := eventTypesOf(delivered(ems, w.gm, w.gm)); len(got) != 1 || got[0] != "table.updated" {
		t.Fatalf("the GM received %v, want only the table", got)
	}
}

// REMOVING A LAYER DELETES WHAT IS STANDING ON IT, in the order the clients
// need to hear it, and moves the active layer when the GM removed the floor
// they were on.
func TestRemovingALayerEmptiesItInOrder(t *testing.T) {
	w := newWorld(t)
	ground := w.layer
	cellar := w.addLayer("Cellar")

	// The party is downstairs, so the cellar is what everybody is looking at.
	w.apply(&TableSetActiveLayer{Layer: cellar}, w.gm)
	w.spawn(Pawn{Name: "Upstairs", LayerID: ground, Visible: true})

	goblin := w.spawn(Pawn{Name: "Goblin", LayerID: cellar, Visible: true})
	w.spawn(Pawn{Name: "Ambusher", LayerID: cellar, Visible: false})
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}}}}, w.gm)
	w.apply(&FogAdd{Layer: cellar, Kind: ShapeRect, Mode: FogHide, Points: []int{0, 0, 64, 64}}, w.gm)
	w.apply(&StrokeBegin{ID: testID(700), Layer: cellar, Color: "#ffffff", Width: 2, Points: []int{0, 0}}, w.gm)

	ems := w.apply(&TableRemoveLayer{Layer: cellar}, w.gm)

	equalStrings(t, "emissions", summary(ems), []string{
		// The visible goblin goes from both tables; the hidden ambusher was
		// only ever on the GM's.
		"pawn.removed to gm",
		"pawn.removed to players",
		"pawn.removed to gm",

		"fog.cleared to all",
		"stroke.cleared to all",
		"initiative.updated to all",
		"table.updated to all",

		// And the floor everybody landed on brings its own pawns with it.
		"pawn.spawned to players",
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

// Removing a layer nobody is standing on leaves the active layer alone, so
// there is nothing to spawn afterwards.
func TestRemovingAnInactiveLayerDoesNotMoveAnybody(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	w.spawn(Pawn{Name: "Upstairs", Visible: true})

	ems := w.apply(&TableRemoveLayer{Layer: cellar}, w.gm)
	equalStrings(t, "emissions", summary(ems), []string{
		"fog.cleared to all",
		"stroke.cleared to all",
		"table.updated to all",
	})

	if w.s.Table.ActiveLayer != w.layer {
		t.Fatal("removing another floor moved the active layer")
	}
}

// A room needs somewhere to put things.
func TestTheLastLayerCannotBeRemoved(t *testing.T) {
	w := newWorld(t)

	w.refuse(&TableRemoveLayer{Layer: w.layer}, w.gm, CodeInvalid)
	w.refuse(&TableRemoveLayer{Layer: testID(999)}, w.gm, CodeNotFound)
}

// CLEAR TABLETOP IS THE END OF THE EVENING: every floor's map, every pawn, all
// the fog, all the drawing and the tracker, in one command. What it does NOT
// touch is the floors themselves and the grid -- a room needs at least one
// layer, and the cell size a GM matched to their maps is a setting rather than
// a thing standing on the table.
func TestClearingTheTabletopEmptiesEveryFloor(t *testing.T) {
	w := newWorld(t)
	ground := w.layer
	cellar := w.addLayer("Cellar")

	goblin := w.spawn(Pawn{Name: "Goblin", LayerID: ground, Visible: true})
	w.spawn(Pawn{Name: "Ambusher", LayerID: ground, Visible: false})
	w.spawn(Pawn{Name: "Downstairs", LayerID: cellar, Visible: true})

	w.apply(&InitiativeSet{Entries: []InitiativeEntry{{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}}}}, w.gm)
	w.apply(&FogAdd{Layer: ground, Kind: ShapeRect, Mode: FogHide, Points: []int{0, 0, 64, 64}}, w.gm)
	w.apply(&StrokeBegin{ID: testID(701), Layer: cellar, Color: "#ffffff", Width: 2, Points: []int{0, 0}}, w.gm)

	cell := w.s.Table.Grid.CellSize
	ems := w.apply(&TableClear{}, w.gm)

	equalStrings(t, "emissions", summary(ems), []string{
		// Three to the GM and one to the players. Only the goblin was ever on
		// a player's table: the ambusher is hidden, and the third is on a
		// floor nobody is looking at.
		"pawn.removed to gm",
		"pawn.removed to players",
		"pawn.removed to gm",
		"pawn.removed to gm",

		// Both floors, whether or not either had anything on it.
		"fog.cleared to all",
		"stroke.cleared to all",
		"fog.cleared to all",
		"stroke.cleared to all",

		"table.updated to all",
		"initiative.updated to all",
	})

	if len(w.s.Pawns) != 0 || len(w.s.Fog) != 0 || len(w.s.Strokes) != 0 {
		t.Fatalf("the table still holds %d pawns, %d fog shapes and %d strokes",
			len(w.s.Pawns), len(w.s.Fog), len(w.s.Strokes))
	}
	if len(w.s.Initiative.Entries) != 0 {
		t.Fatal("the tracker outlived the pawns it named")
	}

	// The floors stay, and so does everything about them that is a setting
	// rather than a thing on the table.
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

// A player cannot clear the table, which is the same rule every other command
// in this file has and matters more here than in any of them.
func TestOnlyTheGMClearsTheTabletop(t *testing.T) {
	w := newWorld(t)

	w.refuse(&TableClear{}, w.pc, CodeForbidden)
}

// A PLAYER'S COMMANDS STAY ON THE FLOOR EVERYBODY IS LOOKING AT. A player who
// could name another layer could ping into a room the party has not found, or
// draw on a map they cannot see, and in both cases the giveaway is that the
// GM's screen changes.
func TestAPlayerCannotActOnAnotherLayer(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")

	w.refuse(&Ping{Layer: cellar, X: 10, Y: 10}, w.pc, CodeForbidden)
	w.refuse(&StrokeBegin{ID: testID(710), Layer: cellar, Color: "#ffffff", Width: 2, Points: []int{0, 0}}, w.pc, CodeForbidden)

	// fog.add is GM-only anyway, and the layer rule is checked first, so a
	// player naming another floor is refused for the more specific reason.
	w.refuse(&FogAdd{Layer: cellar, Kind: ShapeRect, Mode: FogHide, Points: []int{0, 0, 1, 1}}, w.pc, CodeForbidden)

	// The GM is the one person entitled to work on a floor nobody is watching.
	w.apply(&Ping{Layer: cellar, X: 10, Y: 10}, w.gm)
	w.apply(&StrokeBegin{ID: testID(711), Layer: cellar, Color: "#ffffff", Width: 2, Points: []int{0, 0}}, w.gm)
}

// Moving pawns between floors is GM-only, because a player who moved their own
// pawn downstairs would be holding a pawn they can no longer see.
func TestOnlyTheGMMovesPawnsBetweenLayers(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")

	mine := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true, OwnerID: &testPlayerID})
	w.refuse(&PawnSetLayer{IDs: []ulid.ULID{mine}, Layer: cellar}, w.pc, CodeForbidden)

	ems := w.apply(&PawnSetLayer{IDs: []ulid.ULID{mine}, Layer: cellar}, w.gm)
	equalStrings(t, "emissions", summary(ems), []string{
		"pawn.updated to gm",
		"pawn.removed to players",
	})
}

// THE TRACKER KEEPS A LINE FOR A PAWN ON ANOTHER FLOOR. A creature that stepped
// through a trapdoor still has a turn, and the players know it exists; the
// client draws that line by name because it has no pawn to draw it from.
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

// The grid is room-wide, so a layer carries no grid of its own and a map that
// disagrees with it misaligns rather than erroring.
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

	// A map that has not finished tiling has no usable dimensions, and setting
	// it would give the renderer a zero-by-zero image to fetch tiles from.
	w.refuse(&TableSetLayerMap{Layer: w.layer, AssetID: testAssetID, Map: &MapRef{}}, w.gm, CodeInvalid)

	w.apply(&TableClearLayerMap{Layer: w.layer}, w.gm)
	if w.s.Layer(w.layer).Map != nil {
		t.Fatal("the map was not cleared")
	}
}

// Reordering is a position in the list, and a position outside it is a client
// bug rather than a floor that moved somewhere odd.
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
