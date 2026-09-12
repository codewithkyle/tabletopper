package room

import (
	"slices"
	"testing"
)

func TestThePreviewsAreTheDragThePingAndTheSync(t *testing.T) {
	var found []string
	for wire, cmd := range WireCommandPrototypes() {
		if _, ok := cmd.(Preview); ok {
			found = append(found, wire)
		}
	}
	slices.Sort(found)
	want := []string{"pawn.drag", "ping", "sync.request"}
	if !slices.Equal(found, want) {
		t.Fatalf("%v are previews, want %v; a preview skips the clone and the derivation, which only a command that never mutates can afford", found, want)
	}
}
func TestNoCommandTheHubSendsItselfIsAPreview(t *testing.T) {
	for wire, cmd := range HubCommandPrototypes() {
		if _, ok := cmd.(Preview); ok {
			t.Errorf("%s is a preview, and the hub's own commands all change the room", wire)
		}
	}
}
func TestAPreviewLeavesTheStateAsItFoundIt(t *testing.T) {
	w := newWorld(t)
	pawn := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", X: 96, Y: 96, Visible: true, OwnerID: &testPlayerID})
	before := mustJSON(t, w.s)
	previews := []struct {
		name string
		cmd  Command
		by   Actor
	}{
		{"a drag", &PawnDrag{Anchor: pawn, X: 320, Y: 320}, w.pc},
		{"a ping", &Ping{Layer: w.layer, X: 128, Y: 128}, w.pc},
		{"a sync", &SyncRequest{}, w.pc},
	}
	for _, p := range previews {
		if _, ok := p.cmd.(Preview); !ok {
			t.Fatalf("%s is not a preview", p.name)
		}
		if sigs := w.apply(p.cmd, p.by); len(sigs) == 0 {
			t.Errorf("%s sent nothing", p.name)
		}
		if got := mustJSON(t, w.s); got != before {
			t.Fatalf("%s changed the room\n before %s\n  after %s", p.name, before, got)
		}
	}
}
