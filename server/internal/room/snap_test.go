package room

import "testing"

// THE FOUR COMBINATIONS, which are the whole of the snapping rule. A footprint
// is odd or even and the mode is cells or corners, and every case in the app is
// one of those four read per axis. If this table is right, a wagon is right.
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
		{"odd footprint under corners lands on a vertex", 1, SnapCorners, 100, 128},
		{"even footprint under corners lands on a centre", 2, SnapCorners, 100, 96},

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

	x, y = snapPawn(g, Pawn{Kind: PawnObject, FootprintW: 2, FootprintH: 3}, 100, 100)
	if x != 128 || y != 96 {
		t.Fatalf("a 2x3 object snapped to (%d, %d), want (128, 96)", x, y)
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
