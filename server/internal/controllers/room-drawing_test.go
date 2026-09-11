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
func drawingApp(t *testing.T) (*App, ulid.ULID) {
	t.Helper()
	app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})
	return app, firstLayer(t, app)
}
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
