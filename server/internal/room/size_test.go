package room

import (
	"testing"

	"tabletopper/internal/dnd"
)

func TestRoomSizeValuesMatchDnd(t *testing.T) {
	named := []Size{SizeTiny, SizeSmall, SizeMedium, SizeLarge, SizeHuge, SizeGargantuan}
	values := dnd.SizeValues()
	if len(named) != len(values) {
		t.Fatalf("the room names %d sizes and the rules list %d", len(named), len(values))
	}
	for i, size := range named {
		if string(size) != values[i] {
			t.Errorf("size %d is %q in the room and %q in the rules", i, size, values[i])
		}
		if !size.Valid() {
			t.Errorf("%q is not a valid room size", size)
		}
	}
	if Size(dnd.DefaultSize) != SizeMedium {
		t.Errorf("the rules default to %q, want medium", dnd.DefaultSize)
	}
}
