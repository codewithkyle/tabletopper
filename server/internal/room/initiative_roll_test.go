package room

import (
	"testing"

	"github.com/oklog/ulid/v2"
)

func rolledWorld(t *testing.T) (*world, []ulid.ULID) {
	t.Helper()
	w := newWorld(t)
	first := w.spawn(Pawn{Name: "Ari", Kind: PawnPlayer, Visible: true, CharacterID: &testCharID})
	second := w.spawn(Pawn{Name: "Goblin", Kind: PawnMonster, Visible: true, MonsterID: idp(testMonsterID)})
	third := w.spawn(Pawn{Name: "Ogre", Kind: PawnNPC, Visible: true})
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Ari", PawnIDs: []ulid.ULID{first}},
		{Name: "Goblin", PawnIDs: []ulid.ULID{second}},
		{Name: "Ogre", PawnIDs: []ulid.ULID{third}},
	}}, w.gm)
	ids := make([]ulid.ULID, 0, 3)
	for _, e := range w.s.Initiative.Entries {
		ids = append(ids, e.ID)
	}
	return w, ids
}
func entryOrder(s *State) []string {
	out := make([]string, 0, len(s.Initiative.Entries))
	for _, e := range s.Initiative.Entries {
		out = append(out, e.Name)
	}
	return out
}

func TestRollingTheOrderNumbersEveryEntryAndSortsThem(t *testing.T) {
	w, ids := rolledWorld(t)
	w.apply(&InitiativeRoll{Bonuses: map[ulid.ULID]int{ids[0]: 0, ids[1]: 0, ids[2]: 0}}, w.gm)
	equalStrings(t, "order", entryOrder(w.s), []string{"Ogre", "Goblin", "Ari"})
	for _, e := range w.s.Initiative.Entries {
		if e.Initiative == 0 {
			t.Fatalf("%s was left without a number", e.Name)
		}
	}
	if w.s.Initiative.Entries[0].Initiative != 3 {
		t.Fatalf("the top of the order rolled %d, want the highest of 1, 2 and 3",
			w.s.Initiative.Entries[0].Initiative)
	}
}
func TestABonusIsAddedToWhatWasRolled(t *testing.T) {
	w, ids := rolledWorld(t)
	w.apply(&InitiativeRoll{Bonuses: map[ulid.ULID]int{ids[0]: 10, ids[1]: 0, ids[2]: -1}}, w.gm)
	equalStrings(t, "order", entryOrder(w.s), []string{"Ari", "Goblin", "Ogre"})
	want := map[string]int{"Ari": 11, "Goblin": 2, "Ogre": 2}
	for _, e := range w.s.Initiative.Entries {
		if e.Initiative != want[e.Name] {
			t.Errorf("%s rolled %d, want %d", e.Name, e.Initiative, want[e.Name])
		}
	}
}
func TestATieKeepsTheOrderTheTrackerAlreadyHad(t *testing.T) {
	w, ids := rolledWorld(t)
	w.apply(&InitiativeRoll{Bonuses: map[ulid.ULID]int{ids[0]: 2, ids[1]: 1, ids[2]: 0}}, w.gm)
	equalStrings(t, "order", entryOrder(w.s), []string{"Ari", "Goblin", "Ogre"})
	for _, e := range w.s.Initiative.Entries {
		if e.Initiative != 3 {
			t.Fatalf("%s rolled %d; the fixture is meant to tie them all on 3", e.Name, e.Initiative)
		}
	}
}
func TestRollingTheOrderLeavesTheTurnAndTheRoundAlone(t *testing.T) {
	w, ids := rolledWorld(t)
	w.apply(&InitiativeNext{}, w.gm)
	active, round := w.s.Initiative.Active, w.s.Initiative.Round
	if active == nil {
		t.Fatal("the fixture did not start anybody's turn")
	}
	w.apply(&InitiativeRoll{Bonuses: map[ulid.ULID]int{ids[0]: 0, ids[1]: 0, ids[2]: 0}}, w.gm)
	if w.s.Initiative.Active == nil || *w.s.Initiative.Active != *active {
		t.Errorf("rolling moved the turn to %v, want it left on %v", w.s.Initiative.Active, active)
	}
	if w.s.Initiative.Round != round {
		t.Errorf("rolling changed the round to %d, want %d", w.s.Initiative.Round, round)
	}
}
func TestRollingTheOrderStaysOutOfTheDiceLog(t *testing.T) {
	w, ids := rolledWorld(t)
	w.apply(&InitiativeRoll{Bonuses: map[ulid.ULID]int{ids[0]: 0, ids[1]: 0, ids[2]: 0}}, w.gm)
	if len(w.s.Rolls) != 0 {
		t.Fatalf("rolling the order put %d rows in the dice log; the tracker is where those numbers live", len(w.s.Rolls))
	}
}
func TestRollingTheOrderReadsEveryBonusFromTheLibrary(t *testing.T) {
	w, ids := rolledWorld(t)
	lib := newLibrary()
	monster := goblin()
	monster.InitiativeBonus = 2
	lib.monsters[testMonsterID] = monster
	lib.characters[testCharID] = CharacterInfo{ID: testCharID, Name: "Ari", InitiativeBonus: 5}
	cmd := &InitiativeRoll{}
	w.resolve(cmd, lib)
	want := map[ulid.ULID]int{ids[0]: 5, ids[1]: 2, ids[2]: 0}
	for id, bonus := range want {
		if cmd.Bonuses[id] != bonus {
			t.Errorf("entry %s resolved to %d, want %d", id, cmd.Bonuses[id], bonus)
		}
	}
}
func TestABonusFromABrokenRowIsClamped(t *testing.T) {
	w, ids := rolledWorld(t)
	lib := newLibrary()
	lib.characters[testCharID] = CharacterInfo{ID: testCharID, Name: "Ari", InitiativeBonus: 30_000}
	monster := goblin()
	monster.InitiativeBonus = -30_000
	lib.monsters[testMonsterID] = monster
	cmd := &InitiativeRoll{}
	w.resolve(cmd, lib)
	if cmd.Bonuses[ids[0]] != DiceModLimit {
		t.Errorf("a runaway bonus resolved to %d, want it clamped to %d", cmd.Bonuses[ids[0]], DiceModLimit)
	}
	if cmd.Bonuses[ids[1]] != -DiceModLimit {
		t.Errorf("a runaway penalty resolved to %d, want it clamped to %d", cmd.Bonuses[ids[1]], -DiceModLimit)
	}
}
func TestAGroupOfMonstersRollsOnce(t *testing.T) {
	w := newWorld(t)
	first := w.spawn(Pawn{Name: "Goblin", Kind: PawnMonster, Visible: true, MonsterID: idp(testMonsterID)})
	second := w.spawn(Pawn{Name: "Goblin", Kind: PawnMonster, Visible: true, MonsterID: idp(testMonsterID)})
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Goblin", PawnIDs: []ulid.ULID{first, second}},
	}}, w.gm)
	lib := newLibrary()
	monster := goblin()
	monster.InitiativeBonus = 2
	lib.monsters[testMonsterID] = monster
	cmd := &InitiativeRoll{}
	w.resolve(cmd, lib)
	if len(cmd.Bonuses) != 1 {
		t.Fatalf("two goblins in one entry resolved %d bonuses, want 1", len(cmd.Bonuses))
	}
	w.apply(cmd, w.gm)
	if len(w.s.Initiative.Entries) != 1 || w.s.Initiative.Entries[0].Initiative != 3 {
		t.Fatalf("the group rolled %+v, want a single 1d20+2 of 3", w.s.Initiative.Entries)
	}
}
func TestRollingAnEmptyTrackerIsRefusedBeforeItReachesTheRoom(t *testing.T) {
	w := newWorld(t)
	w.refuseResolve(&InitiativeRoll{}, newLibrary(), CodeInvalid)
	w.refuse(&InitiativeRoll{}, w.gm, CodeInvalid)
}
func TestAnUnresolvedRollIsRefusedRatherThanRolledAtZero(t *testing.T) {
	w, _ := rolledWorld(t)
	e := w.refuse(&InitiativeRoll{}, w.gm, CodeInvalid)
	if e.Heading != "Nothing to roll for" {
		t.Fatalf("heading = %q, want it to say there was nothing to roll for", e.Heading)
	}
	if w.s.Initiative.Entries[0].Initiative != 0 {
		t.Error("a refused roll still numbered the tracker")
	}
}
