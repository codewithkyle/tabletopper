package room

import (
	"testing"

	"github.com/oklog/ulid/v2"
)

func TestSyncTakesTheCreaturesOnTheFloorsThePartyIsOn(t *testing.T) {
	w := newWorld(t)
	upstairs := w.addLayer("First floor")
	ari := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true, OwnerID: idp(testPlayerID)})
	goblin := w.spawn(Pawn{Name: "Goblin", Visible: true, MonsterID: idp(testID(60))})
	captain := w.spawn(Pawn{Kind: PawnNPC, Name: "Captain", Visible: true})
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
func TestSyncWithNoPartyIsRefusedWithASentence(t *testing.T) {
	w := newWorld(t)
	w.spawn(Pawn{Name: "Goblin", Visible: true})
	w.refuse(&InitiativeSync{}, w.gm, CodeInvalid)
}
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
func TestSyncNeverGroupsNPCs(t *testing.T) {
	w := newWorld(t)
	w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true})
	w.spawn(Pawn{Kind: PawnNPC, Name: "Guard", Visible: true})
	w.spawn(Pawn{Kind: PawnNPC, Name: "Guard", Visible: true})
	w.apply(&InitiativeSync{}, w.gm)
	equalStrings(t, "the order", entryNames(w.s), []string{"Ari", "Guard", "Guard"})
}
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
func TestSyncKeepsTheOrderAndAppendsWhatIsNew(t *testing.T) {
	w := newWorld(t)
	w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true})
	w.spawn(Pawn{Kind: PawnNPC, Name: "Captain", Visible: true})
	w.apply(&InitiativeSync{}, w.gm)
	flipped := []InitiativeEntry{w.s.Initiative.Entries[1], w.s.Initiative.Entries[0]}
	w.apply(&InitiativeSet{Entries: flipped}, w.gm)
	w.spawn(Pawn{Kind: PawnNPC, Name: "Informant", Visible: true})
	w.apply(&InitiativeSync{}, w.gm)
	equalStrings(t, "the order", entryNames(w.s), []string{"Captain", "Ari", "Informant"})
}
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
func TestSyncIsTheGMs(t *testing.T) {
	w := newWorld(t)
	w.refuse(&InitiativeSync{}, w.pc, CodeForbidden)
}
func (w *world) individual() {
	w.t.Helper()
	w.apply(&TableSetOptions{
		PawnLabels:         w.s.Table.PawnLabels,
		PlayersCanDraw:     w.s.Table.PlayersCanDraw,
		InitiativeGrouping: GroupIndividual,
	}, w.gm)
}
func entryNames(s *State) []string {
	out := make([]string, 0, len(s.Initiative.Entries))
	for _, e := range s.Initiative.Entries {
		out = append(out, e.Name)
	}
	return out
}
