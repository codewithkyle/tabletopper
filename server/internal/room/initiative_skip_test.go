package room
import (
	"testing"
	"github.com/oklog/ulid/v2"
)
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
func TestARoundCountsOnceHoweverManyLinesAreSkipped(t *testing.T) {
	w := newWorld(t)
	ari := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true, HP: intp(11), MaxHP: intp(11)})
	var corpses []ulid.ULID
	for range 4 {
		corpses = append(corpses, w.spawn(Pawn{Name: "Goblin", Visible: true, HP: intp(0), MaxHP: intp(7)}))
	}
	w.order(append([]ulid.ULID{ari}, corpses...)...)
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
