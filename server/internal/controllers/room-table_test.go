package controllers

import (
	"context"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"tabletopper/internal/hub"
	"tabletopper/internal/room"
	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

// THE GM'S TABLE CONTROLS. What these check is the seam: that the handlers
// establish the right actor and hand every rule to internal/room, that the
// fragments are the GM's alone, and that the one thing the core cannot do --
// turn an asset id into a pyramid -- happens and happens correctly.

var (
	testMapID    = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVTA")
	testMapGen   = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVTB")
	testOtherMap = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVTC")
)

// pyramidAnswer is a GetMapPyramid row: a map that has finished tiling, or one
// that has not when gen is the zero ULID.
func pyramidAnswer(owner ulid.ULID, gen ulid.ULID, w, h, tile, maxZoom int) roomAnswer {
	var genValue driver.Value
	if gen != (ulid.ULID{}) {
		genValue = gen.Bytes()
	}

	return roomAnswer{
		columns: []string{"owner_id", "width", "height", "tile_size", "max_zoom", "tile_gen"},
		values:  []driver.Value{owner.Bytes(), int64(w), int64(h), int64(tile), int64(maxZoom), genValue},
	}
}

// readyMapAnswer is a ListReadyMaps row.
func readyMapAnswer(id ulid.ULID, name string, w, h int) roomAnswer {
	return roomAnswer{
		columns: []string{"id", "name", "width", "height"},
		values:  []driver.Value{id.Bytes(), name, int64(w), int64(h)},
	}
}

func tableRoomAnswer() roomAnswer {
	return getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false)
}

// tableApp is liveRoomApp with a hub that can read rows, which is what
// resolveMap needs: internal/room has no database and the hub is the half of
// TableSetLayerMap that does.
func tableApp(t *testing.T, db *roomDB) *App {
	t.Helper()

	app := newRoomApp(db)
	app.Hub = hub.New(app.Queries, hub.Options{Store: emptyRoomStore{}})

	return app
}

func tableRequest(t *testing.T, handler http.HandlerFunc, method, path string, values map[string]string, form url.Values, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()

	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}

	r := httptest.NewRequest(method, path, body)
	if form != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for key, value := range values {
		r.SetPathValue(key, value)
	}
	r = r.WithContext(session.NewContext(r.Context(), sess))

	rec := httptest.NewRecorder()
	handler(rec, r)

	return rec
}

// firstLayer is the id of the layer NewState builds every room with, which is
// what these tests aim their commands at.
func firstLayer(t *testing.T, app *App) ulid.ULID {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	view, ok := app.Hub.Table(ctx, testRoomID)
	if !ok || len(view.Table.Layers) == 0 {
		t.Fatal("the room has no layers")
	}

	return view.Table.Layers[0].ID
}

// EVERY ONE OF THESE IS THE GM'S, AND A PLAYER GETS THE SAME 404 A STRANGER
// DOES. They are the controls that decide what the table is; a player who could
// fetch one would be reading the room's configuration, and there is no reading
// of that which is not a leak.
func TestTheTableFragmentsAreTheGMsAlone(t *testing.T) {
	for name, handler := range map[string]func(*App) http.HandlerFunc{
		"layers": func(a *App) http.HandlerFunc { return a.RoomLayersFragment },
		"grid":   func(a *App) http.HandlerFunc { return a.RoomGridFragment },
		"maps":   func(a *App) http.HandlerFunc { return a.RoomMapsFragment },
	} {
		t.Run(name, func(t *testing.T) {
			app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})

			rec := tableRequest(t, handler(app), http.MethodGet,
				"/fragment/room/"+name+"?room="+testRoomID.String()+"&layer="+testRoomID.String(),
				nil, nil, memberSession(testRoomID))

			if rec.Code != http.StatusNotFound {
				t.Errorf("a player got %d from the %s fragment, want 404", rec.Code, name)
			}
		})
	}
}

// THE HANDLER AUTHORISES NOTHING AND THAT IS THE POINT. A player posting to a
// layer route is a member of the room, so a 404 would be a lie; the command's
// own Authorize refuses them, and rejectCommand turns that into the alert modal
// with the protocol's own sentence in it.
func TestAPlayerIsRefusedByTheCommandAndNotByTheHandler(t *testing.T) {
	app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})
	layer := firstLayer(t, app)

	rec := tableRequest(t, app.ActivateLayer, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/layers/"+layer.String()+"/activate",
		map[string]string{"id": testRoomID.String(), "layer": layer.String()},
		nil, memberSession(testRoomID))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}

	// The refusal arrives as an alert, not as a swap, and it says what was
	// refused rather than that something went wrong.
	trigger := rec.Header().Get("HX-Trigger")
	if !strings.Contains(trigger, "alert") {
		t.Errorf("no alert on the refusal: %q", trigger)
	}
	if !strings.Contains(trigger, "change the active layer") {
		t.Errorf("the alert does not say what was refused: %q", trigger)
	}
}

// THE PYRAMID IS THE HUB'S HALF OF THE COMMAND. internal/room cannot read a
// row, so the geometry a renderer fetches tiles with is filled in on the way
// past -- and this is the whole of that path, from a form field to a layer with
// a map on it.
func TestSettingAMapCopiesItsPyramidOntoTheLayer(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		tableRoomAnswer(),
		pyramidAnswer(testOwnerID, testMapGen, 12000, 9000, 512, 5),
	}}
	app := tableApp(t, db)
	layer := firstLayer(t, app)

	rec := tableRequest(t, app.SetLayerMap, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/layers/"+layer.String()+"/map",
		map[string]string{"id": testRoomID.String(), "layer": layer.String()},
		url.Values{"asset": {testMapID.String()}}, session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}

	// THE PICKER IS THE ONE MODAL IN THE TABLE FAMILY, and choosing is what
	// ends it. Without this the cards would keep sitting over the table the GM
	// picked the map to look at, which reads as the click having missed.
	if trigger := rec.Header().Get("HX-Trigger"); !strings.Contains(trigger, "modal:close") {
		t.Errorf("the picker stayed open: %q", trigger)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	view, ok := app.Hub.Table(ctx, testRoomID)
	if !ok {
		t.Fatal("the table is gone")
	}

	got := view.Table.Layers[0].Map
	if got == nil {
		t.Fatal("the layer has no map")
	}

	want := room.MapRef{AssetID: testMapID, Gen: testMapGen, Width: 12000, Height: 9000, TileSize: 512, MaxZoom: 5}
	if *got != want {
		t.Errorf("the layer's map is %+v, want %+v", *got, want)
	}
}

// THE TILE ROUTE IS DELIBERATELY UNSCOPED so everybody AT the table can fetch a
// map, which is exactly why CHOOSING one has to be scoped. Without this a GM
// could put any map in the database on their table by guessing an id.
func TestAGMCannotPutSomebodyElsesMapOnTheirTable(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		tableRoomAnswer(),
		pyramidAnswer(testMemberID, testMapGen, 12000, 9000, 512, 5),
	}}
	app := tableApp(t, db)
	layer := firstLayer(t, app)

	rec := tableRequest(t, app.SetLayerMap, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/layers/"+layer.String()+"/map",
		map[string]string{"id": testRoomID.String(), "layer": layer.String()},
		url.Values{"asset": {testOtherMap.String()}}, session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}

	// It reads as gone rather than as forbidden, which is the honest answer:
	// the GM's library does not contain it, and saying "that is somebody
	// else's" would confirm the id names something.
	if trigger := rec.Header().Get("HX-Trigger"); !strings.Contains(trigger, "no longer in your library") {
		t.Errorf("the refusal reads %q", trigger)
	}
}

// A map that is queued, running or failed has no pyramid in the bucket. Setting
// it would give every browser at the table a URL that 404s at every zoom, so it
// is refused with something a GM can act on.
func TestAMapThatHasNotFinishedTilingIsRefused(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		tableRoomAnswer(),
		pyramidAnswer(testOwnerID, ulid.ULID{}, 12000, 9000, 512, 5),
	}}
	app := tableApp(t, db)
	layer := firstLayer(t, app)

	rec := tableRequest(t, app.SetLayerMap, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/layers/"+layer.String()+"/map",
		map[string]string{"id": testRoomID.String(), "layer": layer.String()},
		url.Values{"asset": {testMapID.String()}}, session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body: %s", rec.Code, rec.Body.String())
	}
	if trigger := rec.Header().Get("HX-Trigger"); !strings.Contains(trigger, "not finished tiling") {
		t.Errorf("the refusal reads %q", trigger)
	}
}

// The picker asks for the owner's maps that have a generation, and both halves
// of that matter: the wrong owner is a leak and the wrong filter is a table
// full of broken tiles.
func TestThePickerAsksOnlyForTheOwnersReadyMaps(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		tableRoomAnswer(),
		readyMapAnswer(testMapID, "Death House", 4000, 3000),
	}}
	app := tableApp(t, db)
	layer := firstLayer(t, app)

	rec := tableRequest(t, app.RoomMapsFragment, http.MethodGet,
		"/fragment/room/maps?room="+testRoomID.String()+"&layer="+layer.String(),
		nil, nil, session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var listing recordedCall
	for _, call := range db.calls {
		if strings.Contains(call.query, "tile_gen IS NOT NULL") {
			listing = call
		}
	}
	if listing.query == "" {
		t.Fatalf("nothing filtered on tile_gen; statements: %v", db.queries())
	}
	if !strings.Contains(listing.query, "owner_id = ?") {
		t.Errorf("the listing is not scoped to an owner: %q", listing.query)
	}
	if got, ok := boundRoomID(listing.args[0]); !ok || got != testOwnerID {
		t.Errorf("the listing asked for %s's maps, want %s", got, testOwnerID)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Death House") || !strings.Contains(body, `name="asset" value="`+testMapID.String()+`"`) {
		t.Errorf("the card does not post the map:\n%s", body)
	}
	// The card posts to the LAYER's map resource, because what is being
	// changed is the layer and not the picker.
	if !strings.Contains(body, `hx-post="/rooms/`+testRoomID.String()+`/layers/`+layer.String()+`/map"`) {
		t.Errorf("the card posts somewhere else:\n%s", body)
	}
}

// A form this server did not write is a 404 and not a message. The only thing
// that produces one is somebody posting by hand, and there is nothing to tell
// them that is not a hint.
func TestAFormWithNoAssetInItIsRefusedWithoutAskingTheRoom(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)
	layer := firstLayer(t, app)

	before := len(db.calls)

	rec := tableRequest(t, app.SetLayerMap, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/layers/"+layer.String()+"/map",
		map[string]string{"id": testRoomID.String(), "layer": layer.String()},
		url.Values{"asset": {"not-a-ulid"}}, session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if got := len(db.calls) - before; got != 1 {
		t.Errorf("%d statements past the room lookup, want 0: %v", got-1, db.queries())
	}
}

// THE GRID'S REFUSAL GOES ABOVE THE FIELD AND NOT INTO THE ALERT MODAL, which
// is the one thing in this file that is rendered rather than triggered. "Cell
// size must be between 8 and 512 pixels" is about the 4 somebody typed.
func TestABadCellSizeComesBackIntoTheFormsErrorBlock(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)

	rec := tableRequest(t, app.SetRoomGrid, http.MethodPost, "/rooms/"+testRoomID.String()+"/grid",
		map[string]string{"id": testRoomID.String()},
		url.Values{
			"cellSize": {"4"}, "offsetX": {"0"}, "offsetY": {"0"},
			"color": {"#000000FF"}, "snap": {"cells"}, "feetPerCell": {"5"},
			"diagonals": {"equal"}, "monsterHp": {"band"},
		}, session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Cell size must be between") {
		t.Errorf("the message is not the core's:\n%s", body)
	}
	if rec.Header().Get("HX-Trigger") != "" {
		t.Errorf("a form error also opened the alert modal: %q", rec.Header().Get("HX-Trigger"))
	}
}

// A save that works answers an EMPTY error block rather than 204, which is the
// deliberate exception in this file: the block stays on screen until something
// replaces it, so a good save has to clear what the last one left there.
func TestASavedGridClearsTheMessageTheLastAttemptLeft(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)

	rec := tableRequest(t, app.SetRoomGrid, http.MethodPost, "/rooms/"+testRoomID.String()+"/grid",
		map[string]string{"id": testRoomID.String()},
		url.Values{
			"showGrid": {"on"}, "cellSize": {"70"}, "offsetX": {"12"}, "offsetY": {"-4"},
			"color": {"#3355ffcc"}, "snap": {"corners"}, "feetPerCell": {"10"},
			"diagonals": {"alternating"}, "monsterHp": {"hidden"}, "playersCanDraw": {"on"},
		}, session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Could not save") {
		t.Errorf("a good save still shows an error:\n%s", rec.Body.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	view, ok := app.Hub.Table(ctx, testRoomID)
	if !ok {
		t.Fatal("the table is gone")
	}

	want := room.Grid{
		Visible: true, CellSize: 70, OffsetX: 12, OffsetY: -4,
		Color: "#3355FFCC", Snap: room.SnapCorners, FeetPerCell: 10,
		Diagonals: room.DiagonalsAlternating,
	}
	if view.Table.Grid != want {
		t.Errorf("the grid is %+v, want %+v", view.Table.Grid, want)
	}

	// The options ride in the same form and are a second command, so a form
	// that saved the grid and dropped them would look like it worked.
	if view.Table.MonsterHP != room.HPHidden || !view.Table.PlayersCanDraw {
		t.Errorf("the options are %q / %v", view.Table.MonsterHP, view.Table.PlayersCanDraw)
	}
}

// AN UNCHECKED BOX SENDS NOTHING AT ALL. That is how HTML forms work, and it is
// why both toggles are read by presence -- a reader that looked for "false"
// would leave a grid switched on for ever.
func TestAnUncheckedToggleReadsAsFalse(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/rooms/x/grid", strings.NewReader(url.Values{
		"cellSize": {"64"}, "offsetX": {"0"}, "offsetY": {"0"}, "color": {"#000000FF"},
		"snap": {"cells"}, "feetPerCell": {"5"}, "diagonals": {"equal"}, "monsterHp": {"band"},
	}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	grid, options, problems := gridForm(r)

	if len(problems) != 0 {
		t.Fatalf("a complete form was refused: %v", problems)
	}
	if grid.Visible {
		t.Error("an absent grid toggle read as on")
	}
	if options.PlayersCanDraw {
		t.Error("an absent drawing toggle read as on")
	}
}

// The hash is optional in the field and required in the state, so it is put
// back rather than made the reader's problem -- and the case is normalised so
// two GMs typing the same colour store the same string.
func TestAColourWithoutAHashIsStillAColour(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/rooms/x/grid", strings.NewReader(url.Values{
		"cellSize": {"64"}, "offsetX": {"0"}, "offsetY": {"0"}, "color": {"  3355ffcc  "},
		"snap": {"cells"}, "feetPerCell": {"5"}, "diagonals": {"equal"}, "monsterHp": {"band"},
	}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	grid, _, problems := gridForm(r)

	if len(problems) != 0 {
		t.Fatalf("refused: %v", problems)
	}
	if grid.Color != "#3355FFCC" {
		t.Errorf("colour = %q, want #3355FFCC", grid.Color)
	}
}

// A field that is not a number has no value to send, so the core would be asked
// about a zero the reader never typed. Shape is this function's question;
// whether the number is legal is the core's.
func TestAFieldThatIsNotANumberIsCaughtBeforeTheCoreSeesIt(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/rooms/x/grid", strings.NewReader(url.Values{
		"cellSize": {"sixty four"}, "offsetX": {"0"}, "offsetY": {"0"}, "color": {"#000000FF"},
		"snap": {"cells"}, "feetPerCell": {"5"}, "diagonals": {"equal"}, "monsterHp": {"band"},
	}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	_, _, problems := gridForm(r)

	if len(problems) != 1 || !strings.Contains(problems[0], "Cell size") {
		t.Errorf("problems = %v, want one about the cell size", problems)
	}
}

// THE PAIR THAT MUST NOT CLOSE INTO A CIRCLE. The player's menu bar asks for
// the active layer's name on load and swaps the reply over itself; htmx fires
// a load trigger the moment it processes an element, including one it has just
// swapped in. So the page arms the fetch exactly once and the fragment must
// never arm it again -- the shipped bug was a browser that fetched this one
// string for the rest of the session, with the loading bar up throughout.
func TestTheLayerNameFragmentDoesNotAskForItselfAgain(t *testing.T) {
	page := getRoomPage(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}, memberSession(testRoomID))
	if !strings.Contains(page.Body.String(), `hx-trigger="load, room:tabletop from:window"`) {
		t.Fatal("the player's page never asks for the layer name")
	}

	app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})

	rec := tableRequest(t, app.RoomLayerFragment, http.MethodGet,
		"/fragment/room/layer?room="+testRoomID.String(), nil, nil, memberSession(testRoomID))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "load") {
		t.Errorf("the fragment arms its own load trigger again:\n%s", rec.Body.String())
	}
}
