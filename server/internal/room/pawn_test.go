package room

import (
	"slices"
	"testing"

	"github.com/oklog/ulid/v2"
)

func TestAGroupMoveKeepsEveryOffsetToThePixel(t *testing.T) {
	w := newWorld(t)
	loose := w.s.Table.Grid
	loose.Snap = SnapOff
	w.apply(&TableSetGrid{Grid: loose}, w.gm)
	wagon := w.spawn(Pawn{Kind: PawnObject, Name: "Wagon", Width: 128, Height: 128, X: 128, Y: 128, Visible: true})
	riders := []ulid.ULID{
		w.spawn(Pawn{Name: "Ari", X: 150, Y: 140, Visible: true}),
		w.spawn(Pawn{Name: "Rin", X: 170, Y: 100, Visible: true}),
		w.spawn(Pawn{Name: "Sam", X: 110, Y: 175, Visible: true}),
	}
	snapped := w.s.Table.Grid
	snapped.Snap = SnapCells
	w.apply(&TableSetGrid{Grid: snapped}, w.gm)
	offsets := make(map[ulid.ULID][2]int, len(riders))
	anchorBefore := *w.s.Pawn(wagon)
	for _, id := range riders {
		p := w.s.Pawn(id)
		offsets[id] = [2]int{p.X - anchorBefore.X, p.Y - anchorBefore.Y}
	}
	ch := w.change(&PawnMove{Anchor: wagon, X: 300, Y: 300, Others: riders}, w.gm)
	after := w.s.Pawn(wagon)
	if after.X != 300 || after.Y != 300 {
		t.Fatalf("the anchor landed at (%d, %d), want the raw (300, 300)", after.X, after.Y)
	}
	for _, id := range riders {
		p := w.s.Pawn(id)
		want := offsets[id]
		if p.X-after.X != want[0] || p.Y-after.Y != want[1] {
			t.Fatalf("%s sat at offset %v and is now at %v", p.Name, want, [2]int{p.X - after.X, p.Y - after.Y})
		}
		if (p.X-32)%64 == 0 || (p.Y-32)%64 == 0 {
			t.Fatalf("%s was re-snapped to a cell centre at (%d, %d)", p.Name, p.X, p.Y)
		}
	}
	evs := ch.events(RoleGM)
	equalStrings(t, "the GM", eventTypesOf(evs), []string{"pawn.moved"})
	moved := evs[0].(*PawnMoved)
	if len(moved.Pawns) != 4 {
		t.Fatalf("the move carried %d positions, want 4", len(moved.Pawns))
	}
	for _, id := range append([]ulid.ULID{wagon}, riders...) {
		if !slices.ContainsFunc(moved.Pawns, func(at PawnPosition) bool { return at.ID == id }) {
			t.Fatalf("the move left %s out of the position list", id)
		}
	}
	rider := w.s.Pawn(riders[0])
	offset := [2]int{after.X - rider.X, after.Y - rider.Y}
	w.apply(&PawnMove{Anchor: riders[0], X: 500, Y: 500, Others: []ulid.ULID{wagon}}, w.gm)
	rider = w.s.Pawn(riders[0])
	if rider.X != 480 || rider.Y != 480 {
		t.Fatalf("the creature anchor landed at (%d, %d), want the centre at (480, 480)", rider.X, rider.Y)
	}
	carried := w.s.Pawn(wagon)
	if carried.X-rider.X != offset[0] || carried.Y-rider.Y != offset[1] {
		t.Fatalf("the wagon sat at offset %v and is now at %v", offset, [2]int{carried.X - rider.X, carried.Y - rider.Y})
	}
}
func TestAPlayerCannotSmuggleAMonsterIntoTheirSelection(t *testing.T) {
	w := newWorld(t)
	mine := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", X: 96, Y: 96, Visible: true, OwnerID: &testPlayerID})
	goblin := w.spawn(Pawn{Name: "Goblin", X: 160, Y: 96, Visible: true})
	w.refuse(&PawnMove{Anchor: mine, X: 300, Y: 300, Others: []ulid.ULID{goblin}}, w.pc, CodeForbidden)
	if p := w.s.Pawn(mine); p.X != 96 || p.Y != 96 {
		t.Fatalf("the player's own pawn moved to (%d, %d) despite the refusal", p.X, p.Y)
	}
	if p := w.s.Pawn(goblin); p.X != 160 || p.Y != 96 {
		t.Fatalf("the goblin moved to (%d, %d) despite the refusal", p.X, p.Y)
	}
}
func TestAGroupMoveThatWouldLeaveTheMapMovesNobody(t *testing.T) {
	w := newWorld(t)
	anchor := w.spawn(Pawn{Name: "Anchor", X: 96, Y: 96, Visible: true})
	far := w.spawn(Pawn{Name: "Far", X: CoordLimit - 64, Y: 96, Visible: true})
	w.refuse(&PawnMove{Anchor: anchor, X: 100_000, Y: 96, Others: []ulid.ULID{far}}, w.gm, CodeInvalid)
	if p := w.s.Pawn(anchor); p.X != 96 {
		t.Fatalf("the anchor moved to %d despite the refusal", p.X)
	}
}
func TestADragPreviewsWithoutChangingAnything(t *testing.T) {
	w := newWorld(t)
	pawn := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", X: 96, Y: 96, Visible: true, OwnerID: &testPlayerID})
	before := mustJSON(t, w.s)
	ch := w.change(&PawnDrag{Anchor: pawn, X: 300, Y: 300}, w.pc)
	if got := mustJSON(t, w.s); got != before {
		t.Fatal("a drag changed the state")
	}
	equalStrings(t, "signals", summary(ch.signals), []string{"pawn.dragging to others"})
	if got := ch.events(RoleGM); len(got) != 0 {
		t.Fatalf("a drag derived %v; it changes nothing", eventTypesOf(got))
	}
	ghost := ch.signals[0].Event.(*PawnDragging)
	if ghost.Pawns[0].X != 300 || ghost.Pawns[0].Y != 300 {
		t.Fatalf("the ghost was snapped to (%d, %d); a drag preview is never snapped", ghost.Pawns[0].X, ghost.Pawns[0].Y)
	}
	if got := ch.seen(w.pc); len(got) != 0 {
		t.Fatalf("the dragging player received their own ghost: %v", eventTypesOf(got))
	}
	if got := ch.seen(w.gm); len(got) != 1 {
		t.Fatalf("the GM received %v, want the ghost", eventTypesOf(got))
	}
	if got := ch.seen(w.other); len(got) != 1 {
		t.Fatalf("the other player received %v, want the ghost", eventTypesOf(got))
	}
}
func TestADragOfAHiddenPawnReachesTheGMAlone(t *testing.T) {
	w := newWorld(t)
	hidden := w.spawn(Pawn{Name: "Ambusher", X: 96, Y: 96, Visible: false})
	ch := w.change(&PawnDrag{Anchor: hidden, X: 300, Y: 300}, w.gm)
	if got := ProjectSignal(w.s, ch.signals[0], RolePlayer); got != nil {
		t.Fatalf("a drag of nothing but hidden pawns produced a players' copy: %#v", got)
	}
	if got := ProjectSignal(w.s, ch.signals[0], RoleGM); got == nil {
		t.Fatal("the GM's copy of a hidden drag went missing")
	}
}
func TestObjectsAreRectanglesWithoutConditions(t *testing.T) {
	w := newWorld(t)
	wagon := w.spawn(Pawn{Kind: PawnObject, Name: "Wagon", Width: 128, Height: 256, Visible: true})
	p := w.s.Pawn(wagon)
	if p.Size != "" {
		t.Fatalf("the object was given the size %q", p.Size)
	}
	if p.HP != nil || p.MaxHP != nil {
		t.Fatal("the object was given hit points it was not asked for")
	}
	if p.Width != 128 || p.Height != 256 {
		t.Fatalf("the object is %dx%d pixels, want 128x256", p.Width, p.Height)
	}
	w.refuse(&PawnSetConditions{ID: wagon, Conditions: []Condition{
		{Name: "Poisoned", Color: ColorGreen, Duration: -1, Clear: ClearEnd},
	}}, w.gm, CodeInvalid)
	w.refuse(&PawnUpdate{ID: wagon, Size: sizep(SizeLarge)}, w.gm, CodeInvalid)
	goblin := w.spawn(Pawn{Name: "Goblin", Visible: true})
	w.refuse(&PawnUpdate{ID: goblin, Width: intp(192)}, w.gm, CodeInvalid)
	w.refuse(&PawnUpdate{ID: goblin, Rotation: intp(90)}, w.gm, CodeInvalid)
}
func TestAnObjectTurnsAboutItsCentre(t *testing.T) {
	w := newWorld(t)
	wagon := w.spawn(Pawn{Kind: PawnObject, Name: "Wagon", Width: 128, Height: 256, Visible: true})
	if got := w.s.Pawn(wagon).Rotation; got != 0 {
		t.Fatalf("a spawned object starts at %d degrees, want 0", got)
	}
	before := *w.s.Pawn(wagon)
	w.apply(&PawnUpdate{ID: wagon, Rotation: intp(-90)}, w.gm)
	after := w.s.Pawn(wagon)
	if after.Rotation != 270 {
		t.Fatalf("the wagon is at %d degrees, want 270", after.Rotation)
	}
	if after.X != before.X || after.Y != before.Y {
		t.Fatalf("turning moved the wagon from %d,%d to %d,%d", before.X, before.Y, after.X, after.Y)
	}
	if after.Width != before.Width || after.Height != before.Height {
		t.Fatalf("turning resized the wagon to %dx%d", after.Width, after.Height)
	}
}
func TestHidingAPawnTellsEachAudienceSomethingDifferent(t *testing.T) {
	w := newWorld(t)
	goblin := w.spawn(Pawn{Name: "Goblin", X: 96, Y: 96, Visible: true})
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}, Initiative: 12}}}, w.gm)
	hide := w.change(&PawnSetVisible{IDs: []ulid.ULID{goblin}, Visible: false}, w.gm)
	equalStrings(t, "hiding, for the GM", eventTypesOf(hide.events(RoleGM)), []string{"pawn.updated"})
	players := hide.events(RolePlayer)
	equalStrings(t, "hiding, for a player", eventTypesOf(players), []string{"pawn.removed", "initiative.updated"})
	if tracker := players[1].(*InitiativeUpdated); len(tracker.Initiative.Entries) != 0 {
		t.Fatalf("the players' tracker still names the hidden pawn: %+v", tracker.Initiative.Entries)
	}
	reveal := w.change(&PawnSetVisible{IDs: []ulid.ULID{goblin}, Visible: true}, w.gm)
	equalStrings(t, "revealing, for the GM", eventTypesOf(reveal.events(RoleGM)), []string{"pawn.updated"})
	equalStrings(t, "revealing, for a player", eventTypesOf(reveal.events(RolePlayer)), []string{"pawn.spawned", "initiative.updated"})
	again := w.change(&PawnSetVisible{IDs: []ulid.ULID{goblin}, Visible: true}, w.gm)
	if got := again.events(RoleGM); len(got) != 0 {
		t.Fatalf("revealing a pawn that was already shown told the GM %v", eventTypesOf(got))
	}
	if got := again.events(RolePlayer); len(got) != 0 {
		t.Fatalf("revealing a pawn that was already shown told the players %v", eventTypesOf(got))
	}
}
func TestHidingAPawnOnAnotherFloorStillCorrectsThePlayersTracker(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	goblin := w.spawn(Pawn{Name: "Goblin", LayerID: cellar, Visible: true})
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}, Initiative: 12}}}, w.gm)
	hide := w.change(&PawnSetVisible{IDs: []ulid.ULID{goblin}, Visible: false}, w.gm)
	equalStrings(t, "hiding a pawn downstairs, for the GM", eventTypesOf(hide.events(RoleGM)), []string{"pawn.updated"})
	players := hide.events(RolePlayer)
	equalStrings(t, "hiding a pawn downstairs, for a player", eventTypesOf(players), []string{"initiative.updated"})
	tracker, ok := players[len(players)-1].(*InitiativeUpdated)
	if !ok {
		t.Fatalf("the last event a player received was %T", players[len(players)-1])
	}
	if len(tracker.Initiative.Entries) != 0 {
		t.Fatalf("the players' tracker still names the hidden pawn: %+v", tracker.Initiative.Entries)
	}
	if len(w.s.Initiative.Entries) != 1 {
		t.Fatalf("the GM's tracker holds %d entries, want 1", len(w.s.Initiative.Entries))
	}
}
func TestMovingAPawnBetweenFloorsLeavesTheTrackerAlone(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	goblin := w.spawn(Pawn{Name: "Goblin", Visible: true})
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}, Initiative: 12}}}, w.gm)
	ch := w.change(&PawnSetLayer{IDs: []ulid.ULID{goblin}, Layer: cellar}, w.gm)
	equalStrings(t, "the GM", eventTypesOf(ch.events(RoleGM)), []string{"pawn.updated"})
	equalStrings(t, "the players", eventTypesOf(ch.events(RolePlayer)), []string{"pawn.removed"})
	if got := len(projectInitiative(w.s).Entries); got != 1 {
		t.Fatalf("the players' tracker holds %d entries after a floor move, want 1", got)
	}
}
func TestHidingASelectionIsOneCommandAndOneTrackerEvent(t *testing.T) {
	w := newWorld(t)
	first := w.spawn(Pawn{Name: "First goblin", X: 96, Y: 96, Visible: true})
	second := w.spawn(Pawn{Name: "Second goblin", X: 160, Y: 96, Visible: true})
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "First goblin", PawnIDs: []ulid.ULID{first}, Initiative: 12},
		{Name: "Second goblin", PawnIDs: []ulid.ULID{second}, Initiative: 11},
	}}, w.gm)
	hide := w.change(&PawnSetVisible{IDs: []ulid.ULID{first, second}, Visible: false}, w.gm)
	equalStrings(t, "hiding two, for the GM", eventTypesOf(hide.events(RoleGM)), []string{
		"pawn.updated",
		"pawn.updated",
	})
	players := hide.events(RolePlayer)
	equalStrings(t, "hiding two, for a player", eventTypesOf(players), []string{
		"pawn.removed",
		"pawn.removed",
		"initiative.updated",
	})
	if tracker := players[len(players)-1].(*InitiativeUpdated); len(tracker.Initiative.Entries) != 0 {
		t.Fatalf("the players' tracker still names a hidden pawn: %+v", tracker.Initiative.Entries)
	}
	reveal := w.change(&PawnSetVisible{IDs: []ulid.ULID{first, second}, Visible: true}, w.gm)
	equalStrings(t, "revealing two, for the GM", eventTypesOf(reveal.events(RoleGM)), []string{
		"pawn.updated",
		"pawn.updated",
	})
	equalStrings(t, "revealing two, for a player", eventTypesOf(reveal.events(RolePlayer)), []string{
		"pawn.spawned",
		"pawn.spawned",
		"initiative.updated",
	})
}
func TestHidingAMixedSelectionSetsRatherThanToggles(t *testing.T) {
	w := newWorld(t)
	seen := w.spawn(Pawn{Name: "Goblin", X: 96, Y: 96, Visible: true})
	hidden := w.spawn(Pawn{Name: "Ambusher", X: 160, Y: 96, Visible: false})
	ch := w.change(&PawnSetVisible{IDs: []ulid.ULID{seen, hidden}, Visible: false}, w.gm)
	equalStrings(t, "hiding a mixed selection, for the GM", eventTypesOf(ch.events(RoleGM)), []string{"pawn.updated"})
	equalStrings(t, "hiding a mixed selection, for a player", eventTypesOf(ch.events(RolePlayer)), []string{"pawn.removed"})
	if w.s.Pawn(seen).Visible || w.s.Pawn(hidden).Visible {
		t.Fatal("a pawn is still visible after the whole selection was hidden")
	}
}
func TestHidingRefusesAnUnknownPawnBeforeChangingAnything(t *testing.T) {
	w := newWorld(t)
	goblin := w.spawn(Pawn{Name: "Goblin", Visible: true})
	w.refuse(&PawnSetVisible{IDs: []ulid.ULID{goblin, testID(999)}, Visible: false}, w.gm, CodeNotFound)
	if !w.s.Pawn(goblin).Visible {
		t.Fatal("the goblin was hidden by a command that was refused")
	}
}
func TestTheSpawnedPartyIsVisible(t *testing.T) {
	w := newWorld(t)
	ch := w.change(&PawnSpawnCharacters{Pawns: []Pawn{
		{Name: "Ari", Size: SizeMedium, LayerID: w.layer, X: 96, Y: 96, OwnerID: &testPlayerID, CharacterID: &testCharID},
		{Name: "Rin", Size: SizeMedium, LayerID: w.layer, X: 160, Y: 96, OwnerID: &testOtherID, CharacterID: &testOtherChar},
	}}, w.gm)
	for _, role := range []Role{RoleGM, RolePlayer} {
		equalStrings(t, "spawning the party for the "+string(role), eventTypesOf(ch.events(role)), []string{
			"pawn.spawned",
			"pawn.spawned",
		})
	}
	for _, p := range w.s.Pawns {
		if !p.Visible {
			t.Fatalf("%s was spawned hidden", p.Name)
		}
	}
}
func TestRemovingPawnsEmitsOneTrackerEvent(t *testing.T) {
	w := newWorld(t)
	first := w.spawn(Pawn{Name: "Goblin", Visible: true})
	second := w.spawn(Pawn{Name: "Goblin", Visible: true})
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "First", PawnIDs: []ulid.ULID{first}},
		{Name: "Second", PawnIDs: []ulid.ULID{second}},
	}}, w.gm)
	ch := w.change(&PawnRemove{IDs: []ulid.ULID{first, second}}, w.gm)
	for _, role := range []Role{RoleGM, RolePlayer} {
		equalStrings(t, "removing two for the "+string(role), eventTypesOf(ch.events(role)), []string{
			"pawn.removed",
			"pawn.removed",
			"initiative.updated",
		})
	}
	if len(w.s.Initiative.Entries) != 0 {
		t.Fatalf("the tracker still holds %d entries", len(w.s.Initiative.Entries))
	}
	if w.s.Initiative.Active != nil {
		t.Fatal("the tracker still points at a turn")
	}
}
func TestAMissingPawnIsNotFound(t *testing.T) {
	w := newWorld(t)
	w.refuse(&PawnUpdate{ID: testID(999)}, w.gm, CodeNotFound)
	w.refuse(&PawnMove{Anchor: testID(999)}, w.pc, CodeNotFound)
	w.refuse(&PawnRemove{IDs: []ulid.ULID{testID(999)}}, w.gm, CodeNotFound)
}
func TestASpawnedPawnLandsOnTop(t *testing.T) {
	w := newWorld(t)
	first := w.spawn(Pawn{Name: "First", Visible: true})
	second := w.spawn(Pawn{Name: "Second", Visible: true})
	if w.s.Pawn(second).Z <= w.s.Pawn(first).Z {
		t.Fatalf("the second pawn's z is %d, not above the first's %d", w.s.Pawn(second).Z, w.s.Pawn(first).Z)
	}
}
func sizep(s Size) *Size { return &s }
