package room

import (
	"strings"
	"testing"
)

func TestNewStateStartsUsable(t *testing.T) {
	s := NewState(testRoomID, "The Sunless Citadel", newEnv())
	if s.Schema != Schema {
		t.Fatalf("schema = %d, want %d", s.Schema, Schema)
	}
	if s.Room.Name != "The Sunless Citadel" {
		t.Fatalf("room name = %q", s.Room.Name)
	}
	if len(s.Table.Layers) != 1 {
		t.Fatalf("a new room has %d layers, want exactly 1", len(s.Table.Layers))
	}
	if s.Table.Layers[0].Name != DefaultLayerName {
		t.Fatalf("the first layer is called %q, want %q", s.Table.Layers[0].Name, DefaultLayerName)
	}
	if s.Table.ActiveLayer != s.Table.Layers[0].ID {
		t.Fatal("the one layer a new room has is not the active one")
	}
	if s.Table.Layers[0].Map != nil {
		t.Fatal("a new room's layer already has a map")
	}
	if s.Table.Layers[0].FogEnabled {
		t.Fatal("a new room starts with fog switched on")
	}
	if !s.Table.Layers[0].FogPrefill {
		t.Fatal("a new room's fog is not prefilled, so turning it on would reveal everything")
	}
	if s.Table.PawnLabels != LabelsDefault {
		t.Fatalf("monster hit points default to %q, want %q", s.Table.PawnLabels, LabelsDefault)
	}
	if !s.Table.PlayersCanDraw {
		t.Fatal("players cannot draw in a new room")
	}
	g := s.Table.Grid
	if g.Lines != GridLinesSolid || g.CellSize != DefaultCellSize || g.Color != DefaultGridColor ||
		g.Snap != SnapCells || g.FeetPerCell != DefaultFeetPerCell || g.Diagonals != DiagonalsEqual {
		t.Fatalf("the default grid is %+v", g)
	}
	if err := checkGrid(g); err != nil {
		t.Fatalf("the default grid does not pass its own validation: %v", err)
	}
}
func TestNormalizeSortsWhatHasNoOrderOfItsOwn(t *testing.T) {
	s := &State{
		Pawns:   []Pawn{{ID: testID(3)}, {ID: testID(1)}, {ID: testID(2)}},
		Players: []Player{{ID: testID(9)}, {ID: testID(4)}},
		Strokes: []Stroke{{ID: testID(7)}, {ID: testID(5)}},
	}
	s.Normalize()
	if s.Pawns[0].ID != testID(1) || s.Pawns[2].ID != testID(3) {
		t.Fatal("pawns did not sort by id")
	}
	if s.Players[0].ID != testID(4) {
		t.Fatal("players did not sort by id")
	}
	if s.Strokes[0].ID != testID(5) {
		t.Fatal("strokes did not sort by id")
	}
}
func TestNormalizeLeavesTheOrderedCollectionsAlone(t *testing.T) {
	s := &State{
		Fog: []FogShape{{ID: testID(9)}, {ID: testID(2)}},
		Initiative: Initiative{
			Entries: []InitiativeEntry{{ID: testID(8)}, {ID: testID(1)}},
		},
	}
	s.Normalize()
	if s.Fog[0].ID != testID(9) {
		t.Fatal("fog was sorted; a hide drawn over a reveal would now be under it")
	}
	if s.Initiative.Entries[0].ID != testID(8) {
		t.Fatal("the turn order was sorted")
	}
}
func TestNewStateMarshalsWithNoNullCollections(t *testing.T) {
	s := NewState(testRoomID, "Room", newEnv())
	s.Pawns = append(s.Pawns, Pawn{ID: testID(1), Kind: PawnMonster, Size: SizeMedium})
	s.Fog = append(s.Fog, FogShape{ID: testID(2)})
	s.Strokes = append(s.Strokes, Stroke{ID: testID(3)})
	s.Normalize()
	b, err := Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, field := range []string{
		`"players":null`, `"pawns":null`, `"fog":null`, `"strokes":null`,
		`"layers":null`, `"entries":null`, `"conditions":null`, `"points":null`,
	} {
		if strings.Contains(string(b), field) {
			t.Fatalf("the snapshot contains %s; the client would have to null-check that collection", field)
		}
	}
}
func TestFootprintIsTheSizeCategory(t *testing.T) {
	sizes := map[Size]int{
		SizeTiny: 1, SizeSmall: 1, SizeMedium: 1,
		SizeLarge: 2, SizeHuge: 3, SizeGargantuan: 4,
	}
	for size, want := range sizes {
		if got := size.Footprint(); got != want {
			t.Fatalf("a %s creature stands on %d cells, want %d", size, got, want)
		}
	}
	if got := Size("enormous").Footprint(); got != 1 {
		t.Fatalf("an unknown size stands on %d cells, want 1", got)
	}
}
func TestRotationIsFoldedIntoOneTurn(t *testing.T) {
	for in, want := range map[int]int{
		0: 0, 45: 45, 359: 359, 360: 0, 361: 1,
		-1: 359, -90: 270, -360: 0, -361: 359, 725: 5,
	} {
		if got := normalizeRotation(in); got != want {
			t.Fatalf("%d degrees folds to %d, want %d", in, got, want)
		}
	}
}
func TestEnumsAgreeWithTheirOwnValues(t *testing.T) {
	type enum interface {
		Values() []string
	}
	enums := map[string]enum{
		"Role": RoleGM, "Snap": SnapCells, "Diagonals": DiagonalsEqual,
		"PawnLabels": LabelsDefault, "PawnKind": PawnMonster, "Size": SizeMedium,
		"HPBand": BandHealthy, "ConditionColor": ColorRed,
		"ClearTrigger": ClearStart, "ShapeKind": ShapeRect, "FogMode": FogReveal,
	}
	valid := map[string]func(string) bool{
		"Role":       func(v string) bool { return Role(v).Valid() },
		"Snap":       func(v string) bool { return Snap(v).Valid() },
		"Diagonals":  func(v string) bool { return Diagonals(v).Valid() },
		"PawnLabels": func(v string) bool { return PawnLabels(v).Valid() },
		"PawnKind":   func(v string) bool { return PawnKind(v).Valid() },
		"Size":       func(v string) bool { return Size(v).Valid() },
		"HPBand":     func(v string) bool { return HPBand(v).Valid() },
		"ConditionColor": func(v string) bool {
			return ConditionColor(v).Valid()
		},
		"ClearTrigger": func(v string) bool { return ClearTrigger(v).Valid() },
		"ShapeKind":    func(v string) bool { return ShapeKind(v).Valid() },
		"FogMode":      func(v string) bool { return FogMode(v).Valid() },
	}
	for name, e := range enums {
		values := e.Values()
		if len(values) == 0 {
			t.Fatalf("%s has no values", name)
		}
		for _, v := range values {
			if !valid[name](v) {
				t.Fatalf("%s.Valid refuses %q, which is one of its own values", name, v)
			}
		}
		if valid[name]("nonsense-" + name) {
			t.Fatalf("%s.Valid accepts a value that is not in its list", name)
		}
	}
}
