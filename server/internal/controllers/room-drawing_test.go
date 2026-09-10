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

// CLEAR DRAWING, AND WHAT IT IS ASSERTED THROUGH.
//
// The route answers 204 and says nothing, and the strokes do not reach
// hub.TableView -- nothing on any screen needs a count of them, and adding one
// so that a test could read it would be production weight carried for a test.
// So what the floor holds is read back through the protocol instead: a stroke id
// may be used once, so re-beginning a line the clear should have taken away
// SUCCEEDS if it went and is refused as "already on the table" if it did not.
// That is a positive assertion in both directions, which a "stroke gone"
// refusal would not have been.

// drawingApp is tableApp with a room already running, and the floor to aim at.
func drawingApp(t *testing.T) (*App, ulid.ULID) {
	t.Helper()

	app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})

	return app, firstLayer(t, app)
}

// sketch puts one finished line on a floor.
func sketch(t *testing.T, app *App, layer ulid.ULID, id ulid.ULID) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	gm := room.Actor{ID: testOwnerID, Role: room.RoleGM}
	if err := app.Hub.Dispatch(ctx, testRoomID, gm, &room.StrokeBegin{
		ID: id, Layer: layer, Kind: room.StrokeFree,
		Color: "#FF0000", Width: 4, Points: []int{0, 0, 64, 64},
	}); err != nil {
		t.Fatalf("could not draw on the floor: %v", err)
	}
	if err := app.Hub.Dispatch(ctx, testRoomID, gm, &room.StrokeEnd{ID: id}); err != nil {
		t.Fatalf("could not finish the line: %v", err)
	}
}

// stillThere answers whether the room still holds a stroke with this id, by
// trying to use the id again. See the header.
func stillThere(t *testing.T, app *App, layer ulid.ULID, id ulid.ULID) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := app.Hub.Dispatch(ctx, testRoomID, room.Actor{ID: testOwnerID, Role: room.RoleGM}, &room.StrokeBegin{
		ID: id, Layer: layer, Kind: room.StrokeFree,
		Color: "#00FF00", Width: 2, Points: []int{0, 0},
	})

	return err != nil
}

// drawingRequest posts to the route with the layer where the room bundle puts
// it: in the form, not in the path.
func drawingRequest(t *testing.T, app *App, layer string, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()

	return tableRequest(t, app.ClearLayerDrawing, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/drawing/clear",
		map[string]string{"id": testRoomID.String()},
		url.Values{"layer": {layer}}, sess)
}

func TestClearDrawingEmptiesTheFloorItIsAimedAt(t *testing.T) {
	app, layer := drawingApp(t)
	id := ulid.Make()
	sketch(t, app, layer, id)

	if !stillThere(t, app, layer, id) {
		t.Fatal("the line was not on the floor to begin with; the test proves nothing")
	}

	rec := drawingRequest(t, app, layer.String(), gmSession())
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}

	if stillThere(t, app, layer, id) {
		t.Error("the line is still on the floor")
	}
}

// THE ROUTE ACTS ON THE FLOOR IT IS GIVEN AND NOT ON THE ACTIVE ONE, which is
// the whole reason the layer travels in the form. A GM tidying the first floor
// while the party is in the cellar is what the menu item is for.
func TestClearDrawingActsOnTheFloorInTheFormAndNotTheActiveOne(t *testing.T) {
	app, active := drawingApp(t)

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

	here, there := ulid.Make(), ulid.Make()
	sketch(t, app, active, here)
	sketch(t, app, other, there)

	rec := drawingRequest(t, app, other.String(), gmSession())
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}

	if stillThere(t, app, other, there) {
		t.Error("the floor named in the form was not emptied")
	}
	if !stillThere(t, app, active, here) {
		t.Error("the ACTIVE floor was emptied instead")
	}
}

// A PLAYER IS REFUSED BY THE COMMAND AND NOT BY THE HANDLER, which is the rule
// every other layer route follows: a player posting here is a member of the
// room, so a 404 would be a lie, and the alert modal carries the protocol's own
// sentence.
//
// DRAWING IS EVERYBODY'S AND THIS IS NOT, which is the distinction worth
// pinning: rubbing out one line is the author's own, and throwing away every
// line on the floor including four other people's is the room owner's.
func TestClearDrawingIsTheGMsAlone(t *testing.T) {
	app, layer := drawingApp(t)
	id := ulid.Make()
	sketch(t, app, layer, id)

	rec := drawingRequest(t, app, layer.String(), memberSession(testRoomID))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}
	if trigger := rec.Header().Get("HX-Trigger"); !strings.Contains(trigger, "alert") {
		t.Errorf("no alert on the refusal: %q", trigger)
	}
	if !stillThere(t, app, layer, id) {
		t.Error("a player's refused request emptied the floor anyway")
	}
}

// NO LAYER AT ALL IS THE ACTIVE FLOOR, which is what makes the item work in a
// browser where the room bundle never ran. The item posts hx-vals of "{}" until
// the bundle fills it in, and a menu item that silently did nothing for the
// first second of a page load would be reported as broken.
func TestClearDrawingWithNoLayerFallsBackToTheActiveFloor(t *testing.T) {
	app, active := drawingApp(t)
	id := ulid.Make()
	sketch(t, app, active, id)

	rec := drawingRequest(t, app, "", gmSession())
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	if stillThere(t, app, active, id) {
		t.Error("the active floor was not emptied")
	}
}

// A layer that is PRESENT and is not an id is a request this server did not
// write, which is a 404 with nothing in it rather than a message: the only thing
// that produces one is somebody posting by hand.
func TestClearDrawingWithALayerThatIsNotAnIDIs404(t *testing.T) {
	app, _ := drawingApp(t)

	rec := drawingRequest(t, app, "not-a-ulid", gmSession())
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("the 404 has a body: %s", rec.Body.String())
	}
}
