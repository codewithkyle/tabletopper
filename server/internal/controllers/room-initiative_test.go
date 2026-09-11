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

const roomReads = 12

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
		if !strings.Contains(rec.Body.String(), "Round") {
			t.Errorf("%s was sent a counter with no round in it:\n%s", name, rec.Body.String())
		}
	}
}
func TestTheAddEntryDialogIsTheGMsAlone(t *testing.T) {
	app, _ := initiativeApp(t)
	rec := tableRequest(t, app.RoomInitiativeEntryFragment, http.MethodGet,
		"/fragment/room/initiative/entry?room="+testRoomID.String(), nil, nil,
		memberSession(testRoomID))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("a player got status %d from the add dialog, want 404", rec.Code)
	}
}
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
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s got status %d, want 422 with the refusal in the alert", name, rec.Code)
		}
		if !strings.Contains(rec.Header().Get("HX-Trigger"), "Order out of date") {
			t.Errorf("%s: the refusal did not reach the alert: %q", name, rec.Header().Get("HX-Trigger"))
		}
	}
}
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
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a request carrying both gave status %d, want 422", rec.Code)
	}
}
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
func TestAPlayerOutOfTurnMayNotAdvance(t *testing.T) {
	app, _ := initiativeApp(t)
	rec := initiativeRequest(t, app.NextInitiative, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/initiative/next", map[string]string{}, nil,
		memberSession(testRoomID))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}
}
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
	view := &hub.InitiativeView{Pawns: map[ulid.ULID]room.Pawn{
		pawn:   {ID: pawn, Kind: room.PawnMonster, Name: "Goblin"},
		second: {ID: second, Kind: room.PawnMonster, Name: "Goblin"},
	}}
	got := initiativeEntryData(room.RoleGM, ulid.ULID{}, view,
		room.InitiativeEntry{ID: entry, PawnIDs: []ulid.ULID{pawn, second}, Name: "Goblin"})
	if got.Kind != pages.EntryGroup || got.Side != pages.SideMonster {
		t.Errorf("a group of goblins is a %q on the %q side", got.Kind, got.Side)
	}
	lair := initiativeEntryData(room.RoleGM, ulid.ULID{}, &hub.InitiativeView{},
		room.InitiativeEntry{ID: entry, Name: "Lair action"})
	if lair.Side != "" {
		t.Errorf("a lair action was put on the %q side", lair.Side)
	}
}
