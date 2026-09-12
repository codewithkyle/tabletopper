package room

import (
	"testing"

	"github.com/oklog/ulid/v2"
)

func layerNamed(t *testing.T, s *State, id ulid.ULID) Layer {
	t.Helper()
	for _, l := range s.Table.Layers {
		if l.ID.Compare(id) == 0 {
			return l
		}
	}
	t.Fatalf("there is no layer %s", id)
	return Layer{}
}
func TestAFirstRevealCoversTheFloorItIsCutOutOf(t *testing.T) {
	w := newWorld(t)
	if l := layerNamed(t, w.s, w.layer); l.FogEnabled {
		t.Fatal("a new room's floor already has fog on")
	}
	ch := w.change(&FogAdd{
		Layer: w.layer, Kind: ShapeRect, Mode: FogReveal, Points: []int{0, 0, 100, 100},
	}, w.gm)
	l := layerNamed(t, w.s, w.layer)
	if !l.FogEnabled {
		t.Error("the first shape did not turn the fog on")
	}
	if !l.FogPrefill {
		t.Error("a reveal did not leave the floor covered; the hole is cut in nothing")
	}
	equalStrings(t, "the waking add", changeTypesOf(ch.changes(RoleGM)), []string{"fog.upserted", "layers.updated"})
}
func TestAFirstHideLeavesTheFloorClearUnderIt(t *testing.T) {
	w := newWorld(t)
	w.apply(&FogAdd{
		Layer: w.layer, Kind: ShapeRect, Mode: FogHide, Points: []int{0, 0, 100, 100},
	}, w.gm)
	l := layerNamed(t, w.s, w.layer)
	if !l.FogEnabled {
		t.Error("the first shape did not turn the fog on")
	}
	if l.FogPrefill {
		t.Error("a hide covered the whole floor; the patch is drawn on nothing")
	}
}
func TestASecondShapeDoesNotResendTheTable(t *testing.T) {
	w := newWorld(t)
	w.apply(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogReveal, Points: []int{0, 0, 10, 10}}, w.gm)
	ch := w.change(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogReveal, Points: []int{20, 20, 30, 30}}, w.gm)
	equalStrings(t, "a second add", changeTypesOf(ch.changes(RoleGM)), []string{"fog.upserted"})
}
func TestAHideOnAnAwakeFloorLeavesThePrefillAlone(t *testing.T) {
	w := newWorld(t)
	w.apply(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogReveal, Points: []int{0, 0, 10, 10}}, w.gm)
	w.apply(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogHide, Points: []int{2, 2, 6, 6}}, w.gm)
	if !layerNamed(t, w.s, w.layer).FogPrefill {
		t.Error("a hide over a reveal uncovered the rest of the floor")
	}
}
func TestThePrefillSwitchSeedsANewFloorAndTouchesNoOldOne(t *testing.T) {
	w := newWorld(t)
	w.apply(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogHide, Points: []int{0, 0, 10, 10}}, w.gm)
	w.apply(&TableSetOptions{
		PawnLabels:         LabelsDefault,
		InitiativeGrouping: GroupMonsters,
		FogPrefill:         true,
	}, w.gm)
	if !w.s.Table.FogPrefill {
		t.Fatal("the switch did not stick")
	}
	if layerNamed(t, w.s, w.layer).FogPrefill {
		t.Error("the switch covered a floor that was already on the table")
	}
	cellar := w.addLayer("Cellar")
	l := layerNamed(t, w.s, cellar)
	if !l.FogEnabled || !l.FogPrefill {
		t.Errorf("a floor added under the switch is not covered: enabled=%v prefill=%v", l.FogEnabled, l.FogPrefill)
	}
}
func TestAFloorAddedWithTheSwitchOffArrivesClear(t *testing.T) {
	w := newWorld(t)
	attic := w.addLayer("Attic")
	if layerNamed(t, w.s, attic).FogEnabled {
		t.Error("a floor added with the switch off arrived covered")
	}
}
func TestClearingOneFloorsFogLeavesTheOtherFloorsAlone(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	w.apply(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogReveal, Points: []int{0, 0, 10, 10}}, w.gm)
	w.apply(&FogAdd{Layer: cellar, Kind: ShapeRect, Mode: FogReveal, Points: []int{0, 0, 10, 10}}, w.gm)
	w.apply(&FogClear{Layer: w.layer}, w.gm)
	if got := len(w.s.Fog); got != 1 {
		t.Fatalf("%d shapes are left, want 1", got)
	}
	if w.s.Fog[0].LayerID.Compare(cellar) != 0 {
		t.Error("the clear emptied the wrong floor")
	}
	if !layerNamed(t, w.s, w.layer).FogEnabled {
		t.Error("fog.clear turned the fog off as well; that belongs to fog.setEnabled")
	}
}
