package controllers

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"tabletopper/internal/room"
	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

// WHAT IS ON THE TABLE. What these check is the seam, the way room-table_test
// does: that the handlers establish the right actor and hand every rule to
// internal/room, that the fragments are refused to the people they are not for,
// and that the one piece of logic these routes own -- the hit-point arithmetic
// -- is correct.

var (
	testPawnA = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVTD")
	testPawnB = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVTE")
)

func gmSession() session.UserSession {
	return session.UserSession{UserID: testOwnerID, Hash: []byte("session-hash")}
}

// removePath puts the ids in the QUERY STRING, which is where htmx puts hx-vals
// for a DELETE: its own source tests /GET|DELETE/ against the method and
// appends to the URL for both. A test that sent them as a body would be testing
// a request no browser makes -- and would pass or fail on net/http's rule that
// ParseForm reads a body only for POST, PUT and PATCH.
func removePath(values url.Values) string {
	return "/rooms/" + testRoomID.String() + "/pawns?" + values.Encode()
}

// THE ARITHMETIC IS THE ONE THING THESE ROUTES DECIDE FOR THEMSELVES, and it is
// the difference between a GM typing what they would say out loud -- "the
// goblin takes 7" -- and a GM doing subtraction in their head every round.
//
// AN UNSIGNED NUMBER IS AN ABSOLUTE VALUE. "7" in a box showing 12 means seven,
// not nineteen: somebody setting a monster's hit points reads a number off a
// sheet, and somebody applying damage types the minus sign.
func TestHitPointArithmetic(t *testing.T) {
	twelve := 12

	cases := []struct {
		entry string
		from  *int
		want  int
		bad   bool
	}{
		{entry: "12", from: &twelve, want: 12},
		{entry: "0", from: &twelve, want: 0},
		{entry: "-7", from: &twelve, want: 5},
		{entry: "+3", from: &twelve, want: 15},
		{entry: "  -7  ", from: &twelve, want: 5},

		// Past zero is left to the core, which clamps against the max hit
		// points it holds rather than the ones a request happened to carry.
		{entry: "-99", from: &twelve, want: -87},

		// A pawn whose hit points were projected away cannot be edited by the
		// person who cannot see them, but the arithmetic still has to have an
		// answer rather than a panic.
		{entry: "+3", from: nil, want: 3},

		{entry: "", from: &twelve, bad: true},
		{entry: "lots", from: &twelve, bad: true},
		{entry: "7hp", from: &twelve, bad: true},
		{entry: "--7", from: &twelve, bad: true},
	}

	for _, tc := range cases {
		got, bad := evaluateHP(tc.entry, tc.from)
		if (bad != "") != tc.bad {
			t.Errorf("evaluateHP(%q) refusal = %q, want bad = %v", tc.entry, bad, tc.bad)

			continue
		}
		if !tc.bad && got != tc.want {
			t.Errorf("evaluateHP(%q) = %d, want %d", tc.entry, got, tc.want)
		}
	}
}

// THE SPAWN DIALOG IS THE GM'S MANUAL AND THEIR LIBRARY. A player who fetched
// one would be handed every monster the GM has written up, which is the whole
// of next week's session.
func TestTheSpawnFragmentsAreTheGMsAlone(t *testing.T) {
	for name, handler := range map[string]func(*App) http.HandlerFunc{
		"spawn":      func(a *App) http.HandlerFunc { return a.RoomSpawnFragment },
		"spawn-list": func(a *App) http.HandlerFunc { return a.RoomSpawnListFragment },
	} {
		t.Run(name, func(t *testing.T) {
			app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})

			rec := tableRequest(t, handler(app), http.MethodGet,
				"/fragment/room/"+name+"?room="+testRoomID.String()+"&kind=monsters",
				nil, nil, memberSession(testRoomID))

			if rec.Code != http.StatusNotFound {
				t.Errorf("a player got %d, want 404", rec.Code)
			}
		})
	}
}

// THE KIND IS MATCHED AGAINST THE TWO VALUES BEFORE ANYTHING REACHES A
// STATEMENT, which is the rule every kind-parameterised route in this app
// follows. A bad one is an empty 404 rather than http.NotFound, which writes a
// page-shaped body into a fragment slot.
func TestTheSpawnFragmentRefusesAKindItDoesNotKnow(t *testing.T) {
	for _, kind := range []string{"", "avatars", "maps", "monsters; DROP"} {
		app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})

		rec := tableRequest(t, app.RoomSpawnFragment, http.MethodGet,
			"/fragment/room/spawn?room="+testRoomID.String()+"&kind="+url.QueryEscape(kind),
			nil, nil, gmSession())

		if rec.Code != http.StatusNotFound {
			t.Errorf("kind %q got %d, want 404", kind, rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("kind %q answered with a body: %s", kind, rec.Body.String())
		}
	}
}

// THE STAT BLOCK IS THE GM'S. The room's monster-health setting exists so a
// table can hide a monster's hit points; a stat block carries those, its armour
// class and its legendary actions, so serving one to a player would contradict
// in one window the setting the GM chose in another.
func TestTheStatBlockRouteRefusesAPlayer(t *testing.T) {
	app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})

	rec := tableRequest(t, app.RoomStatBlockFragment, http.MethodGet,
		"/fragment/room/stat-block?room="+testRoomID.String()+"&pawn="+testPawnA.String(),
		nil, nil, memberSession(testRoomID))

	if rec.Code != http.StatusNotFound {
		t.Errorf("a player got %d from the stat block, want 404", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("the refusal carried a body: %s", rec.Body.String())
	}
}

// A pawn nobody spawned is the same empty 404 a hidden one is, which is the
// property the whole projection rests on: the two must be indistinguishable.
func TestThePawnFragmentIsAnEmpty404ForAPawnThatIsNotThere(t *testing.T) {
	app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})

	rec := tableRequest(t, app.RoomPawnFragment, http.MethodGet,
		"/fragment/room/pawn?room="+testRoomID.String()+"&pawn="+testPawnA.String(),
		nil, nil, gmSession())

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("the refusal carried a body: %s", rec.Body.String())
	}
}

// THE HANDLER AUTHORISES NOTHING AND THAT IS THE POINT, which is the rule
// room-table_test pins for the layer routes. A player posting here is a member
// of the room, so a 404 would be a lie; PawnRemove.Authorize refuses them and
// rejectCommand turns that into the alert modal with the protocol's own
// sentence in it.
func TestRemovingPawnsIsRefusedByTheCommandAndNotByTheHandler(t *testing.T) {
	app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})

	rec := tableRequest(t, app.RemovePawns, http.MethodDelete,
		removePath(url.Values{"ids": {testPawnA.String()}}),
		map[string]string{"id": testRoomID.String()},
		nil, memberSession(testRoomID))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}

	trigger := rec.Header().Get("HX-Trigger")
	if !strings.Contains(trigger, "alert") {
		t.Errorf("no alert on the refusal: %q", trigger)
	}
	if !strings.Contains(trigger, "remove pawns") {
		t.Errorf("the alert does not say what was refused: %q", trigger)
	}
}

// The same rule for the floor, and the reason it is GM-only is not a matter of
// taste: a player who sent their own pawn upstairs would stop being sent it.
func TestMovingPawnsBetweenLayersIsRefusedForAPlayer(t *testing.T) {
	app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})
	layer := firstLayer(t, app)

	rec := tableRequest(t, app.MovePawnsToLayer, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/pawns/layer",
		map[string]string{"id": testRoomID.String()},
		url.Values{"ids": {testPawnA.String()}, "layer": {layer.String()}}, memberSession(testRoomID))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}
}

// EVERY ID REACHES THE CORE, which is what makes one command with a list the
// right shape rather than a loop of commands. The GM here names two pawns that
// are not on the table, and the refusal that comes back is the CORE's not-found
// rather than the handler's -- so the list was parsed, bounded and dispatched
// whole.
func TestEveryIDReachesTheCore(t *testing.T) {
	for name, handler := range map[string]struct {
		fn     func(*App) http.HandlerFunc
		method string
		form   func(layer ulid.ULID) url.Values
		query  func(layer ulid.ULID) url.Values
	}{
		"remove": {
			fn:     func(a *App) http.HandlerFunc { return a.RemovePawns },
			method: http.MethodDelete,
			query: func(ulid.ULID) url.Values {
				return url.Values{"ids": {testPawnA.String(), testPawnB.String()}}
			},
		},
		"layer": {
			fn:     func(a *App) http.HandlerFunc { return a.MovePawnsToLayer },
			method: http.MethodPost,
			form: func(layer ulid.ULID) url.Values {
				return url.Values{"ids": {testPawnA.String(), testPawnB.String()}, "layer": {layer.String()}}
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})
			layer := firstLayer(t, app)

			path := "/rooms/" + testRoomID.String() + "/pawns"
			var form url.Values
			if handler.query != nil {
				path = removePath(handler.query(layer))
			} else {
				form = handler.form(layer)
			}

			rec := tableRequest(t, handler.fn(app), handler.method,
				path, map[string]string{"id": testRoomID.String()}, form, gmSession())

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404 from the core; body: %s", rec.Code, rec.Body.String())
			}
			if trigger := rec.Header().Get("HX-Trigger"); !strings.Contains(trigger, "alert") {
				t.Errorf("the core's refusal did not reach the alert modal: %q", trigger)
			}
		})
	}
}

// THE LIST IS BOUNDED BEFORE IT IS PARSED, because the protocol's own limit is
// what a selection may hold and a request naming ten thousand pawns is not a
// browser this server wrote.
func TestTheIDListIsBoundedAndValidated(t *testing.T) {
	tooMany := make([]string, room.SelectionMax+1)
	for i := range tooMany {
		tooMany[i] = testPawnA.String()
	}

	cases := map[string]url.Values{
		"no ids at all":                  {},
		"an empty id":                    {"ids": {""}},
		"an id that is not a ULID":       {"ids": {"goblin"}},
		"more than a selection may hold": {"ids": tooMany},
	}

	for name, form := range cases {
		t.Run(name, func(t *testing.T) {
			app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})

			rec := tableRequest(t, app.RemovePawns, http.MethodDelete,
				removePath(form), map[string]string{"id": testRoomID.String()}, nil, gmSession())

			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404", rec.Code)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("answered with a body: %s", rec.Body.String())
			}
		})
	}
}

// SPAWNING THE PARTY IS THE GM'S, and the refusal comes from the command rather
// than from the handler for the reason every other route here gives.
func TestSpawningThePartyIsRefusedForAPlayer(t *testing.T) {
	app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})

	rec := tableRequest(t, app.SpawnParty, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/pawns/party",
		map[string]string{"id": testRoomID.String()}, url.Values{}, memberSession(testRoomID))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}

	// THE REFUSAL IS THE ALERT AND NOTHING ELSE. This route answers a menu item
	// rather than a dialog, so there is no modal to dismiss on the way out --
	// and a modal:close in the same header would clobber the alert, which is
	// the whole reason internal/htmx owns HX-Trigger rather than the handlers.
	trigger := rec.Header().Get("HX-Trigger")
	if !strings.Contains(trigger, "Only the GM") {
		t.Errorf("the refusal did not reach the alert modal: %q", trigger)
	}
	if strings.Contains(trigger, "modal:close") {
		t.Errorf("a refused spawn closed a dialog: %q", trigger)
	}
}

// CLEARING THE TABLETOP IS THE GM'S AND THE ROUTE IS NOT THE RULE. The menu
// item is disabled for a player, which is a courtesy to somebody who cannot
// press it; the refusal is TableClear.Authorize, which runs against a role
// derived from the rooms row whether or not a button was drawn.
func TestClearingTheTabletopIsRefusedForAPlayer(t *testing.T) {
	app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})

	rec := tableRequest(t, app.ClearTabletop, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/tabletop/clear",
		map[string]string{"id": testRoomID.String()}, url.Values{}, memberSession(testRoomID))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}
	if trigger := rec.Header().Get("HX-Trigger"); !strings.Contains(trigger, "Only the GM") {
		t.Errorf("the refusal did not reach the alert modal: %q", trigger)
	}
}
