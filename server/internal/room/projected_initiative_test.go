package room

import (
	"testing"

	"github.com/oklog/ulid/v2"
)

// THE TRACKER AND THE PAWNS IT NAMES ARE PROJECTED IN ONE PASS, and these are
// the tests that say why it had to be one pass.

// THE SECURITY TEST, AND IT IS WRITTEN FIRST. A hidden creature is one the
// party has not met; a line in the turn order naming it would be the giveaway
// the hiding exists to prevent.
func TestAPlayersTrackerNamesNothingHidden(t *testing.T) {
	w := newWorld(t)

	seen := w.spawn(Pawn{Name: "Goblin", Visible: true})
	hidden := w.spawn(Pawn{Name: "Ambusher", Visible: false})

	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Goblin", PawnIDs: []ulid.ULID{seen}},
		{Name: "Ambusher", PawnIDs: []ulid.ULID{hidden}},
	}}, w.gm)

	tracker, pawns := w.s.ProjectedInitiative(RolePlayer)

	if len(tracker.Entries) != 1 || tracker.Entries[0].Name != "Goblin" {
		t.Fatalf("the player's tracker is %v", entryNamesIn(tracker))
	}
	if _, found := pawns[hidden]; found {
		t.Fatal("the hidden pawn reached the player's strip")
	}
	if _, found := pawns[seen]; !found {
		t.Fatal("the visible pawn is missing from the player's strip")
	}
}

// HIDING ONE OF NINE GOBLINS TAKES A DOT OFF THE GROUP AND LEAVES THE LINE, so
// the players' count is the count of what they can see -- which is the whole of
// what a group card is telling them.
func TestHidingOneMemberShortensTheGroupForPlayers(t *testing.T) {
	w := newWorld(t)

	var goblins []ulid.ULID
	for range 3 {
		goblins = append(goblins, w.spawn(Pawn{Name: "Goblin", Visible: true}))
	}

	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Goblin", PawnIDs: goblins},
	}}, w.gm)
	w.apply(&PawnSetVisible{IDs: []ulid.ULID{goblins[0]}, Visible: false}, w.gm)

	tracker, pawns := w.s.ProjectedInitiative(RolePlayer)

	if len(tracker.Entries) != 1 {
		t.Fatalf("the player's tracker holds %d lines, want 1", len(tracker.Entries))
	}
	if got := len(tracker.Entries[0].PawnIDs); got != 2 {
		t.Fatalf("the group names %d pawns for the player, want 2", got)
	}
	if len(pawns) != 2 {
		t.Fatalf("the player was handed %d pawns, want 2", len(pawns))
	}

	tracker, pawns = w.s.ProjectedInitiative(RoleGM)
	if got := len(tracker.Entries[0].PawnIDs); got != 3 || len(pawns) != 3 {
		t.Fatalf("the GM's group names %d pawns and %d were handed over, want 3 of each", got, len(pawns))
	}
}

// HIDING THE WHOLE GROUP TAKES THE LINE, and clears the turn when it was that
// line's.
func TestHidingEveryMemberDropsTheLineAndTheTurn(t *testing.T) {
	w := newWorld(t)

	goblin := w.spawn(Pawn{Name: "Goblin", Visible: true})

	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}},
	}}, w.gm)
	w.apply(&InitiativeNext{}, w.gm)
	w.apply(&PawnSetVisible{IDs: []ulid.ULID{goblin}, Visible: false}, w.gm)

	tracker, _ := w.s.ProjectedInitiative(RolePlayer)

	if len(tracker.Entries) != 0 {
		t.Fatalf("the player's tracker holds %d lines, want none", len(tracker.Entries))
	}
	if tracker.Active != nil {
		t.Fatal("the player's tracker points at a line they cannot see")
	}
}

// THE FLOOR CASE, AND IT IS THE ONE THIS FUNCTION EXISTS FOR.
//
// Project filters a player's pawns through Shown, which gates on the ACTIVE
// LAYER, and the tracker deliberately keeps the line of a creature that walked
// upstairs. Asking those two separately gives a player a nameless line with no
// portrait for the rest of the fight -- so they are asked together, and the gate
// on the pawns is Visible alone.
func TestAPlayerKeepsThePortraitOfAPawnOnAnotherFloor(t *testing.T) {
	w := newWorld(t)
	upstairs := w.addLayer("First floor")

	ari := w.spawn(Pawn{
		Kind: PawnPlayer, Name: "Ari", Visible: true,
		Image: "/assets/ari.webp", OwnerID: idp(testPlayerID),
	})

	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Ari", PawnIDs: []ulid.ULID{ari}},
	}}, w.gm)

	w.apply(&PawnSetLayer{IDs: []ulid.ULID{ari}, Layer: upstairs}, w.gm)

	if w.s.Shown(*w.s.Pawn(ari)) {
		t.Fatal("the pawn is still on the active layer; the test is not exercising the case")
	}

	tracker, pawns := w.s.ProjectedInitiative(RolePlayer)

	if len(tracker.Entries) != 1 {
		t.Fatalf("the player's tracker holds %d lines, want the one upstairs", len(tracker.Entries))
	}

	p, found := pawns[ari]
	if !found {
		t.Fatal("the pawn upstairs has no portrait on the player's strip")
	}
	if p.Image != "/assets/ari.webp" || p.Name != "Ari" {
		t.Fatalf("the pawn upstairs came back as %q with image %q", p.Name, p.Image)
	}
}

// THE PAWNS ARE PROJECTED AND NOT MERELY FILTERED. A monster's armour class is
// drawn by nothing, so it is withheld; its hit points are drawn by the canvas,
// so they are sent -- which is projectPawn's decision and this is the strip
// inheriting it rather than making it again.
func TestTheStripsPawnsGoThroughTheSameProjectionTheSocketDoes(t *testing.T) {
	w := newWorld(t)

	goblin := w.spawn(Pawn{Name: "Goblin", Visible: true, HP: intp(3), MaxHP: intp(7), AC: intp(15)})

	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}},
	}}, w.gm)

	_, pawns := w.s.ProjectedInitiative(RolePlayer)

	p := pawns[goblin]
	if p.AC != nil {
		t.Fatal("a player was handed a monster's armour class")
	}
	if p.HP == nil || *p.HP != 3 {
		t.Fatal("a player was not handed the hit points the canvas draws blood from")
	}
	if p.HPBand == nil || *p.HPBand != BandBloody {
		t.Fatalf("the band a player is given is %v, want bloody", p.HPBand)
	}

	_, gmPawns := w.s.ProjectedInitiative(RoleGM)
	if gm := gmPawns[goblin]; gm.AC == nil || *gm.AC != 15 {
		t.Fatal("the GM's copy lost the armour class")
	}
}

func entryNamesIn(in Initiative) []string {
	out := make([]string, 0, len(in.Entries))
	for _, e := range in.Entries {
		out = append(out, e.Name)
	}

	return out
}
