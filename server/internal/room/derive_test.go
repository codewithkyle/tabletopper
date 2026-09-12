package room

import (
	"testing"

	"github.com/oklog/ulid/v2"
)

func TestRevealingAPawnSpawnsItForPlayersAndHidingItTakesItAway(t *testing.T) {
	w := newWorld(t)
	goblin := w.spawn(Pawn{Name: "Goblin", Visible: false, HP: intp(3), MaxHP: intp(7), AC: intp(15)})
	reveal := w.change(&PawnSetVisible{IDs: []ulid.ULID{goblin}, Visible: true}, w.gm)
	equalStrings(t, "the GM", changeTypesOf(reveal.changes(RoleGM)), []string{"pawns.upserted"})
	shown := reveal.changes(RolePlayer)
	equalStrings(t, "the players", changeTypesOf(shown), []string{"pawns.upserted"})
	p := onePawn(t, shown[0])
	if p.ID != goblin {
		t.Fatalf("the players were given pawn %s, want %s", p.ID, goblin)
	}
	if p.AC != nil {
		t.Fatal("the upserted copy carries the armour class the projection removes")
	}
	if p.HPBand == nil || *p.HPBand != BandBloody {
		t.Fatalf("the upserted copy's band is %v, want the projected bloody", Health(p))
	}
	hide := w.change(&PawnSetVisible{IDs: []ulid.ULID{goblin}, Visible: false}, w.gm)
	equalStrings(t, "the GM", changeTypesOf(hide.changes(RoleGM)), []string{"pawns.upserted"})
	gone := hide.changes(RolePlayer)
	equalStrings(t, "the players", changeTypesOf(gone), []string{"pawns.removed"})
	if ids := gone[0].(*PawnsRemoved).IDs; len(ids) != 1 || ids[0] != goblin {
		t.Fatalf("the players were told %v left, want %s", ids, goblin)
	}
}
func TestChangingTheActiveLayerSwapsTheFloorUnderThePlayers(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	upstairs := w.spawn(Pawn{Name: "Upstairs", Visible: true})
	downstairs := w.spawn(Pawn{Name: "Downstairs", LayerID: cellar, Visible: true})
	w.spawn(Pawn{Name: "Hidden downstairs", LayerID: cellar, Visible: false})
	ch := w.change(&TableSetActiveLayer{Layer: cellar}, w.gm)
	equalStrings(t, "the GM", changeTypesOf(ch.changes(RoleGM)), []string{"table.updated"})
	players := ch.changes(RolePlayer)
	equalStrings(t, "the players", changeTypesOf(players), []string{"pawns.removed", "pawns.upserted", "table.updated"})
	if ids := players[0].(*PawnsRemoved).IDs; len(ids) != 1 || ids[0] != upstairs {
		t.Fatal("the wrong pawn was taken off the players' table")
	}
	if got := onePawn(t, players[1]).ID; got != downstairs {
		t.Fatal("the wrong pawn was put on the players' table")
	}
}
func TestTurningOnExactHitPointsReprojectsEveryShownMonster(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	goblin := w.spawn(Pawn{Kind: PawnMonster, Name: "Goblin", Visible: true, HP: intp(5), MaxHP: intp(7), AC: intp(15)})
	innkeeper := w.spawn(Pawn{Kind: PawnNPC, Name: "Innkeeper", Visible: true, HP: intp(9), MaxHP: intp(9), AC: intp(12)})
	w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true, HP: intp(12), MaxHP: intp(12), AC: intp(16)})
	w.spawn(Pawn{Kind: PawnMonster, Name: "Ambusher", Visible: false, HP: intp(5), MaxHP: intp(5), AC: intp(15)})
	w.spawn(Pawn{Kind: PawnMonster, Name: "Downstairs", LayerID: cellar, Visible: true, HP: intp(5), MaxHP: intp(5), AC: intp(15)})
	ch := w.change(&TableSetOptions{PawnLabels: LabelsFull, PlayersCanDraw: true, InitiativeGrouping: GroupMonsters}, w.gm)
	equalStrings(t, "the GM", changeTypesOf(ch.changes(RoleGM)), []string{"table.updated"})
	players := ch.changes(RolePlayer)
	equalStrings(t, "the players", changeTypesOf(players), []string{"pawns.upserted", "table.updated"})
	reprojected := players[0].(*PawnsUpserted).Pawns
	if len(reprojected) != 2 {
		t.Fatalf("the upsert carries %d pawns, want the two shown creatures", len(reprojected))
	}
	for i, want := range []ulid.ULID{goblin, innkeeper} {
		p := reprojected[i]
		if p.ID != want {
			t.Fatalf("position %d carries pawn %s, want %s", i, p.ID, want)
		}
		if p.AC == nil {
			t.Fatalf("%s was reprojected without the armour class the change was about", p.Name)
		}
	}
	again := w.change(&TableSetOptions{PawnLabels: LabelsFull, PlayersCanDraw: false, InitiativeGrouping: GroupMonsters}, w.gm)
	equalStrings(t, "the players", changeTypesOf(again.changes(RolePlayer)), []string{"table.updated"})
}
func TestAGroupMoveIsOnePawnsMovedCarryingWhatTheRoleMaySee(t *testing.T) {
	w := newWorld(t)
	first := w.spawn(Pawn{Name: "Ari", X: 96, Y: 96, Visible: true})
	second := w.spawn(Pawn{Name: "Rin", X: 160, Y: 96, Visible: true})
	hidden := w.spawn(Pawn{Name: "Ambusher", X: 224, Y: 96, Visible: false})
	ch := w.change(&PawnMove{Anchor: first, X: 300, Y: 300, Others: []ulid.ULID{second, hidden}}, w.gm)
	gm := ch.changes(RoleGM)
	equalStrings(t, "the GM", changeTypesOf(gm), []string{"pawns.moved"})
	if got := len(gm[0].(*PawnsMoved).Pawns); got != 3 {
		t.Fatalf("the GM's move carries %d positions, want 3", got)
	}
	players := ch.changes(RolePlayer)
	equalStrings(t, "the players", changeTypesOf(players), []string{"pawns.moved"})
	at := players[0].(*PawnsMoved).Pawns
	if len(at) != 2 || at[0].ID != first || at[1].ID != second {
		t.Fatalf("the players' move carries %v, want the two they can see", at)
	}
	alone := w.spawn(Pawn{Name: "Second ambusher", X: 288, Y: 96, Visible: false})
	dark := w.change(&PawnMove{Anchor: hidden, X: 480, Y: 480, Others: []ulid.ULID{alone}}, w.gm)
	equalStrings(t, "the GM", changeTypesOf(dark.changes(RoleGM)), []string{"pawns.moved"})
	if got := dark.changes(RolePlayer); len(got) != 0 {
		t.Fatalf("a move of nothing but hidden pawns told the players %v", changeTypesOf(got))
	}
}
func TestHidingATrackedPawnDropsItFromThePlayersTrackerAlone(t *testing.T) {
	w := newWorld(t)
	goblin := w.spawn(Pawn{Name: "Goblin", Visible: true})
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}, Initiative: 12}}}, w.gm)
	ch := w.change(&PawnSetVisible{IDs: []ulid.ULID{goblin}, Visible: false}, w.gm)
	equalStrings(t, "the GM", changeTypesOf(ch.changes(RoleGM)), []string{"pawns.upserted"})
	players := ch.changes(RolePlayer)
	equalStrings(t, "the players", changeTypesOf(players), []string{"pawns.removed", "initiative.updated"})
	if got := players[1].(*InitiativeUpdated).Initiative.Entries; len(got) != 0 {
		t.Fatalf("the players' tracker still names the hidden pawn: %+v", got)
	}
	if len(w.s.Initiative.Entries) != 1 {
		t.Fatalf("the GM's tracker holds %d entries, want the one it always had", len(w.s.Initiative.Entries))
	}
}
func TestAStrokeIsExtendedThenEndedThenRewrittenWhole(t *testing.T) {
	w := newWorld(t)
	id := testID(700)
	begun := w.change(&StrokeBegin{
		ID: id, Layer: w.layer, Kind: StrokeFree, Color: "#ff0000ff", Width: 4, Points: []int{0, 0, 10, 10},
	}, w.gm)
	equalStrings(t, "beginning", changeTypesOf(begun.changes(RoleGM)), []string{"strokes.upserted"})
	grown := w.change(&StrokeExtend{ID: id, Points: []int{20, 20, 30, 30}}, w.gm)
	chs := grown.changes(RoleGM)
	equalStrings(t, "extending", changeTypesOf(chs), []string{"strokes.extended"})
	ext := chs[0].(*StrokeExtended)
	if ext.ID != id || len(ext.Points) != 4 || ext.Points[0] != 20 {
		t.Fatalf("the extension carries %v, want the four new numbers", ext.Points)
	}
	ended := w.change(&StrokeEnd{ID: id}, w.gm)
	equalStrings(t, "ending", changeTypesOf(ended.changes(RoleGM)), []string{"strokes.ended"})
	before := w.s.Clone()
	rewritten := w.s.Stroke(id)
	rewritten.Points = []int{99, 99}
	rewritten.Done = false
	chs = Derive(&before, w.s, RoleGM)
	equalStrings(t, "rewriting", changeTypesOf(chs), []string{"strokes.upserted"})
	whole := chs[0].(*StrokesUpserted).Strokes
	if len(whole) != 1 || len(whole[0].Points) != 2 || whole[0].Points[0] != 99 {
		t.Fatalf("the rewrite carries %v, want the whole stroke", whole)
	}
}
func TestRemovingALayerDerivesItsContentsInOrder(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	goblin := w.spawn(Pawn{Name: "Goblin", LayerID: cellar, Visible: true})
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}}}}, w.gm)
	w.apply(&FogAdd{Layer: cellar, Kind: ShapeRect, Mode: FogHide, Points: []int{0, 0, 64, 64}}, w.gm)
	w.apply(&FogAdd{Layer: cellar, Kind: ShapeRect, Mode: FogHide, Points: []int{64, 64, 128, 128}}, w.gm)
	w.apply(&StrokeBegin{ID: testID(701), Layer: cellar, Kind: StrokeFree, Color: "#ffffffff", Width: 2, Points: []int{0, 0}}, w.gm)
	w.apply(&StrokeBegin{ID: testID(702), Layer: cellar, Kind: StrokeFree, Color: "#ffffffff", Width: 2, Points: []int{5, 5}}, w.gm)
	w.apply(&TableSetActiveLayer{Layer: cellar}, w.gm)
	ch := w.change(&TableRemoveLayer{Layer: cellar}, w.gm)
	gm := ch.changes(RoleGM)
	equalStrings(t, "the GM", changeTypesOf(gm), []string{
		"fog.removed",
		"strokes.removed",
		"pawns.removed",
		"initiative.updated",
		"layers.updated",
		"table.updated",
	})
	if got := gm[0].(*FogRemoved).IDs; len(got) != 2 {
		t.Fatalf("the fog removal names %d shapes, want both in one change", len(got))
	}
	if got := gm[1].(*StrokesRemoved).IDs; len(got) != 2 {
		t.Fatalf("the erasure names %d strokes, want both in one change", len(got))
	}
	if got := gm[4].(*LayersUpdated).Layers; len(got) != 1 {
		t.Fatalf("the layer list carries %d layers, want the one that is left", len(got))
	}
}
func TestALayerRenameIsALayerChangeAndNothingElse(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	ch := w.change(&TableRenameLayer{Layer: cellar, Name: "Cellar, flooded"}, w.gm)
	for _, role := range []Role{RoleGM, RolePlayer} {
		equalStrings(t, "renaming for the "+string(role), changeTypesOf(ch.changes(role)), []string{"layers.updated"})
	}
	layers := ch.changes(RoleGM)[0].(*LayersUpdated).Layers
	if len(layers) != 2 || layers[1].Name != "Cellar, flooded" {
		t.Fatalf("the layer list is %+v", layers)
	}
}
func TestReorderingTheLayersIsCarriedByTheirOrder(t *testing.T) {
	w := newWorld(t)
	ground := w.layer
	cellar := w.addLayer("Cellar")
	ch := w.change(&TableMoveLayer{Layer: cellar, Index: 0}, w.gm)
	equalStrings(t, "the GM", changeTypesOf(ch.changes(RoleGM)), []string{"layers.updated"})
	layers := ch.changes(RoleGM)[0].(*LayersUpdated).Layers
	if len(layers) != 2 || layers[0].ID != cellar || layers[1].ID != ground {
		t.Fatal("the reordered list does not carry the new order")
	}
}
func TestOneChangeCarriesEveryPawnThatArrivedAndOneEveryPawnThatWent(t *testing.T) {
	w := newWorld(t)
	first := w.spawn(Pawn{Name: "First", Visible: true})
	second := w.spawn(Pawn{Name: "Second", Visible: true})
	ch := w.change(&PawnRemove{IDs: []ulid.ULID{first, second}}, w.gm)
	gm := ch.changes(RoleGM)
	equalStrings(t, "the GM", changeTypesOf(gm), []string{"pawns.removed"})
	if got := gm[0].(*PawnsRemoved).IDs; len(got) != 2 {
		t.Fatalf("the removal names %d pawns, want both in one change", len(got))
	}
}
func TestTwoIdenticalStatesDeriveNothing(t *testing.T) {
	w := newWorld(t)
	w.spawn(Pawn{Name: "Goblin", Visible: true})
	before := w.s.Clone()
	for _, role := range []Role{RoleGM, RolePlayer} {
		if got := Derive(&before, w.s, role); len(got) != 0 {
			t.Fatalf("a room that did not change told the %s %v", role, changeTypesOf(got))
		}
	}
}
func TestARefusedCommandDerivesNothing(t *testing.T) {
	w := newWorld(t)
	goblin := w.spawn(Pawn{Name: "Goblin", Visible: true})
	before := w.s.Clone()
	w.refuse(&PawnRemove{IDs: []ulid.ULID{goblin}}, w.pc, CodeForbidden)
	for _, role := range []Role{RoleGM, RolePlayer} {
		if got := Derive(&before, w.s, role); len(got) != 0 {
			t.Fatalf("a refused command told the %s %v", role, changeTypesOf(got))
		}
	}
}
func onePawn(t *testing.T, ch Change) Pawn {
	t.Helper()
	up, ok := ch.(*PawnsUpserted)
	if !ok {
		t.Fatalf("the change is a %T, not an upsert", ch)
	}
	if len(up.Pawns) != 1 {
		t.Fatalf("the upsert carries %d pawns, want one", len(up.Pawns))
	}
	return up.Pawns[0]
}
