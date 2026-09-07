package room

import (
	"strings"
	"testing"
)

// A new room is the state a GM meets before they have done anything, so its
// defaults are a design decision rather than an initialisation. Each one here
// is pinned because changing it silently changes what a first session looks
// like.
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

	// Fog off but prefilled: switching fog on should cover the map, because
	// that is what a GM reaching for the switch means by it.
	if s.Table.Layers[0].FogEnabled {
		t.Fatal("a new room starts with fog switched on")
	}
	if !s.Table.Layers[0].FogPrefill {
		t.Fatal("a new room's fog is not prefilled, so turning it on would reveal everything")
	}

	if s.Table.MonsterHP != HPBandOn {
		t.Fatalf("monster hit points default to %q, want %q", s.Table.MonsterHP, HPBandOn)
	}
	if !s.Table.PlayersCanDraw {
		t.Fatal("players cannot draw in a new room")
	}

	g := s.Table.Grid
	if !g.Visible || g.CellSize != DefaultCellSize || g.Color != DefaultGridColor ||
		g.Snap != SnapCells || g.FeetPerCell != DefaultFeetPerCell || g.Diagonals != DiagonalsEqual {
		t.Fatalf("the default grid is %+v", g)
	}
	if err := checkGrid(g); err != nil {
		t.Fatalf("the default grid does not pass its own validation: %v", err)
	}
}

// Normalize is what makes "the same state" a byte comparison, so the three
// sorted collections have to actually sort.
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

// Fog and initiative are the two collections where the order is the meaning: a
// hide drawn over a reveal covers it, and the turn order is the order the GM
// dragged the lines into. Sorting either would be a bug that looked like
// tidiness.
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

// A nil slice marshals as null and the generated TypeScript declares these
// fields as arrays. Every collection has to come out as [] so that the client's
// reducer never meets one it has to null-check.
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

// The footprint is the one answer to "how many cells does this stand on", and
// every part of the app that needs it asks here rather than branching on kind.
func TestFootprintCoversCreaturesAndObjects(t *testing.T) {
	sizes := map[Size]int{
		SizeTiny: 1, SizeSmall: 1, SizeMedium: 1,
		SizeLarge: 2, SizeHuge: 3, SizeGargantuan: 4,
	}
	for size, want := range sizes {
		w, h := Pawn{Kind: PawnMonster, Size: size}.Footprint()
		if w != want || h != want {
			t.Fatalf("a %s creature stands on %dx%d cells, want %dx%d", size, w, h, want, want)
		}
	}

	w, h := Pawn{Kind: PawnObject, FootprintW: 2, FootprintH: 4}.Footprint()
	if w != 2 || h != 4 {
		t.Fatalf("a 2x4 object reports %dx%d", w, h)
	}

	// An object that somehow reached the state with a zero footprint reports 1
	// rather than 0, because zero would divide by nothing in the snapper.
	w, h = Pawn{Kind: PawnObject}.Footprint()
	if w != 1 || h != 1 {
		t.Fatalf("a zero footprint reports %dx%d, want 1x1", w, h)
	}
}

// Every enum's Valid is written in terms of its own Values, so this walks the
// whole set at once: each declared value passes and an invented one does not.
func TestEnumsAgreeWithTheirOwnValues(t *testing.T) {
	type enum interface {
		Values() []string
	}

	enums := map[string]enum{
		"Role": RoleGM, "Snap": SnapCells, "Diagonals": DiagonalsEqual,
		"HPVisibility": HPBandOn, "PawnKind": PawnMonster, "Size": SizeMedium,
		"HPBand": BandHealthy, "ConditionColor": ColorRed,
		"ClearTrigger": ClearStart, "ShapeKind": ShapeRect, "FogMode": FogReveal,
	}

	valid := map[string]func(string) bool{
		"Role":         func(v string) bool { return Role(v).Valid() },
		"Snap":         func(v string) bool { return Snap(v).Valid() },
		"Diagonals":    func(v string) bool { return Diagonals(v).Valid() },
		"HPVisibility": func(v string) bool { return HPVisibility(v).Valid() },
		"PawnKind":     func(v string) bool { return PawnKind(v).Valid() },
		"Size":         func(v string) bool { return Size(v).Valid() },
		"HPBand":       func(v string) bool { return HPBand(v).Valid() },
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
