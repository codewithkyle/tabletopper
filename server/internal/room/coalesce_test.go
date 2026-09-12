package room

import (
	"strings"
	"testing"
)

func TestADragCoalescesUnderItsAnchor(t *testing.T) {
	one, two := testID(41), testID(42)
	first := (&PawnDrag{Anchor: one, X: 10, Y: 10}).CoalesceKey()
	again := (&PawnDrag{Anchor: one, X: 300, Y: 300}).CoalesceKey()
	other := (&PawnDrag{Anchor: two, X: 10, Y: 10}).CoalesceKey()
	if first != again {
		t.Errorf("two drags of one anchor coalesce under %q and %q; the later must replace the earlier", first, again)
	}
	if first == other {
		t.Errorf("drags of two anchors both coalesce under %q; one gesture would swallow the other", first)
	}
	if !strings.Contains(first, one.String()) {
		t.Errorf("the key is %q and names no anchor", first)
	}
}
func TestTheDragIsTheOnlyCommandThatCoalesces(t *testing.T) {
	var found []string
	for wire, cmd := range WireCommandPrototypes() {
		if _, ok := cmd.(Coalescer); ok {
			found = append(found, wire)
		}
	}
	if len(found) != 1 || found[0] != "pawn.drag" {
		t.Fatalf("%v coalesce; coalescing drops the commands it replaces, which only a preview can afford", found)
	}
}
func TestNoCommandTheHubSendsItselfCoalesces(t *testing.T) {
	for wire, cmd := range HubCommandPrototypes() {
		if _, ok := cmd.(Coalescer); ok {
			t.Errorf("%s coalesces, and the hub's own commands never pass through the window", wire)
		}
	}
}
