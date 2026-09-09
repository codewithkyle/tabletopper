package room

import (
	"testing"

	"github.com/oklog/ulid/v2"
)

// THE DEAD ARE SKIPPED AND A PLAYER IS NEVER THE DEAD. That one clause is the
// whole difference between the two kinds of creature this app draws, and it is
// the reason these tests are written out one case at a time.

// A dead monster's turn is a turn wasted, so the button does not spend one.
func TestNextPassesOverADeadMonster(t *testing.T) {
	w := newWorld(t)

	ari := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true, HP: intp(11), MaxHP: intp(11)})
	goblin := w.spawn(Pawn{Name: "Goblin", Visible: true, HP: intp(0), MaxHP: intp(7)})
	ogre := w.spawn(Pawn{Name: "Ogre", Visible: true, HP: intp(30), MaxHP: intp(30)})

	w.order(ari, goblin, ogre)

	w.apply(&InitiativeNext{}, w.gm)
	w.apply(&InitiativeNext{}, w.gm)

	if got := w.activeName(); got != "Ogre" {
		t.Fatalf("the second advance landed on %q, want the ogre", got)
	}
}

// A PLAYER AT ZERO HAS THE MOST CONSEQUENTIAL TURN OF THAT CHARACTER'S LIFE --
// three saves against three failures -- and an app that skipped it would be an
// app that killed somebody's character by omission.
func TestNextDoesNotPassOverADeadPlayerPawn(t *testing.T) {
	w := newWorld(t)

	ari := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true, HP: intp(0), MaxHP: intp(11)})
	ogre := w.spawn(Pawn{Name: "Ogre", Visible: true, HP: intp(30), MaxHP: intp(30)})

	w.order(ari, ogre)
	w.apply(&InitiativeNext{}, w.gm)

	if got := w.activeName(); got != "Ari" {
		t.Fatalf("the first advance landed on %q, want the player making death saves", got)
	}
}

// A group with one goblin still standing is not skipped, which is what makes
// the count on its card matter.
func TestNextDoesNotPassOverAGroupWithASurvivor(t *testing.T) {
	w := newWorld(t)

	ari := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true, HP: intp(11), MaxHP: intp(11)})
	dead := w.spawn(Pawn{Name: "Goblin", Visible: true, HP: intp(0), MaxHP: intp(7)})
	alive := w.spawn(Pawn{Name: "Goblin", Visible: true, HP: intp(7), MaxHP: intp(7)})

	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Ari", PawnIDs: []ulid.ULID{ari}},
		{Name: "Goblin", PawnIDs: []ulid.ULID{dead, alive}},
	}}, w.gm)

	w.apply(&InitiativeNext{}, w.gm)
	w.apply(&InitiativeNext{}, w.gm)

	if got := w.activeName(); got != "Goblin" {
		t.Fatalf("the second advance landed on %q, want the goblins", got)
	}
}

// A LINE WITH NO PAWNS IS NEVER SKIPPED. Nothing about a lair action can be
// dead.
func TestNextNeverPassesOverANamedLine(t *testing.T) {
	w := newWorld(t)

	goblin := w.spawn(Pawn{Name: "Goblin", Visible: true, HP: intp(0), MaxHP: intp(7)})

	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}},
		{Name: "Lair action"},
	}}, w.gm)

	w.apply(&InitiativeNext{}, w.gm)

	if got := w.activeName(); got != "Lair action" {
		t.Fatalf("the first advance landed on %q, want the lair action", got)
	}
}

// A GM WHO PRESSES NEXT ON A FINISHED FIGHT SHOULD GET A PRESS, NOT A HANG. A
// tracker of nothing but corpses advances by one, and crossing the end of it
// counts a round like any other lap.
func TestNextAdvancesByOneThroughATrackerOfCorpses(t *testing.T) {
	w := newWorld(t)

	first := w.spawn(Pawn{Name: "First", Visible: true, HP: intp(0), MaxHP: intp(7)})
	second := w.spawn(Pawn{Name: "Second", Visible: true, HP: intp(0), MaxHP: intp(7)})

	w.order(first, second)

	w.apply(&InitiativeNext{}, w.gm)
	if got := w.activeName(); got != "First" {
		t.Fatalf("the first advance landed on %q, want the first line", got)
	}

	w.apply(&InitiativeNext{}, w.gm)
	if got := w.activeName(); got != "Second" {
		t.Fatalf("the second advance landed on %q, want the second line", got)
	}
	if w.s.Initiative.Round != 1 {
		t.Fatalf("round = %d without crossing the end, want 1", w.s.Initiative.Round)
	}

	w.apply(&InitiativeNext{}, w.gm)
	if got := w.activeName(); got != "First" || w.s.Initiative.Round != 2 {
		t.Fatalf("the lap landed on %q in round %d, want the first line in round 2", got, w.s.Initiative.Round)
	}
}

// THE ROUND COUNTS THE SEAM AND NOT THE SKIPS. Passing over four dead goblins
// on the way past the end of the list is still one lap of the table.
func TestARoundCountsOnceHoweverManyLinesAreSkipped(t *testing.T) {
	w := newWorld(t)

	ari := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true, HP: intp(11), MaxHP: intp(11)})
	var corpses []ulid.ULID
	for range 4 {
		corpses = append(corpses, w.spawn(Pawn{Name: "Goblin", Visible: true, HP: intp(0), MaxHP: intp(7)}))
	}

	w.order(append([]ulid.ULID{ari}, corpses...)...)

	// Round one begins on the player, and the next press walks over all four
	// corpses and back round to them.
	w.apply(&InitiativeNext{}, w.gm)
	if w.s.Initiative.Round != 1 {
		t.Fatalf("round = %d after the first advance, want 1", w.s.Initiative.Round)
	}

	w.apply(&InitiativeNext{}, w.gm)

	if got := w.activeName(); got != "Ari" {
		t.Fatalf("the lap landed on %q, want back on the player", got)
	}
	if w.s.Initiative.Round != 2 {
		t.Fatalf("round = %d after one lap over four corpses, want 2", w.s.Initiative.Round)
	}
}

// A LINE THAT WAS SKIPPED TICKS NOTHING. Its turn did not happen, and a
// condition counting down on a corpse is bookkeeping about a creature that has
// stopped taking turns.
func TestASkippedLineDoesNotTickItsConditions(t *testing.T) {
	w := newWorld(t)

	ari := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true, HP: intp(11), MaxHP: intp(11)})
	goblin := w.spawn(Pawn{Name: "Goblin", Visible: true, HP: intp(0), MaxHP: intp(7)})
	ogre := w.spawn(Pawn{Name: "Ogre", Visible: true, HP: intp(30), MaxHP: intp(30)})

	w.apply(&PawnSetConditions{ID: goblin, Conditions: []Condition{
		{Name: "Burning", Color: ColorOrange, Duration: 3, Clear: ClearStart},
	}}, w.gm)

	w.order(ari, goblin, ogre)

	w.apply(&InitiativeNext{}, w.gm)
	w.apply(&InitiativeNext{}, w.gm)

	if got := conditionsByName(w.s.Pawn(goblin))["Burning"]; got != 3 {
		t.Fatalf("the skipped corpse's condition is at %d turns, want 3", got)
	}
}

// EVERY MEMBER OF A GROUP TICKS AT THE SEAM, because the group is one turn and
// every creature in it took it.
func TestAGroupTicksEveryMember(t *testing.T) {
	w := newWorld(t)

	one := w.spawn(Pawn{Name: "Goblin", Visible: true, HP: intp(7), MaxHP: intp(7)})
	two := w.spawn(Pawn{Name: "Goblin", Visible: true, HP: intp(7), MaxHP: intp(7)})

	for _, id := range []ulid.ULID{one, two} {
		w.apply(&PawnSetConditions{ID: id, Conditions: []Condition{
			{Name: "Blessed", Color: ColorYellow, Duration: 2, Clear: ClearStart},
		}}, w.gm)
	}

	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Goblin", PawnIDs: []ulid.ULID{one, two}},
	}}, w.gm)

	w.apply(&InitiativeNext{}, w.gm)

	for _, id := range []ulid.ULID{one, two} {
		if got := conditionsByName(w.s.Pawn(id))["Blessed"]; got != 1 {
			t.Fatalf("a group member's condition is at %d turns, want 1", got)
		}
	}
}

// ONE PAWN IS IN THE ORDER ONCE, across every line and not only within one. A
// goblin in its group AND on a line of its own would take two turns and tick
// its conditions twice, and the second of those is silent.
func TestSetRefusesTheSamePawnTwice(t *testing.T) {
	w := newWorld(t)
	goblin := w.spawn(Pawn{Name: "Goblin", Visible: true})

	w.refuse(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}},
		{Name: "Goblin again", PawnIDs: []ulid.ULID{goblin}},
	}}, w.gm, CodeInvalid)

	w.refuse(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Goblins", PawnIDs: []ulid.ULID{goblin, goblin}},
	}}, w.gm, CodeInvalid)
}

// WHOEVER OWNS A PAWN IN THE ACTING LINE MAY END ITS TURN, and a group is a
// line like any other -- so a player whose character is one of two creatures on
// a count may still press the button.
func TestAPlayerMayEndAGroupTurnTheyAreIn(t *testing.T) {
	w := newWorld(t)

	mine := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true, OwnerID: idp(testPlayerID)})
	familiar := w.spawn(Pawn{Kind: PawnNPC, Name: "Owl", Visible: true})

	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Ari and the owl", PawnIDs: []ulid.ULID{familiar, mine}},
	}}, w.gm)
	w.apply(&InitiativeNext{}, w.gm)

	if err := (&InitiativeNext{}).Authorize(w.s, w.pc); err != nil {
		t.Fatalf("the pawn's owner may not end the turn: %v", err)
	}
	if err := (&InitiativeNext{}).Authorize(w.s, w.other); err == nil {
		t.Fatal("somebody who owns nothing in the acting line may end its turn")
	}
}

// room.Health IS THE GO TWIN OF healthOf IN wounds.ts, and the two have to
// agree or a card and the sprite beside it disagree about the same creature.
// The number wins where both arrive; the band is what a viewer was given
// INSTEAD of the numbers.
func TestHealthReadsTheNumberFirstAndTheBandSecond(t *testing.T) {
	band := BandBruised

	cases := []struct {
		name string
		pawn Pawn
		want *HPBand
	}{
		{"nothing at all", Pawn{}, nil},
		{"numbers alone", Pawn{HP: intp(3), MaxHP: intp(7)}, bandp(BandBloody)},
		{"a band alone", Pawn{HPBand: &band}, bandp(BandBruised)},
		{"both, and the number wins", Pawn{HP: intp(0), MaxHP: intp(7), HPBand: &band}, bandp(BandDead)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Health(tc.pawn)

			switch {
			case got == nil && tc.want == nil:
			case got == nil || tc.want == nil:
				t.Fatalf("Health = %v, want %v", got, tc.want)
			case *got != *tc.want:
				t.Fatalf("Health = %s, want %s", *got, *tc.want)
			}
		})
	}
}

// order puts every pawn in the tracker on a line of its own, in the order given,
// which is what most of these tests want and none of them want to spell out.
func (w *world) order(pawns ...ulid.ULID) {
	w.t.Helper()

	entries := make([]InitiativeEntry, 0, len(pawns))
	for _, id := range pawns {
		p := w.s.Pawn(id)
		if p == nil {
			w.t.Fatalf("order: no pawn %s", id)
		}
		entries = append(entries, InitiativeEntry{Name: p.Name, PawnIDs: []ulid.ULID{id}})
	}

	w.apply(&InitiativeSet{Entries: entries}, w.gm)
}

// activeName is whose turn it is, as a reader would say it.
func (w *world) activeName() string {
	w.t.Helper()

	if w.s.Initiative.Active == nil {
		return ""
	}

	e := w.s.entry(*w.s.Initiative.Active)
	if e == nil {
		return ""
	}

	return e.Name
}

func bandp(b HPBand) *HPBand { return &b }
