package room

import "testing"

// EVERY PARITY AGAINST EVERY MODE, which is the whole of the snapping rule.
// Under cells the parity picks between a centre and a vertex; under half-cells
// the parity does not come into it, because both are on the lattice and the
// nearer one wins whatever the creature's size. If this table is right, a wagon
// is right.
func TestSnapAxisTakesEveryParityAndMode(t *testing.T) {
	const cell = 64

	// With a zero offset the cell centres are at 32, 96, 160 and the vertices
	// at 0, 64, 128. A raw 100 is nearer 96 than 160, and nearer 128 than 64.
	tests := []struct {
		name      string
		footprint int
		mode      Snap
		in        int
		want      int
	}{
		{"odd footprint under cells lands on a centre", 1, SnapCells, 100, 96},
		{"even footprint under cells lands on a vertex", 2, SnapCells, 100, 128},

		// 100 is 4 from the centre at 96 and 28 from the vertex at 128, so
		// half-cells takes the centre for both parities -- the point of the
		// mode being that it is the nearest legal position and nothing else.
		{"odd footprint under half-cells takes whichever is nearer", 1, SnapHalfCells, 100, 96},
		{"even footprint under half-cells takes the same one", 2, SnapHalfCells, 100, 96},

		// And 120 is nearer the vertex at 128 than the centre at 96, so the
		// same mode answers a vertex without being told to.
		{"half-cells reaches a vertex when the vertex is nearer", 1, SnapHalfCells, 120, 128},
		{"half-cells reaches a vertex for an even footprint too", 2, SnapHalfCells, 120, 128},

		{"a gargantuan creature is even and straddles", 4, SnapCells, 100, 128},
		{"a huge creature is odd and centres", 3, SnapCells, 100, 96},

		{"off leaves the coordinate alone", 1, SnapOff, 100, 100},
		{"off leaves an even footprint alone too", 2, SnapOff, 100, 100},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SnapAxis(cell, 0, tc.footprint, tc.mode, tc.in); got != tc.want {
				t.Fatalf("SnapAxis(%d, 0, %d, %q, %d) = %d, want %d", cell, tc.footprint, tc.mode, tc.in, got, tc.want)
			}
		})
	}
}

// A point exactly between two legal positions has to go somewhere, and the
// somewhere has to be the same on every machine. math.Round is half away from
// zero, which is what these two cases pin: 64 is the midpoint between the
// centres at 32 and 96 and goes up, and 0 is the midpoint between -32 and 32
// and goes down. Neither drifts toward the origin.
func TestSnapAxisBreaksTiesAwayFromZero(t *testing.T) {
	if got := SnapAxis(64, 0, 1, SnapCells, 64); got != 96 {
		t.Fatalf("the midpoint above the origin snapped to %d, want 96", got)
	}
	if got := SnapAxis(64, 0, 1, SnapCells, 0); got != -32 {
		t.Fatalf("the midpoint below the origin snapped to %d, want -32", got)
	}
}

// THE HALF-CELL LATTICE IS EXACTLY THE CENTRES AND THE VERTICES, alternating,
// and this is the property the TypeScript port has to reproduce: sweeping a
// coordinate across two cells must produce every one of them and nothing in
// between. A mode that stepped by a whole cell from the vertex would reach half
// of these and a GM would report that corners "sometimes" work.
func TestSnapHalfCellsReachesEveryCentreAndEveryVertex(t *testing.T) {
	const cell = 64

	var seen []int
	for v := 0; v <= 128; v++ {
		got := SnapAxis(cell, 0, 1, SnapHalfCells, v)
		if len(seen) == 0 || seen[len(seen)-1] != got {
			seen = append(seen, got)
		}
	}

	// 0 and 128 are vertices, 32 and 96 centres, 64 the vertex between them.
	want := []int{0, 32, 64, 96, 128}
	if len(seen) != len(want) {
		t.Fatalf("the sweep landed on %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("the sweep landed on %v, want %v", seen, want)
		}
	}
}

// A negative offset is an ordinary offset: a GM who nudged the grid left by a
// quarter cell should get a grid that is a quarter cell left, not one that
// stopped at zero.
func TestSnapAxisTakesANegativeOffset(t *testing.T) {
	// Offset -16 puts the centres at 16, 80, 144 and the vertices at -16, 48.
	if got := SnapAxis(64, -16, 1, SnapCells, 0); got != 16 {
		t.Fatalf("SnapAxis with a negative offset = %d, want 16", got)
	}
	if got := SnapAxis(64, -16, 2, SnapCells, 0); got != -16 {
		t.Fatalf("SnapAxis with a negative offset on an even footprint = %d, want -16", got)
	}
}

// The case objects exist for. A two by three wagon is even across and odd down,
// so it straddles horizontally and centres vertically -- one call, two answers,
// which is why snapping is written per axis rather than per pawn.
func TestSnapPointTakesEachAxisSeparately(t *testing.T) {
	g := Grid{CellSize: 64, Snap: SnapCells}

	x, y := SnapPoint(g, 2, 3, 100, 100)
	if x != 128 {
		t.Fatalf("the even axis snapped to %d, want the vertex at 128", x)
	}
	if y != 96 {
		t.Fatalf("the odd axis snapped to %d, want the centre at 96", y)
	}
}

// A pawn is never asked which rule applies to it; it is asked how big it is.
func TestSnapPawnReadsTheFootprintOffThePawn(t *testing.T) {
	g := Grid{CellSize: 64, Snap: SnapCells}

	x, y := snapPawn(g, Pawn{Kind: PawnMonster, Size: SizeLarge}, 100, 100)
	if x != 128 || y != 128 {
		t.Fatalf("a large creature snapped to (%d, %d), want the vertex at (128, 128)", x, y)
	}

	// AND AN OBJECT IS NOT ASKED AT ALL. It is a picture laid on the floor
	// rather than a creature standing in a square, so it keeps the pixel it was
	// given even with whole-cell snapping switched on.
	x, y = snapPawn(g, Pawn{Kind: PawnObject, Width: 2 * g.CellSize, Height: 3 * g.CellSize}, 100, 100)
	if x != 100 || y != 100 {
		t.Fatalf("a 2x3 object snapped to (%d, %d), want the raw (100, 100)", x, y)
	}
}

// A cell size of zero would divide by nothing. It cannot arrive through
// TableSetGrid, which validates, but SnapAxis is exported and the renderer's
// port of it will be called from wherever the client feels like calling it.
func TestSnapAxisSurvivesAnImpossibleGrid(t *testing.T) {
	if got := SnapAxis(0, 0, 1, SnapCells, 100); got != 100 {
		t.Fatalf("a zero cell size returned %d, want the input unchanged", got)
	}
}

// A room saved when "corners" was a mode comes back with a value that is no
// longer one. Nothing else about that room is wrong, so it is repaired rather
// than discarded -- the alternative is a grid form with no radio selected and a
// refusal on the next unrelated change.
func TestNormalizeRepairsASnappingModeThatNoLongerExists(t *testing.T) {
	s := NewState(testRoomID, "The Sunless Citadel", Env{})
	s.Table.Grid.Snap = Snap("corners")
	s.Normalize()

	if s.Table.Grid.Snap != SnapCells {
		t.Fatalf("a retired snapping mode came back as %q, want %q", s.Table.Grid.Snap, SnapCells)
	}

	// And a mode that IS still one is left exactly as the GM set it.
	s.Table.Grid.Snap = SnapHalfCells
	s.Normalize()
	if s.Table.Grid.Snap != SnapHalfCells {
		t.Fatalf("a live snapping mode was rewritten to %q", s.Table.Grid.Snap)
	}
}
