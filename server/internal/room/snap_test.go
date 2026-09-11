package room

import "testing"






func TestSnapAxisTakesEveryParityAndMode(t *testing.T) {
	const cell = 64

	
	
	tests := []struct {
		name      string
		footprint int
		mode      Snap
		in        int
		want      int
	}{
		{"odd footprint under cells lands on a centre", 1, SnapCells, 100, 96},
		{"even footprint under cells lands on a vertex", 2, SnapCells, 100, 128},

		
		
		
		{"odd footprint under half-cells takes whichever is nearer", 1, SnapHalfCells, 100, 96},
		{"even footprint under half-cells takes the same one", 2, SnapHalfCells, 100, 96},

		
		
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






func TestSnapAxisBreaksTiesAwayFromZero(t *testing.T) {
	if got := SnapAxis(64, 0, 1, SnapCells, 64); got != 96 {
		t.Fatalf("the midpoint above the origin snapped to %d, want 96", got)
	}
	if got := SnapAxis(64, 0, 1, SnapCells, 0); got != -32 {
		t.Fatalf("the midpoint below the origin snapped to %d, want -32", got)
	}
}






func TestSnapHalfCellsReachesEveryCentreAndEveryVertex(t *testing.T) {
	const cell = 64

	var seen []int
	for v := 0; v <= 128; v++ {
		got := SnapAxis(cell, 0, 1, SnapHalfCells, v)
		if len(seen) == 0 || seen[len(seen)-1] != got {
			seen = append(seen, got)
		}
	}

	
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




func TestSnapAxisTakesANegativeOffset(t *testing.T) {
	
	if got := SnapAxis(64, -16, 1, SnapCells, 0); got != 16 {
		t.Fatalf("SnapAxis with a negative offset = %d, want 16", got)
	}
	if got := SnapAxis(64, -16, 2, SnapCells, 0); got != -16 {
		t.Fatalf("SnapAxis with a negative offset on an even footprint = %d, want -16", got)
	}
}




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


func TestSnapPawnReadsTheFootprintOffThePawn(t *testing.T) {
	g := Grid{CellSize: 64, Snap: SnapCells}

	x, y := snapPawn(g, Pawn{Kind: PawnMonster, Size: SizeLarge}, 100, 100)
	if x != 128 || y != 128 {
		t.Fatalf("a large creature snapped to (%d, %d), want the vertex at (128, 128)", x, y)
	}

	
	
	
	x, y = snapPawn(g, Pawn{Kind: PawnObject, Width: 2 * g.CellSize, Height: 3 * g.CellSize}, 100, 100)
	if x != 100 || y != 100 {
		t.Fatalf("a 2x3 object snapped to (%d, %d), want the raw (100, 100)", x, y)
	}
}




func TestSnapAxisSurvivesAnImpossibleGrid(t *testing.T) {
	if got := SnapAxis(0, 0, 1, SnapCells, 100); got != 100 {
		t.Fatalf("a zero cell size returned %d, want the input unchanged", got)
	}
}





func TestNormalizeRepairsASnappingModeThatNoLongerExists(t *testing.T) {
	s := NewState(testRoomID, "The Sunless Citadel", Env{})
	s.Table.Grid.Snap = Snap("corners")
	s.Normalize()

	if s.Table.Grid.Snap != SnapCells {
		t.Fatalf("a retired snapping mode came back as %q, want %q", s.Table.Grid.Snap, SnapCells)
	}

	
	s.Table.Grid.Snap = SnapHalfCells
	s.Normalize()
	if s.Table.Grid.Snap != SnapHalfCells {
		t.Fatalf("a live snapping mode was rewritten to %q", s.Table.Grid.Snap)
	}
}
