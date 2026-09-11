package room

import (
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
	ems := w.apply(&PawnMove{Anchor: wagon, X: 300, Y: 300, Others: riders}, w.gm)
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
	equalStrings(t, "emissions", summary(ems), []string{"pawn.moved to all"})
	moved := ems[0].Event.(*PawnMoved)
	if len(moved.Pawns) != 4 {
		t.Fatalf("the move carried %d positions, want 4", len(moved.Pawns))
	}
	if moved.Pawns[0].ID != wagon {
		t.Fatal("the anchor is not first in the position list")
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
func TestTheMoveEventDropsWhatPlayersCannotSee(t *testing.T) {
	w := newWorld(t)
	seen := w.spawn(Pawn{Name: "Ari", X: 96, Y: 96, Visible: true})
	hidden := w.spawn(Pawn{Name: "Ambusher", X: 160, Y: 96, Visible: false})
	ems := w.apply(&PawnMove{Anchor: seen, X: 300, Y: 300, Others: []ulid.ULID{hidden}}, w.gm)
	ev := ems[0].Event
	gm := ForRole(ev, RoleGM).(*PawnMoved)
	if len(gm.Pawns) != 2 {
		t.Fatalf("the GM's copy carries %d positions, want both", len(gm.Pawns))
	}
	players := ForRole(ev, RolePlayer).(*PawnMoved)
	if len(players.Pawns) != 1 || players.Pawns[0].ID != seen {
		t.Fatalf("the players' copy carries %v, want only the visible pawn", players.Pawns)
	}
	other := w.spawn(Pawn{Name: "Second ambusher", X: 224, Y: 96, Visible: false})
	ems = w.apply(&PawnMove{Anchor: hidden, X: 480, Y: 480, Others: []ulid.ULID{other}}, w.gm)
	if got := ForRole(ems[0].Event, RolePlayer); got != nil {
		t.Fatalf("a move of nothing but hidden pawns produced a players' copy: %#v", got)
	}
	if got := ForRole(ems[0].Event, RoleGM); got == nil {
		t.Fatal("the GM's copy of a hidden move went missing")
	}
}
func TestADragPreviewsWithoutChangingAnything(t *testing.T) {
	w := newWorld(t)
	pawn := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", X: 96, Y: 96, Visible: true, OwnerID: &testPlayerID})
	before := mustJSON(t, w.s)
	ems := w.apply(&PawnDrag{Anchor: pawn, X: 300, Y: 300}, w.pc)
	if got := mustJSON(t, w.s); got != before {
		t.Fatal("a drag changed the state")
	}
	equalStrings(t, "emissions", summary(ems), []string{"pawn.dragging to others"})
	ghost := ems[0].Event.(*PawnDragging)
	if ghost.Pawns[0].X != 300 || ghost.Pawns[0].Y != 300 {
		t.Fatalf("the ghost was snapped to (%d, %d); a drag preview is never snapped", ghost.Pawns[0].X, ghost.Pawns[0].Y)
	}
	if !ghost.Transient() {
		t.Fatal("a drag preview is not marked transient, so it would be reduced into state")
	}
	if got := delivered(ems, w.pc, w.pc); len(got) != 0 {
		t.Fatalf("the dragging player received their own ghost: %v", eventTypesOf(got))
	}
	if got := delivered(ems, w.pc, w.gm); len(got) != 1 {
		t.Fatalf("the GM received %v, want the ghost", eventTypesOf(got))
	}
	if got := delivered(ems, w.pc, w.other); len(got) != 1 {
		t.Fatalf("the other player received %v, want the ghost", eventTypesOf(got))
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
	hide := w.apply(&PawnSetVisible{IDs: []ulid.ULID{goblin}, Visible: false}, w.gm)
	equalStrings(t, "hiding", summary(hide), []string{
		"pawn.updated to gm",
		"pawn.removed to players",
		"initiative.updated to players",
	})
	if got := eventTypesOf(delivered(hide, w.gm, w.gm)); len(got) != 1 || got[0] != "pawn.updated" {
		t.Fatalf("the GM received %v while hiding a pawn", got)
	}
	players := delivered(hide, w.gm, w.pc)
	equalStrings(t, "what a player received", eventTypesOf(players), []string{"pawn.removed", "initiative.updated"})
	if tracker := players[1].(*InitiativeUpdated); len(tracker.Initiative.Entries) != 0 {
		t.Fatalf("the players' tracker still names the hidden pawn: %+v", tracker.Initiative.Entries)
	}
	reveal := w.apply(&PawnSetVisible{IDs: []ulid.ULID{goblin}, Visible: true}, w.gm)
	equalStrings(t, "revealing", summary(reveal), []string{
		"pawn.updated to gm",
		"pawn.spawned to players",
		"initiative.updated to players",
	})
	again := w.apply(&PawnSetVisible{IDs: []ulid.ULID{goblin}, Visible: true}, w.gm)
	equalStrings(t, "revealing twice", summary(again), []string{"pawn.updated to gm"})
}
func TestHidingAPawnOnAnotherFloorStillCorrectsThePlayersTracker(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	goblin := w.spawn(Pawn{Name: "Goblin", LayerID: cellar, Visible: true})
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}, Initiative: 12}}}, w.gm)
	hide := w.apply(&PawnSetVisible{IDs: []ulid.ULID{goblin}, Visible: false}, w.gm)
	equalStrings(t, "hiding a pawn downstairs", summary(hide), []string{
		"pawn.updated to gm",
		"initiative.updated to players",
	})
	players := delivered(hide, w.gm, w.pc)
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
	ems := w.apply(&PawnSetLayer{IDs: []ulid.ULID{goblin}, Layer: cellar}, w.gm)
	equalStrings(t, "sending a tracked pawn downstairs", summary(ems), []string{
		"pawn.updated to gm",
		"pawn.removed to players",
	})
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
	hide := w.apply(&PawnSetVisible{IDs: []ulid.ULID{first, second}, Visible: false}, w.gm)
	equalStrings(t, "hiding two", summary(hide), []string{
		"pawn.updated to gm",
		"pawn.updated to gm",
		"pawn.removed to players",
		"pawn.removed to players",
		"initiative.updated to players",
	})
	players := delivered(hide, w.gm, w.pc)
	if tracker := players[len(players)-1].(*InitiativeUpdated); len(tracker.Initiative.Entries) != 0 {
		t.Fatalf("the players' tracker still names a hidden pawn: %+v", tracker.Initiative.Entries)
	}
	reveal := w.apply(&PawnSetVisible{IDs: []ulid.ULID{first, second}, Visible: true}, w.gm)
	equalStrings(t, "revealing two", summary(reveal), []string{
		"pawn.updated to gm",
		"pawn.updated to gm",
		"pawn.spawned to players",
		"pawn.spawned to players",
		"initiative.updated to players",
	})
}
func TestHidingAMixedSelectionSetsRatherThanToggles(t *testing.T) {
	w := newWorld(t)
	seen := w.spawn(Pawn{Name: "Goblin", X: 96, Y: 96, Visible: true})
	hidden := w.spawn(Pawn{Name: "Ambusher", X: 160, Y: 96, Visible: false})
	ems := w.apply(&PawnSetVisible{IDs: []ulid.ULID{seen, hidden}, Visible: false}, w.gm)
	equalStrings(t, "hiding a mixed selection", summary(ems), []string{
		"pawn.updated to gm",
		"pawn.updated to gm",
		"pawn.removed to players",
	})
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
	ems := w.apply(&PawnSpawnCharacters{Pawns: []Pawn{
		{Name: "Ari", Size: SizeMedium, LayerID: w.layer, X: 96, Y: 96, OwnerID: &testPlayerID, CharacterID: &testCharID},
		{Name: "Rin", Size: SizeMedium, LayerID: w.layer, X: 160, Y: 96, OwnerID: &testOtherID, CharacterID: &testOtherChar},
	}}, w.gm)
	equalStrings(t, "spawning the party", summary(ems), []string{
		"pawn.spawned to gm",
		"pawn.spawned to players",
		"pawn.spawned to gm",
		"pawn.spawned to players",
	})
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
	ems := w.apply(&PawnRemove{IDs: []ulid.ULID{first, second}}, w.gm)
	equalStrings(t, "emissions", summary(ems), []string{
		"pawn.removed to gm",
		"pawn.removed to players",
		"pawn.removed to gm",
		"pawn.removed to players",
		"initiative.updated to all",
	})
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
