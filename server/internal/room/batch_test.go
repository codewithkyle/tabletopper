package room

import (
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
)

func TestABatchAppliesEveryCommandInOrderAndDerivesOneList(t *testing.T) {
	w := newWorld(t)
	pawn := w.spawn(Pawn{Name: "Goblin", X: 64, Y: 64, Visible: false})
	name, size := "Goblin boss", SizeLarge
	ch := w.change(&Batch{Commands: []Command{
		&PawnUpdate{ID: pawn, Name: &name, Size: &size},
		&PawnSetVisible{IDs: []ulid.ULID{pawn}, Visible: true},
	}}, w.gm)
	p := w.s.Pawn(pawn)
	if p.Name != name || p.Size != size || !p.Visible {
		t.Fatalf("the pawn is %+v; every command in the batch was meant to land", p)
	}
	equalStrings(t, "the GM's changes", changeTypesOf(ch.changes(RoleGM)), []string{"pawns.upserted"})
}
func TestABatchReportsTheRefusalOfTheCommandThatFailed(t *testing.T) {
	w := newWorld(t)
	pawn := w.spawn(Pawn{Name: "Goblin", Visible: true})
	name := "Goblin boss"
	got := w.refuse(&Batch{Commands: []Command{
		&PawnUpdate{ID: pawn, Name: &name},
		&PawnSetLayer{IDs: []ulid.ULID{pawn}, Layer: testID(900)},
	}}, w.gm, CodeNotFound)
	if got.Heading != "Layer gone" {
		t.Errorf("heading = %q, want the refusal of the command that failed", got.Heading)
	}
}
func TestABatchByAPlayerCarryingAGMCommandIsRefusedBeforeAnythingApplies(t *testing.T) {
	w := newWorld(t)
	pawn := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", X: 96, Y: 96, Visible: true, OwnerID: &testPlayerID})
	before := mustJSON(t, w.s)
	name := "Ari the Bold"
	got := w.refuse(&Batch{Commands: []Command{
		&PawnUpdate{ID: pawn, Name: &name},
		&PawnSetVisible{IDs: []ulid.ULID{pawn}, Visible: false},
	}}, w.pc, CodeForbidden)
	if !strings.Contains(got.Message, "hide or reveal") {
		t.Errorf("the refusal is %q, want the one the player may not do", got.Message)
	}
	if now := mustJSON(t, w.s); now != before {
		t.Fatal("a batch the player may not finish changed the room before it was refused")
	}
}
func TestABatchResolvesEveryResolverInsideIt(t *testing.T) {
	w := newWorld(t)
	lib := newLibrary()
	lib.maps[testAssetID] = MapRef{
		AssetID: testAssetID, Gen: testID(60), Width: 2048, Height: 2048, TileSize: 512, MaxZoom: 2,
	}
	mapped := &TableSetLayerMap{Layer: w.layer, AssetID: testAssetID}
	batch := &Batch{Commands: []Command{
		&TableRenameLayer{Layer: w.layer, Name: "Crypt"},
		mapped,
	}}
	w.resolve(batch, lib)
	if mapped.Map == nil {
		t.Fatal("the resolver inside the batch was never resolved")
	}
	equalStrings(t, "the library reads", lib.reads, []string{"map"})
	w.apply(batch, w.gm)
	l := w.s.Layer(w.layer)
	if l.Name != "Crypt" || l.Map == nil {
		t.Fatalf("the layer is %+v, want renamed and mapped", l)
	}
}
func TestABatchOfNothingIsAppliedAndChangesNothing(t *testing.T) {
	w := newWorld(t)
	before := mustJSON(t, w.s)
	ch := w.change(&Batch{}, w.gm)
	if now := mustJSON(t, w.s); now != before {
		t.Fatal("an empty batch changed the room")
	}
	if got := ch.changes(RoleGM); len(got) != 0 {
		t.Fatalf("an empty batch derived %v", changeTypesOf(got))
	}
}
