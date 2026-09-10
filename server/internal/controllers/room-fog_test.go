package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"tabletopper/internal/room"
	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

// THE FOG MENU'S TWO ROUTES, AND WHAT THEY ARE ASSERTED THROUGH.
//
// Both answer 204 and say nothing, so what they did is read back off the hub --
// and the only thing the hub hands out about a floor is its two flags. That is
// enough to pin the sequence, because each route is set up here on a floor whose
// flags are the OPPOSITE of what the route should leave them: fill is tested on
// a floor a hide left clear, so a fill that skipped fog.setPrefill fails, and
// clear is tested on a floor a reveal left covered, so a clear that skipped
// fog.setEnabled fails.
//
// THAT fog.clear EMPTIES THE FLOOR IS NOT ASSERTED HERE. The shapes do not reach
// hub.TableView -- nothing on any screen needs a count of them -- and adding one
// so that a test could read it would be production weight carried for a test.
// The command's own behaviour is pinned in internal/room; what is pinned here is
// the seam.

// fogApp is tableApp with a room already running, and the floor to aim at.
func fogApp(t *testing.T) (*App, ulid.ULID) {
	t.Helper()

	app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})

	return app, firstLayer(t, app)
}

// wake puts one shape on a floor, which turns its fog on and sets the prefill
// from the shape's own mode. See room.FogAdd.
func wake(t *testing.T, app *App, layer ulid.ULID, mode room.FogMode) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := app.Hub.Dispatch(ctx, testRoomID, room.Actor{ID: testOwnerID, Role: room.RoleGM}, &room.FogAdd{
		Layer:  layer,
		Kind:   room.ShapeRect,
		Mode:   mode,
		Points: []int{0, 0, 64, 64},
	})
	if err != nil {
		t.Fatalf("could not put a shape on the floor: %v", err)
	}
}

// fogFlags reads one floor's two flags back off the hub.
func fogFlags(t *testing.T, app *App, id ulid.ULID) (enabled bool, prefill bool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	view, ok := app.Hub.Table(ctx, testRoomID)
	if !ok {
		t.Fatal("the room is not running")
	}

	for _, l := range view.Table.Layers {
		if l.ID.Compare(id) == 0 {
			return l.FogEnabled, l.FogPrefill
		}
	}

	t.Fatalf("there is no layer %s", id)

	return false, false
}

// fogRequest posts to one of the two fog routes with the layer where the room
// bundle puts it: in the form, not in the path. See the header of room-fog.go.
func fogRequest(t *testing.T, handler http.HandlerFunc, path string, layer string, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()

	return tableRequest(t, handler, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/fog/"+path,
		map[string]string{"id": testRoomID.String()},
		url.Values{"layer": {layer}}, sess)
}

// Fill on a floor a hide left CLEAR: covering it takes both flags, so a fill
// that only turned the fog on would leave the map showing.
func TestFillFogCoversTheFloorItIsAimedAt(t *testing.T) {
	app, layer := fogApp(t)
	wake(t, app, layer, room.FogHide)

	if _, prefill := fogFlags(t, app, layer); prefill {
		t.Fatal("the floor was already covered; the test proves nothing")
	}

	rec := fogRequest(t, app.FillLayerFog, "fill", layer.String(), gmSession())
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}

	enabled, prefill := fogFlags(t, app, layer)
	if !enabled || !prefill {
		t.Errorf("the floor is not covered: enabled=%v prefill=%v", enabled, prefill)
	}
}

// Clear on a floor a reveal left COVERED, so a clear that only emptied the
// shapes would leave the players staring at a solid block.
func TestClearFogUncoversTheFloorItIsAimedAt(t *testing.T) {
	app, layer := fogApp(t)
	wake(t, app, layer, room.FogReveal)

	if enabled, _ := fogFlags(t, app, layer); !enabled {
		t.Fatal("the floor's fog was already off; the test proves nothing")
	}

	rec := fogRequest(t, app.ClearLayerFog, "clear", layer.String(), gmSession())
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}

	if enabled, _ := fogFlags(t, app, layer); enabled {
		t.Error("the floor still has its fog on")
	}
}

// THE ROUTE ACTS ON THE FLOOR IT IS GIVEN AND NOT ON THE ACTIVE ONE, which is
// the whole reason the layer travels in the form. A GM covering the first floor
// while the party is in the cellar is what the menu is for.
func TestAFogRouteActsOnTheFloorInTheFormAndNotTheActiveOne(t *testing.T) {
	app, active := fogApp(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	gm := room.Actor{ID: testOwnerID, Role: room.RoleGM}
	if err := app.Hub.Dispatch(ctx, testRoomID, gm, &room.TableAddLayer{Name: "Cellar"}); err != nil {
		t.Fatalf("could not add a floor: %v", err)
	}

	view, ok := app.Hub.Table(ctx, testRoomID)
	if !ok || len(view.Table.Layers) != 2 {
		t.Fatal("the second floor is not there")
	}
	other := view.Table.Layers[1].ID

	rec := fogRequest(t, app.FillLayerFog, "fill", other.String(), gmSession())
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}

	if enabled, _ := fogFlags(t, app, other); !enabled {
		t.Error("the floor named in the form was not covered")
	}
	if enabled, _ := fogFlags(t, app, active); enabled {
		t.Error("the ACTIVE floor was covered instead")
	}
}

// A PLAYER IS REFUSED BY THE COMMAND AND NOT BY THE HANDLER, which is the rule
// every other layer route follows: a player posting here is a member of the
// room, so a 404 would be a lie, and the alert modal carries the protocol's own
// sentence.
func TestTheFogRoutesAreTheGMsAlone(t *testing.T) {
	for name, pick := range map[string]func(*App) http.HandlerFunc{
		"fill":  func(a *App) http.HandlerFunc { return a.FillLayerFog },
		"clear": func(a *App) http.HandlerFunc { return a.ClearLayerFog },
	} {
		t.Run(name, func(t *testing.T) {
			app, layer := fogApp(t)

			rec := fogRequest(t, pick(app), name, layer.String(), memberSession(testRoomID))
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
			}
			if trigger := rec.Header().Get("HX-Trigger"); !strings.Contains(trigger, "alert") {
				t.Errorf("no alert on the refusal: %q", trigger)
			}
		})
	}
}

// NO LAYER AT ALL IS THE ACTIVE FLOOR, which is what makes the two items work in
// a browser where the room bundle never ran. The item posts hx-vals of "{}"
// until the bundle fills it in, and a menu item that silently did nothing for
// the first second of a page load would be reported as broken.
func TestAFogRouteWithNoLayerFallsBackToTheActiveFloor(t *testing.T) {
	app, active := fogApp(t)

	rec := fogRequest(t, app.FillLayerFog, "fill", "", gmSession())
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}

	if enabled, prefill := fogFlags(t, app, active); !enabled || !prefill {
		t.Errorf("the active floor is not covered: enabled=%v prefill=%v", enabled, prefill)
	}
}

// A LAYER THAT IS THERE AND IS NOT AN ID IS A 404 WITH NOTHING IN IT. The value
// is written by the room bundle out of the store, so the only thing that
// produces one of these is somebody posting by hand -- and there is nothing to
// tell them that is not a hint.
func TestAFogRouteWithAMangledLayerIsANotFound(t *testing.T) {
	app, _ := fogApp(t)

	rec := fogRequest(t, app.FillLayerFog, "fill", "not-a-ulid", gmSession())
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("the 404 has a body: %s", rec.Body.String())
	}
}
