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

func keyedApp(t *testing.T, revealed bool) (*App, ulid.ULID) {
	t.Helper()
	var layer ulid.ULID
	app := seededApp(t, func(s *room.State) {
		layer = s.Table.ActiveLayer
		s.Notes = []room.HexNote{{
			LayerID: layer, Q: 3, R: -2,
			Title: "Ruined tower", Body: "An owlbear nests on the top floor.",
			Revealed: revealed,
		}}
	})
	return app, layer
}
func hexRequest(t *testing.T, app *App, layer ulid.ULID, q, r string, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()
	query := url.Values{
		"room":  {testRoomID.String()},
		"layer": {layer.String()},
		"q":     {q},
		"r":     {r},
	}
	return tableRequest(t, app.HexNoteFragment, http.MethodGet, "/fragment/room/hex?"+query.Encode(), nil, nil, sess)
}
func TestEverybodyOpensAHexAndOnlyTheGMSeesWhatIsNotShared(t *testing.T) {
	for name, tc := range map[string]struct {
		revealed bool
		sess     session.UserSession
		reads    bool
		controls bool
	}{
		"the GM, hidden":   {false, session.UserSession{UserID: testOwnerID}, true, true},
		"the GM, shared":   {true, session.UserSession{UserID: testOwnerID}, true, true},
		"a player, hidden": {false, memberSession(testRoomID), false, false},
		"a player, shared": {true, memberSession(testRoomID), true, false},
	} {
		t.Run(name, func(t *testing.T) {
			app, layer := keyedApp(t, tc.revealed)
			query := url.Values{
				"room": {testRoomID.String()}, "layer": {layer.String()}, "q": {"3"}, "r": {"-2"},
			}
			rec := tableRequest(t, app.HexNoteFragment, http.MethodGet,
				"/fragment/room/hex?"+query.Encode(), nil, nil, tc.sess)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if !strings.Contains(body, `name="body"`) {
				t.Fatalf("the hex is not an editor:\n%s", body)
			}
			if got := strings.Contains(body, "Ruined tower"); got != tc.reads {
				t.Errorf("the note is on the page: %v, want %v:\n%s", got, tc.reads, body)
			}
			if got := strings.Contains(body, `name="revealed"`) || strings.Contains(body, "hx-delete="); got != tc.controls {
				t.Errorf("the sharing and rubbing out controls are there: %v, want %v", got, tc.controls)
			}
		})
	}
}
func TestAHexTheGMHasNotWrittenOnIsStillTheirsToOpen(t *testing.T) {
	app, layer := keyedApp(t, false)
	query := url.Values{"room": {testRoomID.String()}, "layer": {layer.String()}, "q": {"9"}, "r": {"9"}}
	rec := tableRequest(t, app.HexNoteFragment, http.MethodGet,
		"/fragment/room/hex?"+query.Encode(), nil, nil, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `name="body"`) {
		t.Errorf("an empty hex is not an editor:\n%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Ruined tower") {
		t.Error("the empty hex carries its neighbour's note")
	}
}
func TestAHexFragmentChecksTheCellItIsAskedFor(t *testing.T) {
	app, layer := keyedApp(t, true)
	for name, query := range map[string]url.Values{
		"no layer":                {"room": {testRoomID.String()}, "q": {"3"}, "r": {"-2"}},
		"a layer that is not one": {"room": {testRoomID.String()}, "layer": {"floor"}, "q": {"3"}, "r": {"-2"}},
		"q is not a number":       {"room": {testRoomID.String()}, "layer": {layer.String()}, "q": {"three"}, "r": {"-2"}},
		"r is missing":            {"room": {testRoomID.String()}, "layer": {layer.String()}, "q": {"3"}},
		"off the map":             {"room": {testRoomID.String()}, "layer": {layer.String()}, "q": {"3"}, "r": {"100001"}},
	} {
		t.Run(name, func(t *testing.T) {
			rec := tableRequest(t, app.HexNoteFragment, http.MethodGet,
				"/fragment/room/hex?"+query.Encode(), nil, nil, session.UserSession{UserID: testOwnerID})
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("a refused parameter answered with a body: %s", rec.Body.String())
			}
		})
	}
}
func TestSharingAndRubbingOutAHexAreTheGMsAlone(t *testing.T) {
	app, layer := keyedApp(t, true)
	cell := url.Values{"layer": {layer.String()}, "q": {"3"}, "r": {"-2"}}
	for name, tc := range map[string]struct {
		handler func(*App) http.HandlerFunc
		method  string
		form    url.Values
		path    string
	}{
		"share":   {func(a *App) http.HandlerFunc { return a.RevealHexNote }, http.MethodPost, url.Values{"layer": {layer.String()}, "q": {"3"}, "r": {"-2"}, "revealed": {"on"}}, "/rooms/" + testRoomID.String() + "/notes/reveal"},
		"rub out": {func(a *App) http.HandlerFunc { return a.RemoveHexNote }, http.MethodDelete, nil, "/rooms/" + testRoomID.String() + "/notes?" + cell.Encode()},
	} {
		t.Run(name, func(t *testing.T) {
			rec := tableRequest(t, tc.handler(app), tc.method, tc.path,
				map[string]string{"id": testRoomID.String()}, tc.form, memberSession(testRoomID))
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
			}
		})
	}
}
func TestTheGMWritesSharesAndRubsOutAHex(t *testing.T) {
	app, layer := keyedApp(t, false)
	gm := session.UserSession{UserID: testOwnerID}
	cell := url.Values{"layer": {layer.String()}, "q": {"3"}, "r": {"-2"}}
	rec := tableRequest(t, app.SetHexNote, http.MethodPost, "/rooms/"+testRoomID.String()+"/notes",
		map[string]string{"id": testRoomID.String()},
		url.Values{"layer": {layer.String()}, "q": {"3"}, "r": {"-2"}, "title": {"Ruined tower"}, "body": {"Owlbear gone."}}, gm)
	if rec.Code != http.StatusOK {
		t.Fatalf("write: status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `name="revealed"`) {
		t.Errorf("the first save did not bring back the controls it unlocks:\n%s", rec.Body.String())
	}
	rec = tableRequest(t, app.RevealHexNote, http.MethodPost, "/rooms/"+testRoomID.String()+"/notes/reveal",
		map[string]string{"id": testRoomID.String()},
		url.Values{"layer": {layer.String()}, "q": {"3"}, "r": {"-2"}, "revealed": {"on"}}, gm)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("share: status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	shared := hexRequest(t, app, layer, "3", "-2", memberSession(testRoomID))
	if !strings.Contains(shared.Body.String(), "Ruined tower") {
		t.Fatalf("the shared hex does not reach a player:\n%s", shared.Body.String())
	}
	written := tableRequest(t, app.SetHexNote, http.MethodPost, "/rooms/"+testRoomID.String()+"/notes",
		map[string]string{"id": testRoomID.String()},
		url.Values{"layer": {layer.String()}, "q": {"4"}, "r": {"0"}, "body": {"The ford is waist deep."}},
		memberSession(testRoomID))
	if written.Code != http.StatusOK {
		t.Fatalf("a player writing a hex: status = %d, want 200; body: %s", written.Code, written.Body.String())
	}
	if strings.Contains(written.Body.String(), `name="revealed"`) {
		t.Errorf("a player was handed the sharing toggle:\n%s", written.Body.String())
	}
	rec = tableRequest(t, app.RemoveHexNote, http.MethodDelete,
		"/rooms/"+testRoomID.String()+"/notes?"+cell.Encode(),
		map[string]string{"id": testRoomID.String()}, nil, gm)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("rub out: status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	query := url.Values{"room": {testRoomID.String()}, "layer": {layer.String()}, "q": {"3"}, "r": {"-2"}}
	gone := tableRequest(t, app.HexNoteFragment, http.MethodGet,
		"/fragment/room/hex?"+query.Encode(), nil, nil, memberSession(testRoomID))
	if gone.Code != http.StatusOK {
		t.Fatalf("the rubbed-out hex answers a player with %d", gone.Code)
	}
	if strings.Contains(gone.Body.String(), "Ruined tower") {
		t.Errorf("the rubbed-out note is still on the page:\n%s", gone.Body.String())
	}
}
