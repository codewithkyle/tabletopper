package room

import (
	"testing"

	"github.com/oklog/ulid/v2"
)

// THE WAGON. A group move is one delta taken from the anchor and applied to
// everything else, and the property that matters is that the riders arrive
// sitting exactly where they were sitting -- not snapped into the wagon's cells,
// not each snapped independently, which would shuffle them.
//
// The positions here are chosen to already be snapped, so that the only
// movement in the test is the movement the command causes.
func TestAGroupMoveKeepsEveryOffsetToThePixel(t *testing.T) {
	w := newWorld(t)

	// THE RIDERS ARE PLACED OFF THE GRID ON PURPOSE, with snapping switched off
	// while they are put down. It is the only arrangement that can tell the two
	// implementations apart: when everything already sits on its own lattice,
	// snapping each rider independently and moving them all by the anchor's
	// delta produce the same answer, and the test would pass either way.
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

	// AN OBJECT ANCHOR LANDS ON THE RAW NUMBER, because an object is not on the
	// lattice at all: see snapPawn. Snapping is on, and 300 is neither a cell
	// centre nor a vertex.
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

		// And none of them landed on a cell centre, which is where snapping
		// each rider independently would have put them.
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

	// AND THE SAME PROPERTY WITH THE SNAP THE OTHER WAY ROUND. Ari is a medium
	// creature, so she DOES land on a cell centre -- and the wagon she was
	// sitting on rides along by her delta rather than being snapped to one.
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

// A player may move what they own. Naming somebody else's pawn in the same
// selection is refused, and the refusal happens before anything moves -- a
// group move is all or nothing, including its authority check.
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

// A group that would push one member off the map moves nobody. The check runs
// over every computed position before the first pawn is written.
func TestAGroupMoveThatWouldLeaveTheMapMovesNobody(t *testing.T) {
	w := newWorld(t)

	anchor := w.spawn(Pawn{Name: "Anchor", X: 96, Y: 96, Visible: true})
	far := w.spawn(Pawn{Name: "Far", X: CoordLimit - 64, Y: 96, Visible: true})

	w.refuse(&PawnMove{Anchor: anchor, X: 100_000, Y: 96, Others: []ulid.ULID{far}}, w.gm, CodeInvalid)

	if p := w.s.Pawn(anchor); p.X != 96 {
		t.Fatalf("the anchor moved to %d despite the refusal", p.X)
	}
}

// THE PLAYER COPY OF A MOVE. A hidden rider is not in it, and when nothing in
// the move is visible there is no player copy at all -- ForRole answers nil
// rather than an empty list, so the hub sends nothing.
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

	// And with nothing visible in the move, players are told nothing.
	other := w.spawn(Pawn{Name: "Second ambusher", X: 224, Y: 96, Visible: false})
	ems = w.apply(&PawnMove{Anchor: hidden, X: 480, Y: 480, Others: []ulid.ULID{other}}, w.gm)

	if got := ForRole(ems[0].Event, RolePlayer); got != nil {
		t.Fatalf("a move of nothing but hidden pawns produced a players' copy: %#v", got)
	}
	if got := ForRole(ems[0].Event, RoleGM); got == nil {
		t.Fatal("the GM's copy of a hidden move went missing")
	}
}

// A drag changes nothing, is never snapped, and goes to everybody except the
// person whose hand is on the mouse.
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

	// The dragging player receives nothing; everybody else does.
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

// OBJECTS ARE NOT CREATURES. They are measured in pixels instead of by a
// creature size, they cannot be poisoned, and they may have no hit points at
// all -- a door usually does not.
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

	// The two edits cross over: a creature size on an object and a width on a
	// creature are both the client having confused one for the other.
	w.refuse(&PawnUpdate{ID: wagon, Size: sizep(SizeLarge)}, w.gm, CodeInvalid)

	goblin := w.spawn(Pawn{Name: "Goblin", Visible: true})
	w.refuse(&PawnUpdate{ID: goblin, Width: intp(192)}, w.gm, CodeInvalid)

	// And so is an angle on one. A disc has no facing, so a rotation on a
	// creature is the client having sent an object's field to a monster.
	w.refuse(&PawnUpdate{ID: goblin, Rotation: intp(90)}, w.gm, CodeInvalid)
}

// AN OBJECT TURNS AND A CREATURE DOES NOT, and the angle that comes back is
// always the reduced one -- a client that dragged the handle three times round
// leaves the wagon where a client that dragged it once did.
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

// THE VISIBILITY TRANSITIONS, which are the whole reason for the two-audience
// design. Each direction emits exactly one set of events to exactly one set of
// audiences, and the tracker rides along because hiding a creature also takes
// its line out of the players' turn order.
func TestHidingAPawnTellsEachAudienceSomethingDifferent(t *testing.T) {
	w := newWorld(t)

	goblin := w.spawn(Pawn{Name: "Goblin", X: 96, Y: 96, Visible: true})
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{{Name: "Goblin", PawnID: &goblin, Initiative: 12}}}, w.gm)

	hide := w.apply(&PawnSetVisible{ID: goblin, Visible: false}, w.gm)
	equalStrings(t, "hiding", summary(hide), []string{
		"pawn.updated to gm",
		"pawn.removed to players",
		"initiative.updated to players",
	})

	// The GM's tracker did not change, so the GM is told nothing about it.
	if got := eventTypesOf(delivered(hide, w.gm, w.gm)); len(got) != 1 || got[0] != "pawn.updated" {
		t.Fatalf("the GM received %v while hiding a pawn", got)
	}

	players := delivered(hide, w.gm, w.pc)
	equalStrings(t, "what a player received", eventTypesOf(players), []string{"pawn.removed", "initiative.updated"})
	if tracker := players[1].(*InitiativeUpdated); len(tracker.Initiative.Entries) != 0 {
		t.Fatalf("the players' tracker still names the hidden pawn: %+v", tracker.Initiative.Entries)
	}

	reveal := w.apply(&PawnSetVisible{ID: goblin, Visible: true}, w.gm)
	equalStrings(t, "revealing", summary(reveal), []string{
		"pawn.updated to gm",
		"pawn.spawned to players",
		"initiative.updated to players",
	})

	// Setting it to what it already is changes nothing for players.
	again := w.apply(&PawnSetVisible{ID: goblin, Visible: true}, w.gm)
	equalStrings(t, "revealing twice", summary(again), []string{"pawn.updated to gm"})
}

// Deleting a pawn takes its line out of the tracker, and one tracker event
// covers the whole command however many pawns went with it.
func TestRemovingPawnsEmitsOneTrackerEvent(t *testing.T) {
	w := newWorld(t)

	first := w.spawn(Pawn{Name: "Goblin", Visible: true})
	second := w.spawn(Pawn{Name: "Goblin", Visible: true})
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "First", PawnID: &first},
		{Name: "Second", PawnID: &second},
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

// A pawn that is gone is not_found rather than forbidden, for both roles. Two
// people deleting the same goblin is an ordinary race, and telling a player
// their own pawn belongs to somebody else would be false.
func TestAMissingPawnIsNotFound(t *testing.T) {
	w := newWorld(t)

	w.refuse(&PawnUpdate{ID: testID(999)}, w.gm, CodeNotFound)
	w.refuse(&PawnMove{Anchor: testID(999)}, w.pc, CodeNotFound)
	w.refuse(&PawnRemove{IDs: []ulid.ULID{testID(999)}}, w.gm, CodeNotFound)
}

// A pawn spawns on top of what is already there, so a GM dropping a monster
// onto a crowded square gets the monster rather than a shuffle.
func TestASpawnedPawnLandsOnTop(t *testing.T) {
	w := newWorld(t)

	first := w.spawn(Pawn{Name: "First", Visible: true})
	second := w.spawn(Pawn{Name: "Second", Visible: true})

	if w.s.Pawn(second).Z <= w.s.Pawn(first).Z {
		t.Fatalf("the second pawn's z is %d, not above the first's %d", w.s.Pawn(second).Z, w.s.Pawn(first).Z)
	}
}

func sizep(s Size) *Size { return &s }
