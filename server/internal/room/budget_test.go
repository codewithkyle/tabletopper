package room

import (
	"testing"

	"github.com/oklog/ulid/v2"
)






func TestTheDrawingBudgetRefusesTheChunkThatWouldOverflowIt(t *testing.T) {
	t.Run("the room as a whole", func(t *testing.T) {
		w := newWorld(t)

		
		
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
