package room

import (
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
)

// EVERY LIMIT AT ITS BOUNDARY AND ONE PAST IT. A limit tested only from a long
// way away is a limit whose comparison could be the wrong one and still pass,
// and off-by-one on a bound is the mistake this table exists to catch.
//
// The limits that bound a collection are set up by filling the state rather
// than by running a thousand commands. That is the one place in this package's
// tests that writes to State directly, and it is deliberate: running the
// command a thousand times would test the loop, not the bound.
func TestLimitsHoldAtTheirBoundary(t *testing.T) {
	t.Run("a name of exactly the limit is accepted and one more is not", func(t *testing.T) {
		w := newWorld(t)
		layer := w.addLayer("Cellar")

		w.apply(&TableRenameLayer{Layer: layer, Name: strings.Repeat("a", NameLimit)}, w.gm)
		w.refuse(&TableRenameLayer{Layer: layer, Name: strings.Repeat("a", NameLimit+1)}, w.gm, CodeInvalid)
	})

	t.Run("a name is counted in runes rather than bytes", func(t *testing.T) {
		w := newWorld(t)
		layer := w.addLayer("Cellar")

		// Three bytes each. A byte count would refuse this at 43 characters.
		w.apply(&TableRenameLayer{Layer: layer, Name: strings.Repeat("地", NameLimit)}, w.gm)
	})

	t.Run("a layer needs a name at all", func(t *testing.T) {
		w := newWorld(t)

		w.refuse(&TableAddLayer{Name: "   "}, w.gm, CodeInvalid)
	})

	t.Run("a condition name stops at its own shorter limit", func(t *testing.T) {
		w := newWorld(t)
		pawn := w.spawn(Pawn{Visible: true})

		w.apply(&PawnSetConditions{ID: pawn, Conditions: []Condition{
			{Name: strings.Repeat("a", ConditionNameLimit), Color: ColorRed, Duration: -1, Clear: ClearEnd},
		}}, w.gm)
		w.refuse(&PawnSetConditions{ID: pawn, Conditions: []Condition{
			{Name: strings.Repeat("a", ConditionNameLimit+1), Color: ColorRed, Duration: -1, Clear: ClearEnd},
		}}, w.gm, CodeInvalid)
	})

	t.Run("a coordinate stops at the coordinate limit", func(t *testing.T) {
		w := newWorld(t)

		w.apply(&Ping{Layer: w.layer, X: CoordLimit, Y: -CoordLimit}, w.gm)
		w.refuse(&Ping{Layer: w.layer, X: CoordLimit + 1}, w.gm, CodeInvalid)
		w.refuse(&Ping{Layer: w.layer, Y: -CoordLimit - 1}, w.gm, CodeInvalid)
	})

	t.Run("hit points stop at their limit and need a maximum", func(t *testing.T) {
		w := newWorld(t)
		pawn := w.spawn(Pawn{Visible: true})

		w.apply(&PawnUpdate{ID: pawn, HP: intp(HPLimit), MaxHP: intp(HPLimit)}, w.gm)
		w.refuse(&PawnUpdate{ID: pawn, MaxHP: intp(HPLimit + 1)}, w.gm, CodeInvalid)
		w.refuse(&PawnUpdate{ID: pawn, MaxHP: intp(0)}, w.gm, CodeInvalid)

		bare := w.spawn(Pawn{Visible: true})
		w.refuse(&PawnUpdate{ID: bare, HP: intp(5)}, w.gm, CodeInvalid)
	})

	t.Run("hit points are clamped into the pawn's own range", func(t *testing.T) {
		w := newWorld(t)
		pawn := w.spawn(Pawn{Visible: true, HP: intp(10), MaxHP: intp(10)})

		w.apply(&PawnUpdate{ID: pawn, HP: intp(40)}, w.gm)
		if got := *w.s.Pawn(pawn).HP; got != 10 {
			t.Fatalf("hit points above the maximum were stored as %d, want 10", got)
		}
	})

	t.Run("armour class stops at its limit", func(t *testing.T) {
		w := newWorld(t)
		pawn := w.spawn(Pawn{Visible: true})

		w.apply(&PawnUpdate{ID: pawn, AC: intp(ACLimit)}, w.gm)
		w.refuse(&PawnUpdate{ID: pawn, AC: intp(ACLimit + 1)}, w.gm, CodeInvalid)
	})

	t.Run("the grid's cell size stops at both ends", func(t *testing.T) {
		w := newWorld(t)
		g := w.s.Table.Grid

		g.CellSize = CellSizeMin
		w.apply(&TableSetGrid{Grid: g}, w.gm)
		g.CellSize = CellSizeMax
		w.apply(&TableSetGrid{Grid: g}, w.gm)

		g.CellSize = CellSizeMin - 1
		w.refuse(&TableSetGrid{Grid: g}, w.gm, CodeInvalid)
		g.CellSize = CellSizeMax + 1
		w.refuse(&TableSetGrid{Grid: g}, w.gm, CodeInvalid)
	})

	t.Run("a colour is six or eight hex digits behind a hash", func(t *testing.T) {
		w := newWorld(t)
		g := w.s.Table.Grid

		for _, good := range []string{"#000000", "#00000000", "#AbCdEf", "#abcdef12"} {
			g.Color = good
			w.apply(&TableSetGrid{Grid: g}, w.gm)
		}
		for _, bad := range []string{"black", "#fff", "000000", "#gggggg", "#0000000"} {
			g.Color = bad
			w.refuse(&TableSetGrid{Grid: g}, w.gm, CodeInvalid)
		}
	})

	t.Run("a stroke stops at the maximum width", func(t *testing.T) {
		w := newWorld(t)

		w.apply(&StrokeBegin{ID: testID(600), Layer: w.layer, Color: "#ffffff", Width: StrokeWidthMax, Points: []int{0, 0}}, w.gm)
		w.refuse(&StrokeBegin{ID: testID(601), Layer: w.layer, Color: "#ffffff", Width: StrokeWidthMax + 1, Points: []int{0, 0}}, w.gm, CodeInvalid)
		w.refuse(&StrokeBegin{ID: testID(602), Layer: w.layer, Color: "#ffffff", Width: 0, Points: []int{0, 0}}, w.gm, CodeInvalid)
	})

	t.Run("one stroke chunk stops at the chunk limit", func(t *testing.T) {
		w := newWorld(t)

		w.apply(&StrokeBegin{ID: testID(610), Layer: w.layer, Color: "#ffffff", Width: 2, Points: points(StrokeChunkMax)}, w.gm)
		w.refuse(&StrokeBegin{ID: testID(611), Layer: w.layer, Color: "#ffffff", Width: 2, Points: points(StrokeChunkMax + 2)}, w.gm, CodeInvalid)
	})

	t.Run("a point array has to be pairs", func(t *testing.T) {
		w := newWorld(t)

		w.refuse(&StrokeBegin{ID: testID(620), Layer: w.layer, Color: "#ffffff", Width: 2, Points: []int{0, 0, 5}}, w.gm, CodeInvalid)
		w.refuse(&StrokeBegin{ID: testID(621), Layer: w.layer, Color: "#ffffff", Width: 2, Points: []int{}}, w.gm, CodeInvalid)
	})

	t.Run("one whole stroke stops at the total limit", func(t *testing.T) {
		w := newWorld(t)
		id := testID(630)
		w.apply(&StrokeBegin{ID: id, Layer: w.layer, Color: "#ffffff", Width: 2, Points: []int{0, 0}}, w.gm)

		// Filled rather than drawn: forty chunks would test the loop.
		w.s.Stroke(id).Points = points(StrokePointsMax - 2)

		w.apply(&StrokeExtend{ID: id, Points: points(2)}, w.gm)
		w.refuse(&StrokeExtend{ID: id, Points: points(2)}, w.gm, CodeInvalid)
	})

	t.Run("one fog polygon stops at its point limit", func(t *testing.T) {
		w := newWorld(t)

		w.apply(&FogAdd{Layer: w.layer, Kind: ShapePoly, Mode: FogHide, Points: points(FogPointsMax)}, w.gm)
		w.refuse(&FogAdd{Layer: w.layer, Kind: ShapePoly, Mode: FogHide, Points: points(FogPointsMax + 2)}, w.gm, CodeInvalid)
	})

	t.Run("a fog rectangle is exactly two corners and a polygon at least three", func(t *testing.T) {
		w := newWorld(t)

		w.apply(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogHide, Points: []int{0, 0, 10, 10}}, w.gm)
		w.refuse(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogHide, Points: []int{0, 0, 10, 10, 20, 20}}, w.gm, CodeInvalid)
		w.refuse(&FogAdd{Layer: w.layer, Kind: ShapePoly, Mode: FogHide, Points: []int{0, 0, 10, 10}}, w.gm, CodeInvalid)
	})

	t.Run("the room stops at its fog shape count", func(t *testing.T) {
		w := newWorld(t)
		w.s.Fog = make([]FogShape, FogShapesMax)

		w.refuse(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogHide, Points: []int{0, 0, 1, 1}}, w.gm, CodeInvalid)
	})

	t.Run("the room stops at its stroke count", func(t *testing.T) {
		w := newWorld(t)
		w.s.Strokes = make([]Stroke, StrokesMax)

		w.refuse(&StrokeBegin{ID: testID(640), Layer: w.layer, Color: "#ffffff", Width: 2, Points: []int{0, 0}}, w.gm, CodeInvalid)
	})

	t.Run("the table stops at its pawn count", func(t *testing.T) {
		w := newWorld(t)
		w.s.Pawns = make([]Pawn, PawnsMax)

		_, err := w.run(&PawnSpawn{Kind: PawnMonster, Layer: w.layer, Pawn: &Pawn{Size: SizeMedium}}, w.gm)
		if err == nil {
			t.Fatal("a full table accepted another pawn")
		}
	})

	t.Run("a pawn stops at its condition count", func(t *testing.T) {
		w := newWorld(t)
		pawn := w.spawn(Pawn{Visible: true})

		w.apply(&PawnSetConditions{ID: pawn, Conditions: conditions(ConditionsMax)}, w.gm)
		w.refuse(&PawnSetConditions{ID: pawn, Conditions: conditions(ConditionsMax + 1)}, w.gm, CodeInvalid)
	})

	t.Run("a selection stops at the selection limit", func(t *testing.T) {
		w := newWorld(t)

		ids := make([]ulid.ULID, 0, SelectionMax)
		for range SelectionMax {
			ids = append(ids, w.spawn(Pawn{Visible: true}))
		}

		w.apply(&PawnMove{Anchor: ids[0], X: 64, Y: 64, Others: ids[1:]}, w.gm)

		ids = append(ids, w.spawn(Pawn{Visible: true}))
		w.refuse(&PawnMove{Anchor: ids[0], X: 128, Y: 128, Others: ids[1:]}, w.gm, CodeInvalid)
	})

	t.Run("an object's footprint stops at both ends", func(t *testing.T) {
		w := newWorld(t)

		w.spawn(Pawn{Kind: PawnObject, FootprintW: FootprintMax, FootprintH: FootprintMax, Visible: true})

		for _, bad := range [][2]int{{0, 1}, {1, 0}, {FootprintMax + 1, 1}, {1, FootprintMax + 1}} {
			_, err := w.run(&PawnSpawn{
				Kind: PawnObject, Layer: w.layer, FootprintW: bad[0], FootprintH: bad[1],
				Pawn: &Pawn{},
			}, w.gm)
			if err == nil {
				t.Fatalf("an object of %dx%d cells was accepted", bad[0], bad[1])
			}
		}
	})

	t.Run("the tracker stops at its entry count", func(t *testing.T) {
		w := newWorld(t)

		w.apply(&InitiativeSet{Entries: entries(InitiativeMax)}, w.gm)
		w.refuse(&InitiativeSet{Entries: entries(InitiativeMax + 1)}, w.gm, CodeInvalid)
	})

	t.Run("the room stops at its layer count", func(t *testing.T) {
		w := newWorld(t)

		for len(w.s.Table.Layers) < LayersMax {
			w.addLayer("Another floor")
		}
		w.refuse(&TableAddLayer{Name: "One too many"}, w.gm, CodeInvalid)
	})

	t.Run("a condition's duration is at least one turn or minus one", func(t *testing.T) {
		w := newWorld(t)
		pawn := w.spawn(Pawn{Visible: true})

		for _, good := range []int{-1, 1, 10} {
			w.apply(&PawnSetConditions{ID: pawn, Conditions: []Condition{
				{Name: "Poisoned", Color: ColorGreen, Duration: good, Clear: ClearEnd},
			}}, w.gm)
		}
		for _, bad := range []int{0, -2} {
			w.refuse(&PawnSetConditions{ID: pawn, Conditions: []Condition{
				{Name: "Poisoned", Color: ColorGreen, Duration: bad, Clear: ClearEnd},
			}}, w.gm, CodeInvalid)
		}
	})
}

func points(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i % 512
	}

	return out
}

func conditions(n int) []Condition {
	out := make([]Condition, 0, n)
	for range n {
		out = append(out, Condition{Name: "Prone", Color: ColorWhite, Duration: -1, Clear: ClearEnd})
	}

	return out
}

func entries(n int) []InitiativeEntry {
	out := make([]InitiativeEntry, 0, n)
	for range n {
		out = append(out, InitiativeEntry{Name: "Goblin"})
	}

	return out
}
