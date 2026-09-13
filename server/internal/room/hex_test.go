package room

import "testing"

func hexGrid(t GridType) Grid {
	return Grid{Type: t, CellSize: 64, Snap: SnapCells, FeetPerCell: 5, Units: UnitsFeet}
}
func TestHexCentreSpacesTheHoneycombByTheAcrossFlatsSize(t *testing.T) {
	tests := []struct {
		kind GridType
		q, r int
		x, y int
	}{
		{GridHexPointy, 0, 0, 32, 32},
		{GridHexPointy, 1, 0, 96, 32},
		{GridHexPointy, 0, 1, 64, 87},
		{GridHexPointy, -1, 2, 32, 143},
		{GridHexPointy, 2, -1, 128, -23},
		{GridHexPointy, 3, 3, 320, 198},
		{GridHexFlat, 0, 0, 32, 32},
		{GridHexFlat, 1, 0, 87, 64},
		{GridHexFlat, 0, 1, 32, 96},
		{GridHexFlat, -1, 2, -23, 128},
		{GridHexFlat, 2, -1, 143, 32},
		{GridHexFlat, 3, 3, 198, 320},
	}
	for _, tc := range tests {
		x, y := hexCentre(hexGrid(tc.kind), tc.q, tc.r)
		if x != tc.x || y != tc.y {
			t.Errorf("hexCentre(%s, %d, %d) = (%d, %d), want (%d, %d)", tc.kind, tc.q, tc.r, x, y, tc.x, tc.y)
		}
	}
}
func TestHexAtFindsTheHexAPointIsInside(t *testing.T) {
	tests := []struct {
		kind GridType
		x, y int
		q, r int
	}{
		{GridHexPointy, 32, 32, 0, 0},
		{GridHexPointy, 100, 40, 1, 0},
		{GridHexPointy, 64, 87, 0, 1},
		{GridHexPointy, 0, 0, 0, -1},
		{GridHexPointy, 200, 200, 1, 3},
		{GridHexPointy, -40, -40, -1, -1},
		{GridHexFlat, 32, 32, 0, 0},
		{GridHexFlat, 100, 70, 1, 0},
		{GridHexFlat, 32, 96, 0, 1},
		{GridHexFlat, 0, 0, -1, 0},
		{GridHexFlat, 200, 200, 3, 1},
		{GridHexFlat, -40, -40, -1, -1},
	}
	for _, tc := range tests {
		q, r := hexAt(hexGrid(tc.kind), tc.x, tc.y)
		if q != tc.q || r != tc.r {
			t.Errorf("hexAt(%s, %d, %d) = (%d, %d), want (%d, %d)", tc.kind, tc.x, tc.y, q, r, tc.q, tc.r)
		}
	}
}
func TestEveryHexCentreLandsBackInItsOwnHex(t *testing.T) {
	for _, g := range []Grid{
		hexGrid(GridHexPointy),
		hexGrid(GridHexFlat),
		{Type: GridHexPointy, CellSize: 97, OffsetX: 17, OffsetY: -9},
		{Type: GridHexFlat, CellSize: 97, OffsetX: 17, OffsetY: -9},
	} {
		for q := -8; q <= 8; q++ {
			for r := -8; r <= 8; r++ {
				x, y := hexCentre(g, q, r)
				backQ, backR := hexAt(g, x, y)
				if backQ != q || backR != r {
					t.Fatalf("%s cell (%d, %d) centres at (%d, %d), which reads back as (%d, %d)", g.Type, q, r, x, y, backQ, backR)
				}
			}
		}
	}
}
func TestAPointOnASharedEdgeResolvesTheSameWayEveryTime(t *testing.T) {
	g := hexGrid(GridHexPointy)
	q, r := hexAt(g, 64, 32)
	for i := 0; i < 8; i++ {
		againQ, againR := hexAt(g, 64, 32)
		if againQ != q || againR != r {
			t.Fatalf("the shared edge resolved to (%d, %d) and then (%d, %d)", q, r, againQ, againR)
		}
	}
	if q != 1 || r != 0 {
		t.Fatalf("the shared edge resolved to (%d, %d), want the hex to its right at (1, 0)", q, r)
	}
}
func TestHexDistanceCountsSteps(t *testing.T) {
	tests := []struct {
		q0, r0, q1, r1 int
		want           int
	}{
		{0, 0, 0, 0, 0},
		{0, 0, 3, 0, 3},
		{0, 0, 0, 3, 3},
		{0, 0, -2, 5, 5},
		{0, 0, 3, -3, 3},
		{2, -1, -1, 3, 4},
	}
	for _, tc := range tests {
		if got := hexDistance(tc.q0, tc.r0, tc.q1, tc.r1); got != tc.want {
			t.Errorf("hexDistance(%d, %d, %d, %d) = %d, want %d", tc.q0, tc.r0, tc.q1, tc.r1, got, tc.want)
		}
	}
}
func TestHexLineWalksEveryCellBetweenTheEnds(t *testing.T) {
	tests := []struct {
		q0, r0, q1, r1 int
		want           []int
	}{
		{0, 0, 0, 0, []int{0, 0}},
		{0, 0, 3, -1, []int{0, 0, 1, 0, 2, -1, 3, -1}},
		{0, 0, 0, 3, []int{0, 0, 0, 1, 0, 2, 0, 3}},
		{0, 0, -2, 5, []int{0, 0, 0, 1, -1, 2, -1, 3, -2, 4, -2, 5}},
	}
	for _, tc := range tests {
		got := hexLine(tc.q0, tc.r0, tc.q1, tc.r1, nil)
		if len(got) != len(tc.want) {
			t.Fatalf("hexLine(%d, %d, %d, %d) = %v, want %v", tc.q0, tc.r0, tc.q1, tc.r1, got, tc.want)
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Fatalf("hexLine(%d, %d, %d, %d) = %v, want %v", tc.q0, tc.r0, tc.q1, tc.r1, got, tc.want)
			}
		}
	}
}
func TestHexLineStopsAtThePathCap(t *testing.T) {
	got := hexLine(0, 0, 5000, 0, nil)
	if len(got) != PathCellsMax*2 {
		t.Fatalf("a path across 5000 hexes returned %d cells, want the cap at %d", len(got)/2, PathCellsMax)
	}
}
func TestHexCornersRingTheCentre(t *testing.T) {
	pointy := hexCorners(hexGrid(GridHexPointy), 0, 0, nil)
	want := []int{64, 14, 64, 50, 32, 69, 0, 50, 0, 14, 32, -5}
	for i := range want {
		if pointy[i] != want[i] {
			t.Fatalf("pointy corners = %v, want %v", pointy, want)
		}
	}
	flat := hexCorners(hexGrid(GridHexFlat), 0, 0, nil)
	want = []int{69, 32, 50, 64, 14, 64, -5, 32, 14, 0, 50, 0}
	for i := range want {
		if flat[i] != want[i] {
			t.Fatalf("flat corners = %v, want %v", flat, want)
		}
	}
}
func TestSnapPawnCentresEverySizeOnAHexGrid(t *testing.T) {
	for _, kind := range []GridType{GridHexPointy, GridHexFlat} {
		g := hexGrid(kind)
		var first [2]int
		for i, size := range []Size{SizeTiny, SizeMedium, SizeLarge, SizeGargantuan} {
			x, y := snapPawn(g, Pawn{Kind: PawnMonster, Size: size}, 100, 40)
			if i == 0 {
				first = [2]int{x, y}
				continue
			}
			if x != first[0] || y != first[1] {
				t.Fatalf("%s: a %s creature snapped to (%d, %d) where a tiny one snapped to (%d, %d); footprint parity has no meaning on hexes", kind, size, x, y, first[0], first[1])
			}
		}
		q, r := hexAt(g, 100, 40)
		cx, cy := hexCentre(g, q, r)
		if first[0] != cx || first[1] != cy {
			t.Fatalf("%s: a creature snapped to (%d, %d), want the hex centre at (%d, %d)", kind, first[0], first[1], cx, cy)
		}
	}
}
func TestSnapPawnOnAHexGridTreatsHalfCellsAsCells(t *testing.T) {
	g := hexGrid(GridHexPointy)
	whole := Pawn{Kind: PawnMonster, Size: SizeMedium}
	x, y := snapPawn(g, whole, 100, 40)
	g.Snap = SnapHalfCells
	hx, hy := snapPawn(g, whole, 100, 40)
	if hx != x || hy != y {
		t.Fatalf("half-cells snapped to (%d, %d) where cells snapped to (%d, %d)", hx, hy, x, y)
	}
}
func TestSnapPawnOnAHexGridLeavesOffAndObjectsAlone(t *testing.T) {
	g := hexGrid(GridHexFlat)
	g.Snap = SnapOff
	if x, y := snapPawn(g, Pawn{Kind: PawnMonster, Size: SizeMedium}, 100, 40); x != 100 || y != 40 {
		t.Fatalf("snapping off returned (%d, %d), want the input unchanged", x, y)
	}
	g.Snap = SnapCells
	if x, y := snapPawn(g, Pawn{Kind: PawnObject, Width: 128, Height: 192}, 100, 40); x != 100 || y != 40 {
		t.Fatalf("an object returned (%d, %d), want the input unchanged", x, y)
	}
}
