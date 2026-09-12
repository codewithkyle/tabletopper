package room

import (
	"testing"

	"github.com/oklog/ulid/v2"
)

func TestRevealingAPawnSpawnsItForPlayersAndHidingItTakesItAway(t *testing.T) {
	w := newWorld(t)
	goblin := w.spawn(Pawn{Name: "Goblin", Visible: false, HP: intp(3), MaxHP: intp(7), AC: intp(15)})
	reveal := w.change(&PawnSetVisible{IDs: []ulid.ULID{goblin}, Visible: true}, w.gm)
	equalStrings(t, "the GM", eventTypesOf(reveal.events(RoleGM)), []string{"pawn.updated"})
	shown := reveal.events(RolePlayer)
	equalStrings(t, "the players", eventTypesOf(shown), []string{"pawn.spawned"})
	p := shown[0].(*PawnSpawned).Pawn
	if p.ID != goblin {
		t.Fatalf("the players were given pawn %s, want %s", p.ID, goblin)
	}
	if p.AC != nil {
		t.Fatal("the spawned copy carries the armour class the projection removes")
	}
	if p.HPBand == nil || *p.HPBand != BandBloody {
		t.Fatalf("the spawned copy's band is %v, want the projected bloody", Health(p))
	}
	hide := w.change(&PawnSetVisible{IDs: []ulid.ULID{goblin}, Visible: false}, w.gm)
	equalStrings(t, "the GM", eventTypesOf(hide.events(RoleGM)), []string{"pawn.updated"})
	gone := hide.events(RolePlayer)
	equalStrings(t, "the players", eventTypesOf(gone), []string{"pawn.removed"})
	if got := gone[0].(*PawnRemoved).ID; got != goblin {
		t.Fatalf("the players were told %s left, want %s", got, goblin)
	}
}
func TestChangingTheActiveLayerSwapsTheFloorUnderThePlayers(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	upstairs := w.spawn(Pawn{Name: "Upstairs", Visible: true})
	downstairs := w.spawn(Pawn{Name: "Downstairs", LayerID: cellar, Visible: true})
	w.spawn(Pawn{Name: "Hidden downstairs", LayerID: cellar, Visible: false})
	ch := w.change(&TableSetActiveLayer{Layer: cellar}, w.gm)
	equalStrings(t, "the GM", eventTypesOf(ch.events(RoleGM)), []string{"table.updated"})
	players := ch.events(RolePlayer)
	equalStrings(t, "the players", eventTypesOf(players), []string{"pawn.removed", "pawn.spawned", "table.updated"})
	if got := players[0].(*PawnRemoved).ID; got != upstairs {
		t.Fatal("the wrong pawn was taken off the players' table")
	}
	if got := players[1].(*PawnSpawned).Pawn.ID; got != downstairs {
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
	equalStrings(t, "the GM", eventTypesOf(ch.events(RoleGM)), []string{"table.updated"})
	players := ch.events(RolePlayer)
	equalStrings(t, "the players", eventTypesOf(players), []string{"pawn.updated", "pawn.updated", "table.updated"})
	for i, want := range []ulid.ULID{goblin, innkeeper} {
		p := players[i].(*PawnUpdated).Pawn
		if p.ID != want {
			t.Fatalf("the %d event carries pawn %s, want %s", i, p.ID, want)
		}
		if p.AC == nil {
			t.Fatalf("%s was reprojected without the armour class the change was about", p.Name)
		}
	}
	again := w.change(&TableSetOptions{PawnLabels: LabelsFull, PlayersCanDraw: false, InitiativeGrouping: GroupMonsters}, w.gm)
	equalStrings(t, "the players", eventTypesOf(again.events(RolePlayer)), []string{"table.updated"})
}
func TestAGroupMoveIsOnePawnMovedCarryingWhatTheRoleMaySee(t *testing.T) {
	w := newWorld(t)
	first := w.spawn(Pawn{Name: "Ari", X: 96, Y: 96, Visible: true})
	second := w.spawn(Pawn{Name: "Rin", X: 160, Y: 96, Visible: true})
	hidden := w.spawn(Pawn{Name: "Ambusher", X: 224, Y: 96, Visible: false})
	ch := w.change(&PawnMove{Anchor: first, X: 300, Y: 300, Others: []ulid.ULID{second, hidden}}, w.gm)
	gm := ch.events(RoleGM)
	equalStrings(t, "the GM", eventTypesOf(gm), []string{"pawn.moved"})
	if got := len(gm[0].(*PawnMoved).Pawns); got != 3 {
		t.Fatalf("the GM's move carries %d positions, want 3", got)
	}
	players := ch.events(RolePlayer)
	equalStrings(t, "the players", eventTypesOf(players), []string{"pawn.moved"})
	at := players[0].(*PawnMoved).Pawns
	if len(at) != 2 || at[0].ID != first || at[1].ID != second {
		t.Fatalf("the players' move carries %v, want the two they can see", at)
	}
	alone := w.spawn(Pawn{Name: "Second ambusher", X: 288, Y: 96, Visible: false})
	dark := w.change(&PawnMove{Anchor: hidden, X: 480, Y: 480, Others: []ulid.ULID{alone}}, w.gm)
	equalStrings(t, "the GM", eventTypesOf(dark.events(RoleGM)), []string{"pawn.moved"})
	if got := dark.events(RolePlayer); len(got) != 0 {
		t.Fatalf("a move of nothing but hidden pawns told the players %v", eventTypesOf(got))
	}
}
func TestHidingATrackedPawnDropsItFromThePlayersTrackerAlone(t *testing.T) {
	w := newWorld(t)
	goblin := w.spawn(Pawn{Name: "Goblin", Visible: true})
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}, Initiative: 12}}}, w.gm)
	ch := w.change(&PawnSetVisible{IDs: []ulid.ULID{goblin}, Visible: false}, w.gm)
	equalStrings(t, "the GM", eventTypesOf(ch.events(RoleGM)), []string{"pawn.updated"})
	players := ch.events(RolePlayer)
	equalStrings(t, "the players", eventTypesOf(players), []string{"pawn.removed", "initiative.updated"})
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
	equalStrings(t, "beginning", eventTypesOf(begun.events(RoleGM)), []string{"stroke.began"})
	grown := w.change(&StrokeExtend{ID: id, Points: []int{20, 20, 30, 30}}, w.gm)
	evs := grown.events(RoleGM)
	equalStrings(t, "extending", eventTypesOf(evs), []string{"stroke.extended"})
	ext := evs[0].(*StrokeExtended)
	if ext.ID != id || len(ext.Points) != 4 || ext.Points[0] != 20 {
		t.Fatalf("the extension carries %v, want the four new numbers", ext.Points)
	}
	ended := w.change(&StrokeEnd{ID: id}, w.gm)
	equalStrings(t, "ending", eventTypesOf(ended.events(RoleGM)), []string{"stroke.ended"})
	before := w.s.Clone()
	rewritten := w.s.Stroke(id)
	rewritten.Points = []int{99, 99}
	rewritten.Done = false
	evs = Derive(&before, w.s, RoleGM)
	equalStrings(t, "rewriting", eventTypesOf(evs), []string{"stroke.began"})
	if got := evs[0].(*StrokeBegan).Stroke.Points; len(got) != 2 || got[0] != 99 {
		t.Fatalf("the rewrite carries %v, want the whole stroke", got)
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
	ch := w.change(&TableRemoveLayer{Layer: cellar}, w.gm)
	gm := ch.events(RoleGM)
	equalStrings(t, "the GM", eventTypesOf(gm), []string{
		"fog.removed",
		"fog.removed",
		"stroke.erased",
		"pawn.removed",
		"initiative.updated",
		"table.updated",
	})
	if got := gm[2].(*StrokeErased).IDs; len(got) != 2 {
		t.Fatalf("the erasure names %d strokes, want both in one event", len(got))
	}
}
func TestTwoIdenticalStatesDeriveNothing(t *testing.T) {
	w := newWorld(t)
	w.spawn(Pawn{Name: "Goblin", Visible: true})
	before := w.s.Clone()
	for _, role := range []Role{RoleGM, RolePlayer} {
		if got := Derive(&before, w.s, role); len(got) != 0 {
			t.Fatalf("a room that did not change told the %s %v", role, eventTypesOf(got))
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
			t.Fatalf("a refused command told the %s %v", role, eventTypesOf(got))
		}
	}
}
