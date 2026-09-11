package hub

import (
	"testing"

	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)









func TestInitiativeIsProjectedForTheRoleThatAsksForIt(t *testing.T) {
	tb := newTabletop(t, Options{})

	gm := tb.join(gmID, "Kyle", room.RoleGM)
	player := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, player)

	seen := tb.spawnMonster(gm, "Goblin", true, 4, 10)
	hidden := tb.spawnMonster(gm, "Ambusher", false, 9, 10)

	tb.send(gm, "order", &room.InitiativeSet{Entries: []room.InitiativeEntry{
		{Name: "Goblin", PawnIDs: []ulid.ULID{seen}},
		{Name: "Ambusher", PawnIDs: []ulid.ULID{hidden}},
	}})
	frames(t, gm)
	frames(t, player)

	view, ok := tb.Initiative(tb.ctx(), roomID, room.RoleGM)
	if !ok {
		t.Fatal("the GM was shown nothing of the turn order")
	}
	if len(view.Initiative.Entries) != 2 || len(view.Pawns) != 2 {
		t.Fatalf("the GM's strip has %d lines and %d pawns, want 2 of each",
			len(view.Initiative.Entries), len(view.Pawns))
	}

	view, ok = tb.Initiative(tb.ctx(), roomID, room.RolePlayer)
	if !ok {
		t.Fatal("the player was shown nothing of the turn order")
	}
	if len(view.Initiative.Entries) != 1 {
		t.Fatalf("the player's strip has %d lines, want the one they have met", len(view.Initiative.Entries))
	}
	if _, found := view.Pawns[hidden]; found {
		t.Fatal("the hidden pawn's portrait reached the player's strip")
	}

	
	
	
	p, found := view.Pawns[seen]
	if !found {
		t.Fatal("the visible pawn has no portrait on the player's strip")
	}
	if p.AC != nil {
		t.Fatal("a player was handed a monster's armour class through the strip")
	}
	if p.HP == nil {
		t.Fatal("a player was not handed the hit points the strip draws blood from")
	}
}




func TestInitiativeAnswersWithTheTableItProjectedAgainst(t *testing.T) {
	tb := newTabletop(t, Options{})

	gm := tb.join(gmID, "Kyle", room.RoleGM)
	frames(t, gm)

	tb.send(gm, "opt", &room.TableSetOptions{
		PawnLabels:         room.LabelsFull,
		PlayersCanDraw:     true,
		InitiativeGrouping: room.GroupIndividual,
	})
	frames(t, gm)

	view, ok := tb.Initiative(tb.ctx(), roomID, room.RoleGM)
	if !ok {
		t.Fatal("the room would not answer with its turn order")
	}
	if view.Table.PawnLabels != room.LabelsFull {
		t.Fatalf("the strip was handed the %q setting", view.Table.PawnLabels)
	}
	if view.Table.InitiativeGrouping != room.GroupIndividual {
		t.Fatalf("the strip was handed the %q grouping", view.Table.InitiativeGrouping)
	}
}



func TestInitiativeAnswersAnEmptyTracker(t *testing.T) {
	tb := newTabletop(t, Options{})

	view, ok := tb.Initiative(tb.ctx(), roomID, room.RolePlayer)
	if !ok {
		t.Fatal("a room with no fight on would not answer")
	}
	if len(view.Initiative.Entries) != 0 || len(view.Pawns) != 0 {
		t.Fatal("an empty tracker came back with something in it")
	}
}
