package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"tabletopper/internal/hub"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

// THE TURN ORDER'S ROUTES. What these check is the seam, the way the table's
// tests do: that each handler establishes the right actor, reads the live
// tracker, hands every rule to internal/room, and refuses what it should refuse
// -- plus the one piece of logic these routes own, which is turning a dropped
// line back into a whole order.

// initiativeApp is a live room with the GM and one player seated and a fight in
// progress: two lines, and the entries behind them.
//
// THE LINES ARE NAMED RATHER THAN CREATURES, and that is not a shortcut around
// the interesting case -- it is what keeps these tests about the seam. Putting
// a pawn on the table goes through the hub's resolver, which reads the GM's
// manual out of the database; what these routes do with a line is the same
// whether it names nine goblins or a lair action, and what they do with the
// PAWNS is internal/room's and is tested there.
func initiativeApp(t *testing.T) (*App, []room.InitiativeEntry) {
	t.Helper()

	answers := make([]roomAnswer, 0, roomReads)
	for range roomReads {
		answers = append(answers, getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false))
	}

	app := liveRoomApp(t, &roomDB{rows: 1, answers: answers})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	gm := room.Actor{ID: testOwnerID, Role: room.RoleGM}
	set := &room.InitiativeSet{Entries: []room.InitiativeEntry{
		{Name: "Lair action"},
		{Name: "The volcano erupts"},
	}}
	if err := app.Hub.Dispatch(ctx, testRoomID, gm, set); err != nil {
		t.Fatalf("building the order: %v", err)
	}

	view, ok := app.Hub.Initiative(ctx, testRoomID, room.RoleGM)
	if !ok {
		t.Fatal("the room would not answer with its turn order")
	}
	if len(view.Initiative.Entries) != 2 {
		t.Fatalf("the fixture built %d lines, want 2", len(view.Initiative.Entries))
	}

	return app, view.Initiative.Entries
}

// roomReads is how many GetRoom answers one of these tests can consume. Every
// route in this file establishes the room once and the fake answers them in
// order, so the queue only has to be longer than the longest test.
const roomReads = 12

// tracker reads the live order back out.
func tracker(t *testing.T, app *App) room.Initiative {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	view, ok := app.Hub.Initiative(ctx, testRoomID, room.RoleGM)
	if !ok {
		t.Fatal("the room would not answer with its turn order")
	}

	return view.Initiative
}

func initiativeRequest(t *testing.T, handler http.HandlerFunc, method, path string, values map[string]string, form url.Values, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()

	values["id"] = testRoomID.String()

	return tableRequest(t, handler, method, path, values, form, sess)
}

// THE STRIP IS EVERYBODY'S. It is the one live surface on the room page that a
// player fetches about the fight, and hub.Initiative is what makes that safe.
func TestTheStripIsFetchedByAnybodyInTheRoom(t *testing.T) {
	app, _ := initiativeApp(t)

	for name, sess := range map[string]session.UserSession{
		"the GM":   {UserID: testOwnerID},
		"a player": memberSession(testRoomID),
	} {
		rec := tableRequest(t, app.RoomInitiativeFragment, http.MethodGet,
			"/fragment/room/initiative?room="+testRoomID.String(), nil, nil, sess)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s got status %d, want 200; body: %s", name, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `id="room-initiative"`) {
			t.Errorf("%s was not sent the strip:\n%s", name, rec.Body.String())
		}
	}
}

// AND SO IS THE ROUND COUNTER IN THE BAR. The round is the one thing about a
// fight that is not projected: a player who can see none of the monsters still
// knows which round it is, because their own character is in it.
func TestTheRoundCounterIsFetchedByAnybodyInTheRoom(t *testing.T) {
	app, _ := initiativeApp(t)

	for name, sess := range map[string]session.UserSession{
		"the GM":   {UserID: testOwnerID},
		"a player": memberSession(testRoomID),
	} {
		rec := tableRequest(t, app.RoomInitiativeRoundFragment, http.MethodGet,
			"/fragment/room/initiative/round?room="+testRoomID.String(), nil, nil, sess)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s got status %d, want 200; body: %s", name, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `id="room-initiative-round"`) {
			t.Errorf("%s was not sent the counter:\n%s", name, rec.Body.String())
		}

		// The fixture builds two lines and starts nobody's turn, so this is a
		// tracker that exists and has not been advanced: a dash, not a zero.
		if !strings.Contains(rec.Body.String(), "Round") {
			t.Errorf("%s was sent a counter with no round in it:\n%s", name, rec.Body.String())
		}
	}
}

// AND THE ADD DIALOG IS THE GM'S, like the layer manager: it is a control for
// the fight rather than a reading of it.
func TestTheAddEntryDialogIsTheGMsAlone(t *testing.T) {
	app, _ := initiativeApp(t)

	rec := tableRequest(t, app.RoomInitiativeEntryFragment, http.MethodGet,
		"/fragment/room/initiative/entry?room="+testRoomID.String(), nil, nil,
		memberSession(testRoomID))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("a player got status %d from the add dialog, want 404", rec.Code)
	}
}

// A DROP POSTS THE WHOLE ORDER, and the handler turns the ids back into the
// lines they name -- in the order they arrived.
func TestADropReordersTheTracker(t *testing.T) {
	app, entries := initiativeApp(t)

	form := url.Values{"entries": {entries[1].ID.String() + "," + entries[0].ID.String()}}
	rec := initiativeRequest(t, app.OrderInitiative, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/initiative/order", map[string]string{}, form,
		session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}

	got := tracker(t, app)
	if got.Entries[0].ID != entries[1].ID || got.Entries[1].ID != entries[0].ID {
		t.Fatal("the order did not follow the drop")
	}
}

// AN ID SET THAT IS NOT EXACTLY THE TRACKER'S IS REFUSED. The drag raced a
// change, and reordering what came back would put the tracker into a shape
// nobody asked for.
func TestADropThatRacedAChangeIsRefused(t *testing.T) {
	app, entries := initiativeApp(t)

	for name, value := range map[string]string{
		"a line that is missing":            entries[0].ID.String(),
		"a line that is not in the tracker": entries[0].ID.String() + "," + testPawnA.String(),
		"the same line twice":               entries[0].ID.String() + "," + entries[0].ID.String(),
	} {
		rec := initiativeRequest(t, app.OrderInitiative, http.MethodPost,
			"/rooms/"+testRoomID.String()+"/initiative/order", map[string]string{},
			url.Values{"entries": {value}}, session.UserSession{UserID: testOwnerID})

		if rec.Code != http.StatusNotFound {
			t.Errorf("%s got status %d, want 404", name, rec.Code)
		}
	}
}

// CLICKING A LINE GIVES THAT CREATURE THE TURN, and the GM may do it to a
// corpse: skipping the dead is what the button does, not a rule about what may
// be acting.
func TestClickingALineGivesItTheTurn(t *testing.T) {
	app, entries := initiativeApp(t)

	rec := initiativeRequest(t, app.ActivateInitiative, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/initiative/"+entries[1].ID.String()+"/activate",
		map[string]string{"entry": entries[1].ID.String()}, nil,
		session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}

	got := tracker(t, app)
	if got.Active == nil || *got.Active != entries[1].ID {
		t.Fatal("the turn did not move to the line that was pressed")
	}
}

// REMOVING THE ACTING LINE MOVES THE TURN TO THE NEXT ONE IN THE OLD ORDER,
// because "the next combatant" is a fact about the order the GM built.
func TestRemovingTheActingLineMovesTheTurnOn(t *testing.T) {
	app, entries := initiativeApp(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := app.Hub.Dispatch(ctx, testRoomID, room.Actor{ID: testOwnerID, Role: room.RoleGM}, &room.InitiativeNext{}); err != nil {
		t.Fatalf("starting the fight: %v", err)
	}

	rec := initiativeRequest(t, app.RemoveInitiative, http.MethodDelete,
		"/rooms/"+testRoomID.String()+"/initiative/"+entries[0].ID.String(),
		map[string]string{"entry": entries[0].ID.String()}, nil,
		session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}

	got := tracker(t, app)
	if len(got.Entries) != 1 {
		t.Fatalf("the tracker holds %d lines, want 1", len(got.Entries))
	}
	if got.Active == nil || *got.Active != entries[1].ID {
		t.Fatal("the turn did not move on to the next line in the old order")
	}
}

// AND REMOVING THE LAST LINE LEAVES NOTHING RATHER THAN A TRACKER POINTING AT
// A LINE THAT HAS GONE.
func TestRemovingTheLastLineEmptiesTheTracker(t *testing.T) {
	app, entries := initiativeApp(t)

	for _, e := range entries {
		rec := initiativeRequest(t, app.RemoveInitiative, http.MethodDelete,
			"/rooms/"+testRoomID.String()+"/initiative/"+e.ID.String(),
			map[string]string{"entry": e.ID.String()}, nil,
			session.UserSession{UserID: testOwnerID})

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
		}
	}

	got := tracker(t, app)
	if len(got.Entries) != 0 || got.Active != nil {
		t.Fatalf("the emptied tracker is %v with %v acting", got.Entries, got.Active)
	}
}

// ADD TAKES A NAME OR A PAWN AND NEVER BOTH. A named line is a lair action and
// has no creature; a pawn line is a creature and takes its name from the pawn.
func TestAddTakesANameOrAPawnAndNotBoth(t *testing.T) {
	app, _ := initiativeApp(t)

	rec := initiativeRequest(t, app.AddInitiative, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/initiative", map[string]string{},
		url.Values{"name": {"Lair action"}}, session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("adding a name gave status %d; body: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("HX-Trigger") == "" {
		t.Error("the dialog was not told to close")
	}

	got := tracker(t, app)
	if len(got.Entries) != 3 || got.Entries[2].Name != "Lair action" {
		t.Fatalf("the named line did not land at the end: %v", got.Entries)
	}
	if len(got.Entries[2].PawnIDs) != 0 {
		t.Fatal("the named line names a pawn")
	}

	rec = initiativeRequest(t, app.AddInitiative, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/initiative", map[string]string{},
		url.Values{"name": {"Both"}, "pawn": {testPawnA.String()}},
		session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("a request carrying both gave status %d, want 404", rec.Code)
	}
}

// A PLAYER MAY NOT EDIT THE ORDER, and the core is what refuses them rather
// than the mux.
func TestAPlayerMayNotEditTheOrder(t *testing.T) {
	app, entries := initiativeApp(t)

	rec := initiativeRequest(t, app.ActivateInitiative, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/initiative/"+entries[0].ID.String()+"/activate",
		map[string]string{"entry": entries[0].ID.String()}, nil,
		memberSession(testRoomID))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}
}

// AND A PLAYER WHOSE TURN IT IS NOT MAY NOT END IT. Next is the one route here
// that is not the GM's alone, and the core decides who may press it.
func TestAPlayerOutOfTurnMayNotAdvance(t *testing.T) {
	app, _ := initiativeApp(t)

	rec := initiativeRequest(t, app.NextInitiative, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/initiative/next", map[string]string{}, nil,
		memberSession(testRoomID))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}
}

// WHERE A PAWN GOES WHEN THE GM PRESSES ADD TO INITIATIVE. It is the same rule
// Sync applies -- a monster joins the line whose members share its key -- and it
// is exercised here rather than through the route because putting a pawn on the
// table goes through the hub's resolver and its database.
// WHAT COLOURS A CARD'S FRAME IS WHAT THE CREATURE IS. The row answers "how
// much of this is trying to kill us" before anybody reads a word, and the words
// are room.PawnKind's own so there is no table between the protocol and the
// attribute the stylesheet keys on.
func TestACardsSideIsItsCreaturesKind(t *testing.T) {
	entry := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVV0")
	pawn := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVV1")
	second := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVV2")

	for _, kind := range []room.PawnKind{room.PawnPlayer, room.PawnMonster, room.PawnNPC} {
		view := &hub.InitiativeView{Pawns: map[ulid.ULID]room.Pawn{
			pawn: {ID: pawn, Kind: kind, Name: "Somebody"},
		}}

		got := initiativeEntryData(room.RoleGM, ulid.ULID{}, view,
			room.InitiativeEntry{ID: entry, PawnIDs: []ulid.ULID{pawn}, Name: "Somebody"})

		if got.Side != string(kind) {
			t.Errorf("a %s is drawn on the %q side", kind, got.Side)
		}
	}

	// A GROUP READS IT OFF ITS FIRST MEMBER, which is safe because grouping
	// only ever puts monsters together.
	view := &hub.InitiativeView{Pawns: map[ulid.ULID]room.Pawn{
		pawn:   {ID: pawn, Kind: room.PawnMonster, Name: "Goblin"},
		second: {ID: second, Kind: room.PawnMonster, Name: "Goblin"},
	}}

	got := initiativeEntryData(room.RoleGM, ulid.ULID{}, view,
		room.InitiativeEntry{ID: entry, PawnIDs: []ulid.ULID{pawn, second}, Name: "Goblin"})

	if got.Kind != pages.EntryGroup || got.Side != pages.SideMonster {
		t.Errorf("a group of goblins is a %q on the %q side", got.Kind, got.Side)
	}

	// AND A LINE WITH NO CREATURE IS ON NOBODY'S SIDE. A lair action takes the
	// neutral frame.
	lair := initiativeEntryData(room.RoleGM, ulid.ULID{}, &hub.InitiativeView{},
		room.InitiativeEntry{ID: entry, Name: "Lair action"})

	if lair.Side != "" {
		t.Errorf("a lair action was put on the %q side", lair.Side)
	}
}

func TestAddPutsAMonsterInItsGroup(t *testing.T) {
	manual := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVU1")
	first := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVU2")
	second := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVU3")
	ogre := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVU4")
	line := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVU5")

	goblin := func(id ulid.ULID) room.Pawn {
		return room.Pawn{ID: id, Kind: room.PawnMonster, Name: "Goblin", MonsterID: &manual}
	}

	view := func(grouping room.InitiativeGrouping) *hub.InitiativeView {
		return &hub.InitiativeView{
			Pawns: map[ulid.ULID]room.Pawn{
				first:  goblin(first),
				second: goblin(second),
				ogre:   {ID: ogre, Kind: room.PawnMonster, Name: "Ogre"},
			},
			Table: room.Table{InitiativeGrouping: grouping},
		}
	}

	held := []room.InitiativeEntry{{ID: line, PawnIDs: []ulid.ULID{first}, Name: "Goblin"}}

	// GROUPED: the second goblin joins the first one's line rather than taking
	// a turn of its own.
	got, ok := withPawn(append([]room.InitiativeEntry(nil), held...), view(room.GroupMonsters), second)
	if !ok {
		t.Fatal("adding a second goblin was refused")
	}
	if len(got) != 1 || len(got[0].PawnIDs) != 2 {
		t.Fatalf("the second goblin made %d lines holding %v", len(got), got)
	}

	// A DIFFERENT MONSTER IS A DIFFERENT LINE, which is what the key is for.
	got, _ = withPawn(append([]room.InitiativeEntry(nil), held...), view(room.GroupMonsters), ogre)
	if len(got) != 2 || got[1].Name != "Ogre" {
		t.Fatalf("the ogre did not get a line of its own: %v", got)
	}

	// AND INDIVIDUAL IS ALWAYS A NEW LINE, because that is what the setting
	// says the fight is being run as.
	got, _ = withPawn(append([]room.InitiativeEntry(nil), held...), view(room.GroupIndividual), second)
	if len(got) != 2 {
		t.Fatalf("the second goblin was grouped in individual mode: %v", got)
	}

	// A CREATURE THAT IS ALREADY IN THE ORDER IS REFUSED, and the route turns
	// that into a sentence rather than a second turn.
	if _, ok := withPawn(append([]room.InitiativeEntry(nil), held...), view(room.GroupMonsters), first); ok {
		t.Fatal("a pawn already in the order was added again")
	}
}
