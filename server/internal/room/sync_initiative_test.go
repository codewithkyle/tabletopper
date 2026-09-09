package room

import (
	"testing"

	"github.com/oklog/ulid/v2"
)

// SYNC IS HOW A FIGHT STARTS AND HOW IT GROWS, and every clause of it is a rule
// about room state that nothing outside the room can see. These tests are the
// clauses, one apiece.

// The shape of a fresh sync: the party, then the monsters, and nothing from a
// floor the party is not on.
func TestSyncTakesTheCreaturesOnTheFloorsThePartyIsOn(t *testing.T) {
	w := newWorld(t)
	upstairs := w.addLayer("First floor")

	ari := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true, OwnerID: idp(testPlayerID)})
	goblin := w.spawn(Pawn{Name: "Goblin", Visible: true, MonsterID: idp(testID(60))})
	captain := w.spawn(Pawn{Kind: PawnNPC, Name: "Captain", Visible: true})

	// None of these three belongs in the order: the wagon is not a creature,
	// the ambusher has not been met, and the dragon is on a floor nobody is
	// standing on.
	w.spawn(Pawn{Kind: PawnObject, Name: "Wagon", Visible: true, Width: 128, Height: 256})
	w.spawn(Pawn{Name: "Ambusher", Visible: false})
	w.spawn(Pawn{Name: "Dragon", Visible: true, LayerID: upstairs})

	w.apply(&InitiativeSync{}, w.gm)

	equalStrings(t, "the order", entryNames(w.s), []string{"Ari", "Goblin", "Captain"})

	if got := w.s.Initiative.Entries[0].PawnIDs; len(got) != 1 || got[0] != ari {
		t.Fatalf("the first line names %v, want the player's pawn", got)
	}
	if got := w.s.Initiative.Entries[1].PawnIDs; len(got) != 1 || got[0] != goblin {
		t.Fatalf("the goblin's line names %v", got)
	}
	if got := w.s.Initiative.Entries[2].PawnIDs; len(got) != 1 || got[0] != captain {
		t.Fatalf("the captain's line names %v", got)
	}
}

// A ROOM WITH NOBODY ON THE TABLE HAS NO ORDER TO BUILD, and saying so is
// better than a button that appears to do nothing.
func TestSyncWithNoPartyIsRefusedWithASentence(t *testing.T) {
	w := newWorld(t)
	w.spawn(Pawn{Name: "Goblin", Visible: true})

	w.refuse(&InitiativeSync{}, w.gm, CodeInvalid)
}

// GROUPED IS THE DEFAULT AND IT IS ONE LINE PER KIND OF MONSTER. One war chief,
// three bugbears and nine goblins is three lines and not thirteen.
func TestSyncGroupsMonstersByDefault(t *testing.T) {
	w := newWorld(t)
	w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true})

	chief := w.spawn(Pawn{Name: "Goblin War Chief", Visible: true, MonsterID: idp(testID(61))})
	var goblins []ulid.ULID
	for range 9 {
		goblins = append(goblins, w.spawn(Pawn{Name: "Goblin", Visible: true, MonsterID: idp(testID(60))}))
	}

	w.apply(&InitiativeSync{}, w.gm)

	equalStrings(t, "the order", entryNames(w.s), []string{"Ari", "Goblin War Chief", "Goblin"})

	if got := w.s.Initiative.Entries[1].PawnIDs; len(got) != 1 || got[0] != chief {
		t.Fatalf("the chief's line names %v", got)
	}
	if got := len(w.s.Initiative.Entries[2].PawnIDs); got != len(goblins) {
		t.Fatalf("the goblin line holds %d pawns, want %d", got, len(goblins))
	}
}

// AND INDIVIDUAL IS ONE LINE PER PAWN, which is the same table with the setting
// moved and is why the setting exists.
func TestSyncMakesOneLinePerPawnWhenIndividual(t *testing.T) {
	w := newWorld(t)
	w.individual()

	w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true})
	for range 3 {
		w.spawn(Pawn{Name: "Goblin", Visible: true, MonsterID: idp(testID(60))})
	}

	w.apply(&InitiativeSync{}, w.gm)

	equalStrings(t, "the order", entryNames(w.s), []string{"Ari", "Goblin", "Goblin", "Goblin"})
}

// AN NPC IS A NAMED INDIVIDUAL AND IS NEVER GROUPED WITH ANOTHER. The kinds
// exist to draw that line; grouping two of them under one card would be the app
// deciding they are interchangeable.
func TestSyncNeverGroupsNPCs(t *testing.T) {
	w := newWorld(t)
	w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true})
	w.spawn(Pawn{Kind: PawnNPC, Name: "Guard", Visible: true})
	w.spawn(Pawn{Kind: PawnNPC, Name: "Guard", Visible: true})

	w.apply(&InitiativeSync{}, w.gm)

	equalStrings(t, "the order", entryNames(w.s), []string{"Ari", "Guard", "Guard"})
}

// TWO PAWNS WITH NO MONSTER BEHIND THEM ARE THE SAME CREATURE WHEN A TABLE
// WOULD SAY THEY ARE: same name, same picture. A "Goblin" dragged out of the
// asset library has no manual id, and two of them are still two goblins.
func TestSyncGroupsLibraryTokensByNameAndPicture(t *testing.T) {
	w := newWorld(t)
	w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true})
	w.spawn(Pawn{Name: "Goblin", Image: "/assets/goblin.webp", Visible: true})
	w.spawn(Pawn{Name: "Goblin", Image: "/assets/goblin.webp", Visible: true})
	w.spawn(Pawn{Name: "Goblin", Image: "/assets/other.webp", Visible: true})

	w.apply(&InitiativeSync{}, w.gm)

	equalStrings(t, "the order", entryNames(w.s), []string{"Ari", "Goblin", "Goblin"})

	if got := len(w.s.Initiative.Entries[1].PawnIDs); got != 2 {
		t.Fatalf("the first goblin line holds %d pawns, want 2", got)
	}
}

// A CORPSE GOES AND A DEAD PLAYER PAWN STAYS, which is the same exception the
// turn key makes: a player at zero is making death saving throws and is still
// in the fight.
func TestSyncDropsDeadMonstersAndKeepsDeadPlayers(t *testing.T) {
	w := newWorld(t)

	ari := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true, HP: intp(11), MaxHP: intp(11)})
	goblin := w.spawn(Pawn{Name: "Goblin", Visible: true, HP: intp(7), MaxHP: intp(7), MonsterID: idp(testID(60))})

	w.apply(&InitiativeSync{}, w.gm)
	equalStrings(t, "the order", entryNames(w.s), []string{"Ari", "Goblin"})

	w.apply(&PawnUpdate{ID: goblin, HP: intp(0)}, w.gm)
	w.apply(&PawnUpdate{ID: ari, HP: intp(0)}, w.gm)

	w.apply(&InitiativeSync{}, w.gm)

	equalStrings(t, "the order", entryNames(w.s), []string{"Ari"})
	if got := w.s.Initiative.Entries[0].PawnIDs[0]; got != ari {
		t.Fatalf("the surviving line names %v, want the player's pawn", got)
	}
	_ = goblin
}

// SYNC NEVER REORDERS. A GM who has dragged the order into shape and presses it
// again to bring in reinforcements gets their order back with more on the end.
func TestSyncKeepsTheOrderAndAppendsWhatIsNew(t *testing.T) {
	w := newWorld(t)
	w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true})
	w.spawn(Pawn{Kind: PawnNPC, Name: "Captain", Visible: true})

	w.apply(&InitiativeSync{}, w.gm)

	// The GM drags the captain in front of the player, which is what the strip
	// posts back as a whole order.
	flipped := []InitiativeEntry{w.s.Initiative.Entries[1], w.s.Initiative.Entries[0]}
	w.apply(&InitiativeSet{Entries: flipped}, w.gm)

	w.spawn(Pawn{Kind: PawnNPC, Name: "Informant", Visible: true})
	w.apply(&InitiativeSync{}, w.gm)

	equalStrings(t, "the order", entryNames(w.s), []string{"Captain", "Ari", "Informant"})
}

// THREE MORE GOBLINS ARRIVING IN ROUND FOUR ARE MORE GOBLINS, not a second
// goblin turn.
func TestSyncMergesReinforcementsIntoTheirGroup(t *testing.T) {
	w := newWorld(t)
	w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true})
	for range 2 {
		w.spawn(Pawn{Name: "Goblin", Visible: true, MonsterID: idp(testID(60))})
	}

	w.apply(&InitiativeSync{}, w.gm)

	for range 3 {
		w.spawn(Pawn{Name: "Goblin", Visible: true, MonsterID: idp(testID(60))})
	}
	w.apply(&InitiativeSync{}, w.gm)

	equalStrings(t, "the order", entryNames(w.s), []string{"Ari", "Goblin"})
	if got := len(w.s.Initiative.Entries[1].PawnIDs); got != 5 {
		t.Fatalf("the goblin line holds %d pawns after the reinforcements, want 5", got)
	}
}

// THE ROUND SURVIVES A SYNC, for InitiativeSet's reason: reinforcements
// arriving are not the fight starting again, and the round is what the party's
// spell durations are counted in.
func TestSyncLeavesTheRoundAlone(t *testing.T) {
	w := newWorld(t)
	w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true})
	w.spawn(Pawn{Kind: PawnNPC, Name: "Captain", Visible: true})

	w.apply(&InitiativeSync{}, w.gm)
	w.apply(&InitiativeNext{}, w.gm)
	w.apply(&InitiativeNext{}, w.gm)
	w.apply(&InitiativeNext{}, w.gm)

	round := w.s.Initiative.Round
	if round < 2 {
		t.Fatalf("the fight is in round %d; the test needs it past the first", round)
	}

	w.spawn(Pawn{Kind: PawnNPC, Name: "Informant", Visible: true})
	w.apply(&InitiativeSync{}, w.gm)

	if w.s.Initiative.Round != round {
		t.Fatalf("round = %d after a sync, want %d", w.s.Initiative.Round, round)
	}
}

// A PLAYER MAY NOT BUILD THE ORDER.
func TestSyncIsTheGMs(t *testing.T) {
	w := newWorld(t)
	w.refuse(&InitiativeSync{}, w.pc, CodeForbidden)
}

// individual moves the table's setting, which is what a GM does mid-session for
// a fight where each monster matters.
func (w *world) individual() {
	w.t.Helper()

	w.apply(&TableSetOptions{
		PawnLabels:         w.s.Table.PawnLabels,
		PlayersCanDraw:     w.s.Table.PlayersCanDraw,
		InitiativeGrouping: GroupIndividual,
	}, w.gm)
}

// entryNames is the order as a reader would say it out loud.
func entryNames(s *State) []string {
	out := make([]string, 0, len(s.Initiative.Entries))
	for _, e := range s.Initiative.Entries {
		out = append(out, e.Name)
	}

	return out
}
