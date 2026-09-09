package room

import (
	"testing"

	"github.com/oklog/ulid/v2"
)

// The first press of the button starts round one. There is no turn ending, so
// only the start-of-turn conditions on the first combatant tick.
func TestTheFirstAdvanceStartsRoundOne(t *testing.T) {
	w := newWorld(t)
	first, second := w.twoInTheOrder()

	if w.s.Initiative.Round != 0 || w.s.Initiative.Active != nil {
		t.Fatal("a freshly set tracker is already running")
	}

	w.apply(&InitiativeNext{}, w.gm)

	if w.s.Initiative.Round != 1 {
		t.Fatalf("round = %d after the first advance, want 1", w.s.Initiative.Round)
	}
	if w.s.Initiative.Active == nil || *w.s.Initiative.Active != first {
		t.Fatal("the first advance did not land on the first entry")
	}
	_ = second
}

// The order wraps and the round goes up when it does, which is what makes "how
// long has this spell been running" answerable.
func TestAdvancingPastTheEndWrapsAndCountsARound(t *testing.T) {
	w := newWorld(t)
	first, second := w.twoInTheOrder()

	w.apply(&InitiativeNext{}, w.gm)
	w.apply(&InitiativeNext{}, w.gm)

	if *w.s.Initiative.Active != second || w.s.Initiative.Round != 1 {
		t.Fatalf("after two advances the tracker is on %v in round %d", w.s.Initiative.Active, w.s.Initiative.Round)
	}

	w.apply(&InitiativeNext{}, w.gm)

	if *w.s.Initiative.Active != first {
		t.Fatal("the third advance did not wrap to the first entry")
	}
	if w.s.Initiative.Round != 2 {
		t.Fatalf("round = %d after wrapping, want 2", w.s.Initiative.Round)
	}
}

// THE CONDITION SEAM. "Until the end of your next turn" counts down as your
// turn ends and "until the start of your next turn" as it begins, which is the
// distinction 5e makes and the reason a condition carries a trigger at all.
func TestConditionsCountDownAtTheRightEndOfATurn(t *testing.T) {
	w := newWorld(t)

	goblin := w.spawn(Pawn{Name: "Goblin", Visible: true})
	ogre := w.spawn(Pawn{Name: "Ogre", Visible: true})

	w.apply(&PawnSetConditions{ID: goblin, Conditions: []Condition{
		{Name: "Burning", Color: ColorOrange, Duration: 1, Clear: ClearEnd},
		{Name: "Blessed", Color: ColorYellow, Duration: 2, Clear: ClearStart},
		{Name: "Prone", Color: ColorWhite, Duration: -1, Clear: ClearEnd},
	}}, w.gm)

	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}},
		{Name: "Ogre", PawnIDs: []ulid.ULID{ogre}},
	}}, w.gm)

	// The goblin's turn begins: only its start-of-turn condition ticks.
	w.apply(&InitiativeNext{}, w.gm)
	byName := conditionsByName(w.s.Pawn(goblin))
	if byName["Blessed"] != 1 {
		t.Fatalf("Blessed is at %d after the goblin's turn began, want 1", byName["Blessed"])
	}
	if byName["Burning"] != 1 {
		t.Fatalf("Burning ticked at the start of a turn, and it is an end-of-turn condition")
	}

	// The goblin's turn ends: the end-of-turn condition ticks to zero and goes.
	ems := w.apply(&InitiativeNext{}, w.gm)
	byName = conditionsByName(w.s.Pawn(goblin))
	if _, still := byName["Burning"]; still {
		t.Fatal("Burning reached zero and was not removed")
	}
	if byName["Prone"] != -1 {
		t.Fatalf("Prone is at %d; a duration of -1 never counts down", byName["Prone"])
	}

	// The tracker first, then the pawn whose conditions changed -- with the
	// usual pair of audiences, because a player sees the projected pawn.
	equalStrings(t, "emissions", summary(ems), []string{
		"initiative.updated to all",
		"pawn.updated to gm",
		"pawn.updated to players",
	})
}

// An advance that changes nobody's conditions emits the tracker alone.
func TestAnAdvanceWithNoConditionsEmitsOnlyTheTracker(t *testing.T) {
	w := newWorld(t)
	w.twoInTheOrder()

	ems := w.apply(&InitiativeNext{}, w.gm)
	equalStrings(t, "emissions", summary(ems), []string{"initiative.updated to all"})
}

// Killing the creature whose turn it is carries the fight on to the next
// combatant rather than leaving the tracker pointed at nothing.
func TestDeletingTheActiveCombatantAdvancesTheTurn(t *testing.T) {
	w := newWorld(t)

	first := w.spawn(Pawn{Name: "First", Visible: true})
	second := w.spawn(Pawn{Name: "Second", Visible: true})
	third := w.spawn(Pawn{Name: "Third", Visible: true})

	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "First", PawnIDs: []ulid.ULID{first}},
		{Name: "Second", PawnIDs: []ulid.ULID{second}},
		{Name: "Third", PawnIDs: []ulid.ULID{third}},
	}}, w.gm)
	w.apply(&InitiativeNext{}, w.gm)
	w.apply(&InitiativeNext{}, w.gm)

	active := *w.s.Initiative.Active
	if w.s.entry(active).Name != "Second" {
		t.Fatalf("the tracker is on %q, want Second", w.s.entry(active).Name)
	}

	w.apply(&PawnRemove{IDs: []ulid.ULID{second}}, w.gm)

	if w.s.Initiative.Active == nil {
		t.Fatal("removing the active combatant left the tracker pointed at nothing")
	}
	if got := w.s.entry(*w.s.Initiative.Active).Name; got != "Third" {
		t.Fatalf("the tracker moved to %q, want Third", got)
	}
}

// Emptying the tracker takes the round with it, because the round is a count of
// something that is no longer happening.
func TestClearingTheTrackerResetsTheRound(t *testing.T) {
	w := newWorld(t)
	w.twoInTheOrder()
	w.apply(&InitiativeNext{}, w.gm)

	ems := w.apply(&InitiativeClear{}, w.gm)
	equalStrings(t, "emissions", summary(ems), []string{"initiative.updated to all"})

	if len(w.s.Initiative.Entries) != 0 || w.s.Initiative.Active != nil || w.s.Initiative.Round != 0 {
		t.Fatalf("the tracker after clearing is %+v", w.s.Initiative)
	}
}

// Editing the order mid-fight is not the fight starting again, so the round
// number survives -- it is what the party's spell durations are counted in.
func TestEditingTheOrderKeepsTheRound(t *testing.T) {
	w := newWorld(t)
	first, _ := w.twoInTheOrder()

	w.apply(&InitiativeNext{}, w.gm)
	w.apply(&InitiativeNext{}, w.gm)
	w.apply(&InitiativeNext{}, w.gm)

	round := w.s.Initiative.Round
	if round < 2 {
		t.Fatalf("the fight is only in round %d; the test needs it past the first wrap", round)
	}

	entries := w.s.Initiative.Entries
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{entries[1], entries[0]}, Active: &first}, w.gm)

	if w.s.Initiative.Round != round {
		t.Fatalf("reordering reset the round to %d, want %d", w.s.Initiative.Round, round)
	}
	if w.s.Initiative.Entries[0].Name != entries[1].Name {
		t.Fatal("the entries were not reordered")
	}
}

// A free-text line has no pawn, which is how a lair action gets a slot in the
// order. Nothing about the tick or the projection may assume there is a pawn.
func TestAFreeTextEntryNeedsNoPawn(t *testing.T) {
	w := newWorld(t)

	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Lair action", Initiative: 20},
	}}, w.gm)
	w.apply(&InitiativeNext{}, w.gm)

	if len(w.s.Project(RolePlayer).Initiative.Entries) != 1 {
		t.Fatal("the players' tracker dropped a line that names no pawn")
	}
}

// The tracker cannot name a pawn that is not there, and cannot point at a turn
// that is not one of its own lines.
func TestTheTrackerRefusesWhatItCannotName(t *testing.T) {
	w := newWorld(t)
	missing := testID(999)

	w.refuse(&InitiativeSet{Entries: []InitiativeEntry{{Name: "Ghost", PawnIDs: []ulid.ULID{missing}}}}, w.gm, CodeNotFound)
	w.refuse(&InitiativeSet{Entries: []InitiativeEntry{{Name: "Goblin"}}, Active: &missing}, w.gm, CodeInvalid)
	w.refuse(&InitiativeSet{Entries: []InitiativeEntry{{Name: "  "}}}, w.gm, CodeInvalid)
	w.refuse(&InitiativeNext{}, w.gm, CodeInvalid)
}

// twoInTheOrder puts two pawns in the tracker and answers with their entry ids.
func (w *world) twoInTheOrder() (first, second ulid.ULID) {
	w.t.Helper()

	a := w.spawn(Pawn{Name: "First", Visible: true})
	b := w.spawn(Pawn{Name: "Second", Visible: true})

	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "First", PawnIDs: []ulid.ULID{a}, Initiative: 18},
		{Name: "Second", PawnIDs: []ulid.ULID{b}, Initiative: 12},
	}}, w.gm)

	return w.s.Initiative.Entries[0].ID, w.s.Initiative.Entries[1].ID
}

func conditionsByName(p *Pawn) map[string]int {
	out := map[string]int{}
	for _, c := range p.Conditions {
		out[c.Name] = c.Duration
	}

	return out
}
