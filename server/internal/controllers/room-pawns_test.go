package controllers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"tabletopper/internal/room"
	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)







var (
	testPawnA = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVTD")
	testPawnB = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVTE")
)

func gmSession() session.UserSession {
	return session.UserSession{UserID: testOwnerID, Hash: []byte("session-hash")}
}






func removePath(values url.Values) string {
	return "/rooms/" + testRoomID.String() + "/pawns?" + values.Encode()
}

















func TestARelativeEntryIsAppliedToTheRoomsNumberAndNotTheBoxes(t *testing.T) {
	form := url.Values{"hp": {"12"}, "hpEntry": {"-4"}}
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if got := hpEntry(r, "hp"); got != "-4" {
		t.Errorf("hpEntry = %q, want the change the twin carried", got)
	}

	
	ten := 10
	value, present, refusal := evaluateHP(hpEntry(r, "hp"), &ten, "Hit points")
	if refusal != "" || !present || value != 6 {
		t.Errorf("evaluateHP(-4 against 10) = %d, %v, %q; want 6", value, present, refusal)
	}

	
	
	form = url.Values{"hp": {"12"}, "hpEntry": {""}}
	r = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if got := hpEntry(r, "hp"); got != "12" {
		t.Errorf("hpEntry = %q, want the box", got)
	}

	
	if _, _, refusal := evaluateHP("lots", &ten, "Hit points"); !strings.HasPrefix(refusal, "Hit points ") {
		t.Errorf("refusal = %q, want it to name the box", refusal)
	}
}




func TestTheSpawnFragmentsAreTheGMsAlone(t *testing.T) {
	for name, handler := range map[string]func(*App) http.HandlerFunc{
		"spawn":      func(a *App) http.HandlerFunc { return a.RoomSpawnFragment },
		"spawn-list": func(a *App) http.HandlerFunc { return a.RoomSpawnListFragment },
		"spawn-npc":  func(a *App) http.HandlerFunc { return a.RoomSpawnNPCFragment },
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





func TestTheSpawnAddRoutesAreTheGMsAlone(t *testing.T) {
	for name, handler := range map[string]func(*App) http.HandlerFunc{
		"tokens":   func(a *App) http.HandlerFunc { return a.UploadSpawnToken },
		"avatars":  func(a *App) http.HandlerFunc { return a.UploadSpawnAvatar },
		"monsters": func(a *App) http.HandlerFunc { return a.CreateSpawnMonster },
	} {
		t.Run(name, func(t *testing.T) {
			app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})

			rec := tableRequest(t, handler(app), http.MethodPost,
				"/rooms/"+testRoomID.String()+"/spawn/"+name,
				map[string]string{"id": testRoomID.String()}, url.Values{"name": {"Goblin"}},
				memberSession(testRoomID))

			if rec.Code != http.StatusNotFound {
				t.Errorf("a player got %d, want 404", rec.Code)
			}
		})
	}
}






func TestTheQuickMonsterFormRefusesAndWritesNothing(t *testing.T) {
	cases := map[string]struct {
		form url.Values
		want string
	}{
		"no name":    {form: url.Values{"hp": {"7"}}, want: "Name is required."},
		"no picture": {form: url.Values{"name": {"Goblin"}}, want: "A picture is required."},
		"silly hp":   {form: url.Values{"name": {"Goblin"}, "hp": {"70000"}}, want: "Hit points"},
		"silly ac":   {form: url.Values{"name": {"Goblin"}, "ac": {"200"}}, want: "Armour class"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
			app := tableApp(t, db)

			rec := tableRequest(t, app.CreateSpawnMonster, http.MethodPost,
				"/rooms/"+testRoomID.String()+"/spawn/monsters",
				map[string]string{"id": testRoomID.String()}, tc.form, gmSession())

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422: %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Errorf("the refusal does not mention %q:\n%s", tc.want, rec.Body.String())
			}
			
			
			for _, query := range db.queries() {
				if strings.Contains(query, "INSERT") || strings.Contains(query, "UPDATE") {
					t.Errorf("a refused form still wrote: %s", query)
				}
			}
		})
	}
}




func TestTheNPCFragmentRefusesAnAssetItCannotName(t *testing.T) {
	for _, asset := range []string{"", "not-a-ulid", "01BX5ZZKBKACTAV9WEVGEMMVT0; DROP"} {
		app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})

		rec := tableRequest(t, app.RoomSpawnNPCFragment, http.MethodGet,
			"/fragment/room/spawn-npc?room="+testRoomID.String()+"&asset="+url.QueryEscape(asset),
			nil, nil, gmSession())

		if rec.Code != http.StatusNotFound {
			t.Errorf("asset %q got %d, want 404", asset, rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("asset %q answered with a body: %s", asset, rec.Body.String())
		}
	}
}





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
		"shown": {
			fn:     func(a *App) http.HandlerFunc { return a.SetPawnsShown },
			method: http.MethodPost,
			form: func(ulid.ULID) url.Values {
				return url.Values{"ids": {testPawnA.String(), testPawnB.String()}, "shown": {"on"}}
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




func TestSettingPawnVisibilityIsRefusedForAPlayer(t *testing.T) {
	app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})

	rec := tableRequest(t, app.SetPawnsShown, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/pawns/shown",
		map[string]string{"id": testRoomID.String()},
		url.Values{"ids": {testPawnA.String()}, "shown": {"on"}}, memberSession(testRoomID))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}
	if trigger := rec.Header().Get("HX-Trigger"); !strings.Contains(trigger, "hide or reveal pawns") {
		t.Errorf("the alert does not say what was refused: %q", trigger)
	}
}



func TestSpawningThePartyIsRefusedForAPlayer(t *testing.T) {
	app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})

	rec := tableRequest(t, app.SpawnParty, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/pawns/party",
		map[string]string{"id": testRoomID.String()}, url.Values{}, memberSession(testRoomID))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}

	
	
	
	
	trigger := rec.Header().Get("HX-Trigger")
	if !strings.Contains(trigger, "Only the GM") {
		t.Errorf("the refusal did not reach the alert modal: %q", trigger)
	}
	if strings.Contains(trigger, "modal:close") {
		t.Errorf("a refused spawn closed a dialog: %q", trigger)
	}
}





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






func TestThePawnPanelPrintsNumbersOnlyWhereTheSettingAllows(t *testing.T) {
	hp, maxHP, ac := 4, 10, 15
	band := room.BandBloody

	tests := []struct {
		name    string
		kind    room.PawnKind
		labels  room.PawnLabels
		role    room.Role
		banded  bool
		numbers string
	}{
		
		
		{"the GM in a room labelling words", room.PawnMonster, room.LabelsDefault, room.RoleGM, false, "4 / 10"},
		{"the GM in a room labelling nothing", room.PawnMonster, room.LabelsNone, room.RoleGM, false, "4 / 10"},

		
		
		{"a player in an open room", room.PawnMonster, room.LabelsFull, room.RolePlayer, false, "4 / 10"},
		{"a player in an ordinary room", room.PawnMonster, room.LabelsDefault, room.RolePlayer, true, ""},

		
		
		
		{"a player in a room labelling nothing", room.PawnMonster, room.LabelsNone, room.RolePlayer, false, ""},

		
		
		{"a player's own character", room.PawnPlayer, room.LabelsNone, room.RolePlayer, false, "4 / 10"},
		{"a door", room.PawnObject, room.LabelsNone, room.RolePlayer, false, "4 / 10"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pawn := &room.Pawn{Kind: tc.kind, Name: "Goblin", HP: &hp, MaxHP: &maxHP, AC: &ac}
			if tc.banded {
				pawn.HPBand = &band
			}

			view := pawnView(pawn, tc.role, tc.labels, "Ground floor")

			if view.HP != tc.numbers {
				t.Errorf("the panel reads %q, want %q", view.HP, tc.numbers)
			}

			
			
			
			if (view.HPValue != "") != (tc.numbers != "") {
				t.Errorf("the hit-point box reads %q beside a line of %q", view.HPValue, view.HP)
			}
			if (view.MaxHP != "") != (tc.numbers != "") {
				t.Errorf("the maximum box reads %q beside a line of %q", view.MaxHP, view.HP)
			}
		})
	}
}




func TestThePawnPanelFallsBackToWordsWhenTheRoomCannotBeRead(t *testing.T) {
	if got := tableLabels(nil); got != room.LabelsDefault {
		t.Fatalf("an unreadable room labels %q, want %q", got, room.LabelsDefault)
	}
}
