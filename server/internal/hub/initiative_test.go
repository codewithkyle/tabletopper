package hub

import (
	"testing"

	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)

// hub.Initiative IS THE SECOND DOOR INTO THE TURN ORDER, and the first is the
// socket. The strip over the table is a fragment fetched over HTTP by everybody
// in the room, so what comes back through here has to be exactly what the
// socket would have sent that role -- which is why the tracker and the pawns it
// names are computed in one pass rather than fetched twice.

// THE SECURITY TEST, AND IT IS FIRST. A hidden creature has no line in a
// player's turn order and no portrait in the map beside it.
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

	// AND THE PAWNS THAT DO ARRIVE ARE PROJECTED, not merely filtered: a
	// monster's armour class is drawn by nothing and is withheld, and its hit
	// points are drawn by the canvas and are sent.
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

// THE TABLE COMES WITH IT, because the strip prints hit points through the
// room's label setting and a second read would be quoting a setting from
// another instant.
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

// AN EMPTY TRACKER IS AN ANSWER AND NOT A FAILURE. The strip is fetched on
// every page load, and a room with no fight on is the common case.
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
