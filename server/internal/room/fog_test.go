package room

import (
	"testing"

	"github.com/oklog/ulid/v2"
)

// layerNamed answers one floor by id, so a test can read its two flags without
// reaching for an index that another test's ordering could move.
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

// THE FIRST SHAPE WAKES THE FLOOR, AND THE MODE SAYS WHICH WAY ROUND. Without
// this a GM picks the Fog tool on a fresh floor, drags a rectangle, and nothing
// happens on any screen -- and the reason is a flag they have never seen.
//
// The two directions are separate tests because they set the prefill to opposite
// values, and a single test asserting one of them would pass against an
// implementation that always wrote true.
func TestAFirstRevealCoversTheFloorItIsCutOutOf(t *testing.T) {
	w := newWorld(t)

	if l := layerNamed(t, w.s, w.layer); l.FogEnabled {
		t.Fatal("a new room's floor already has fog on")
	}

	ems := w.apply(&FogAdd{
		Layer: w.layer, Kind: ShapeRect, Mode: FogReveal, Points: []int{0, 0, 100, 100},
	}, w.gm)

	l := layerNamed(t, w.s, w.layer)
	if !l.FogEnabled {
		t.Error("the first shape did not turn the fog on")
	}
	if !l.FogPrefill {
		t.Error("a reveal did not leave the floor covered; the hole is cut in nothing")
	}

	// The table goes first, so nobody draws a frame of a hole in a floor they
	// still believe is clear.
	if len(ems) != 2 {
		t.Fatalf("the waking add emitted %d events, want 2", len(ems))
	}
	if got := ems[0].Event.eventType(); got != "table.updated" {
		t.Errorf("the first event is %s, want table.updated", got)
	}
	if got := ems[1].Event.eventType(); got != "fog.added" {
		t.Errorf("the second event is %s, want fog.added", got)
	}
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

// AND ONLY THE FIRST ONE. Every add after the floor is awake is one event, not
// two: a table.updated per shape would rebroadcast the whole table on every
// reveal, which on a well-explored floor is hundreds of copies of a thing that
// did not change.
func TestASecondShapeDoesNotResendTheTable(t *testing.T) {
	w := newWorld(t)

	w.apply(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogReveal, Points: []int{0, 0, 10, 10}}, w.gm)
	ems := w.apply(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogReveal, Points: []int{20, 20, 30, 30}}, w.gm)

	if len(ems) != 1 {
		t.Fatalf("a second add emitted %d events, want 1", len(ems))
	}
	if got := ems[0].Event.eventType(); got != "fog.added" {
		t.Errorf("the event is %s, want fog.added", got)
	}
}

// A HIDE ON AN AWAKE FLOOR DOES NOT FLIP IT. Waking is what reads the mode; a
// GM covering part of a floor they have been revealing all evening has not
// asked for the other three quarters of it to come back.
func TestAHideOnAnAwakeFloorLeavesThePrefillAlone(t *testing.T) {
	w := newWorld(t)

	w.apply(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogReveal, Points: []int{0, 0, 10, 10}}, w.gm)
	w.apply(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogHide, Points: []int{2, 2, 6, 6}}, w.gm)

	if !layerNamed(t, w.s, w.layer).FogPrefill {
		t.Error("a hide over a reveal uncovered the rest of the floor")
	}
}

// THE ROOM-WIDE SWITCH IS A DEFAULT FOR THE NEXT FLOOR AND NOTHING ELSE. Both
// halves are asserted together because the failure people would actually ship
// is the other one: a switch that covered every floor already on the table
// throws away an evening of revealing on all of them at once.
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

	// The ground floor keeps the clear prefill its own hide gave it.
	if layerNamed(t, w.s, w.layer).FogPrefill {
		t.Error("the switch covered a floor that was already on the table")
	}

	cellar := w.addLayer("Cellar")
	l := layerNamed(t, w.s, cellar)
	if !l.FogEnabled || !l.FogPrefill {
		t.Errorf("a floor added under the switch is not covered: enabled=%v prefill=%v", l.FogEnabled, l.FogPrefill)
	}
}

// And the other way round, which is the shipping default: a new floor arrives
// with its fog off and waits to be drawn on.
func TestAFloorAddedWithTheSwitchOffArrivesClear(t *testing.T) {
	w := newWorld(t)

	attic := w.addLayer("Attic")
	if layerNamed(t, w.s, attic).FogEnabled {
		t.Error("a floor added with the switch off arrived covered")
	}
}

// CLEARING ONE FLOOR LEAVES THE OTHERS ALONE. The command names a layer rather
// than listing shapes -- a well-explored floor holds hundreds -- and a filter
// written the wrong way round would empty the building.
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

	// The flags are untouched: clearing is emptying, and turning the fog off is
	// its own command. The Fog menu's Clear item sends both, in that order.
	if !layerNamed(t, w.s, w.layer).FogEnabled {
		t.Error("fog.clear turned the fog off as well; that belongs to fog.setEnabled")
	}
}
