package room

import (
	"testing"

	"github.com/oklog/ulid/v2"
)

// THE ROOM-WIDE BUDGET, which the per-item limits do not give. A player holding
// a key down at the rate limit adds thirty thousand points a second, and the
// per-stroke and per-room counts together would let a room hold a hundred
// million integers -- a snapshot the column refuses, after which the room can
// never be saved again.
func TestTheDrawingBudgetRefusesTheChunkThatWouldOverflowIt(t *testing.T) {
	t.Run("the room as a whole", func(t *testing.T) {
		w := newWorld(t)

		// Filled by hand to just under the line: reaching it through the
		// commands would be tens of thousands of chunks for one assertion.
		w.s.Strokes = append(w.s.Strokes, Stroke{
			ID: testID(700), By: testGMID, LayerID: w.layer, Color: "#ffffff", Width: 2,
			Points: points(StrokePointsBudget - 2),
		})
		w.s.Normalize()

		w.apply(&StrokeBegin{ID: testID(701), Layer: w.layer, Kind: StrokeFree, Color: "#ffffff", Width: 2, Points: []int{0, 0}}, w.gm)
		e := w.refuse(&StrokeExtend{ID: testID(701), Points: []int{1, 1}}, w.gm, CodeInvalid)
		if e.Heading != "Drawing full" {
			t.Errorf("heading = %q, want the budget's", e.Heading)
		}
		w.refuse(&StrokeBegin{ID: testID(702), Layer: w.layer, Kind: StrokeFree, Color: "#ffffff", Width: 2, Points: []int{0, 0}}, w.gm, CodeInvalid)

		// Erasing makes room again.
		w.apply(&StrokeErase{IDs: []ulid.ULID{testID(700)}}, w.gm)
		w.apply(&StrokeExtend{ID: testID(701), Points: []int{1, 1}}, w.gm)
	})

	t.Run("one player's share of it", func(t *testing.T) {
		w := newWorld(t)

		share := StrokePointsBudget / PlayerStrokeShare
		w.s.Strokes = append(w.s.Strokes, Stroke{
			ID: testID(710), By: testPlayerID, LayerID: w.layer, Color: "#ffffff", Width: 2,
			Points: points(share - 2),
		})
		w.s.Normalize()

		w.apply(&StrokeBegin{ID: testID(711), Layer: w.layer, Kind: StrokeFree, Color: "#ffffff", Width: 2, Points: []int{0, 0}}, w.pc)
		w.refuse(&StrokeExtend{ID: testID(711), Points: []int{1, 1}}, w.pc, CodeInvalid)

		// The table is nowhere near full, so the other player and the GM go
		// on drawing.
		w.apply(&StrokeBegin{ID: testID(712), Layer: w.layer, Kind: StrokeFree, Color: "#ffffff", Width: 2, Points: []int{0, 0}}, w.other)
		w.apply(&StrokeBegin{ID: testID(713), Layer: w.layer, Kind: StrokeFree, Color: "#ffffff", Width: 2, Points: []int{0, 0}}, w.gm)
	})

	t.Run("the fog, which is the GM's alone", func(t *testing.T) {
		w := newWorld(t)

		w.s.Fog = append(w.s.Fog, FogShape{
			ID: testID(720), LayerID: w.layer, Kind: ShapePoly, Mode: FogHide,
			Points: points(FogPointsBudget - 2),
		})
		w.s.Normalize()

		w.refuse(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogReveal, Points: []int{0, 0, 10, 10}}, w.gm, CodeInvalid)
		w.apply(&FogClear{Layer: w.layer}, w.gm)
		w.apply(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogReveal, Points: []int{0, 0, 10, 10}}, w.gm)
	})
}
