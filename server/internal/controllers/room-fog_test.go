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


















func fogApp(t *testing.T) (*App, ulid.ULID) {
	t.Helper()

	app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})

	return app, firstLayer(t, app)
}



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



func fogRequest(t *testing.T, handler http.HandlerFunc, path string, layer string, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()

	return tableRequest(t, handler, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/fog/"+path,
		map[string]string{"id": testRoomID.String()},
		url.Values{"layer": {layer}}, sess)
}



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
