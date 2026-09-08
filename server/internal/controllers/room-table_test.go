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
	"tabletopper/internal/tiling"
	"tabletopper/templ/pages"

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

// readyMapAnswer is a ListReadyMaps row, which is what the layer manager reads
// to put a name under each layer.
func readyMapAnswer(id ulid.ULID, name string, w, h int) roomAnswer {
	return roomAnswer{
		columns: []string{"id", "name", "width", "height"},
		values:  []driver.Value{id.Bytes(), name, int64(w), int64(h)},
	}
}

// pickerMapColumns is what ListPickerMaps and SearchPickerMaps both select.
var pickerMapColumns = []string{"id", "name", "file_name", "width", "height", "tile_gen", "tile_state", "tile_attempts"}

// pickerMapAnswer is a ListPickerMaps row: a map that has been tiled, so it has
// a generation, dimensions and no job outstanding.
func pickerMapAnswer(id ulid.ULID, name string, w, h int) roomAnswer {
	return roomAnswer{
		columns: pickerMapColumns,
		values:  []driver.Value{id.Bytes(), name, name + ".png", int64(w), int64(h), testMapGen.Bytes(), nil, int64(0)},
	}
}

// buildingMapAnswer is the same row for a map that was uploaded a moment ago:
// no generation, no size yet, and a job queued. It is the state the picker had
// to learn to show, because uploading is now something done from inside it.
func buildingMapAnswer(id ulid.ULID, name string) roomAnswer {
	return roomAnswer{
		columns: pickerMapColumns,
		values:  []driver.Value{id.Bytes(), name, name, nil, nil, nil, "pending", int64(0)},
	}
}

// failedMapAnswer is a map whose tiling failed, attempts goes in as the number
// of tries it has already had. Below tiling.MaxAttempts the worker is coming
// back for it; at the cap it is not, and the card has to say which.
func failedMapAnswer(id ulid.ULID, name string, attempts int) roomAnswer {
	return roomAnswer{
		columns: pickerMapColumns,
		values:  []driver.Value{id.Bytes(), name, name, nil, nil, nil, "failed", int64(attempts)},
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

// pickerRequest is the dialog or its grid, asked for as the room's GM.
func pickerRequest(t *testing.T, app *App, handler http.HandlerFunc, path string, query string) *httptest.ResponseRecorder {
	t.Helper()

	layer := firstLayer(t, app)
	url := path + "?room=" + testRoomID.String() + "&layer=" + layer.String() + query

	return tableRequest(t, handler, http.MethodGet, url, nil, nil, session.UserSession{UserID: testOwnerID})
}

// listing is the statement the picker read its cards from, which is the one
// ordered by whether a map has been tiled. The room lookup runs first and this
// picks the listing out from behind it.
func listing(t *testing.T, db *roomDB) recordedCall {
	t.Helper()

	for _, call := range db.calls {
		if strings.Contains(call.query, "ORDER BY tile_gen IS NOT NULL") {
			return call
		}
	}
	t.Fatalf("the picker read no map listing; statements: %v", db.queries())

	return recordedCall{}
}

// THE PICKER ASKS FOR THE OWNER'S WHOLE MAP LIBRARY, and both halves of that
// matter. The wrong owner is a leak. The wrong filter is the reason this changed:
// it used to ask only for maps with a generation, which was right when the only
// way to get a map was the asset manager and wrong the moment uploading moved in
// here -- a GM who presses Upload has to be able to watch the thing they uploaded
// build, and a listing that hid it until it was finished would show nothing at
// all for the minute that takes.
func TestThePickerAsksForTheOwnersWholeMapLibrary(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		tableRoomAnswer(),
		pickerMapAnswer(testMapID, "Death House", 4000, 3000),
	}}
	app := tableApp(t, db)

	rec := pickerRequest(t, app, app.RoomMapsFragment, "/fragment/room/maps", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	read := listing(t, db)
	if strings.Contains(read.query, "tile_gen IS NOT NULL\n") || strings.Contains(read.query, "AND tile_gen IS NOT NULL") {
		t.Errorf("the picker still filters out maps that have not been tiled: %q", read.query)
	}
	if !strings.Contains(read.query, "owner_id = ?") {
		t.Errorf("the listing is not scoped to an owner: %q", read.query)
	}
	if got, ok := boundRoomID(read.args[0]); !ok || got != testOwnerID {
		t.Errorf("the listing asked for %s's maps, want %s", got, testOwnerID)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Death House") || !strings.Contains(body, `name="asset" value="`+testMapID.String()+`"`) {
		t.Errorf("the card does not post the map:\n%s", body)
	}
	// The card posts to the LAYER's map resource, because what is being
	// changed is the layer and not the picker.
	if !strings.Contains(body, `hx-post="/rooms/`+testRoomID.String()+`/layers/`+firstLayer(t, app).String()+`/map"`) {
		t.Errorf("the card posts somewhere else:\n%s", body)
	}
}

// A SEARCH MATCHES THE FILE THE MAP CAME FROM AS WELL AS ITS NAME, which is the
// one place in the app that does. The manager's search deliberately does not --
// see SearchMaps in assets.sql -- and the picker is the case that reason does not
// cover: it is reached mid-session with a map in mind, and a map renamed a month
// ago is as often remembered by its export as by its name.
func TestThePickerSearchesBothNames(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		tableRoomAnswer(),
		pickerMapAnswer(testMapID, "Death House", 4000, 3000),
	}}
	app := tableApp(t, db)

	rec := pickerRequest(t, app, app.RoomMapListFragment, "/fragment/room/map-list", "&q=death")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	read := listing(t, db)
	if !strings.Contains(read.query, "name LIKE") || !strings.Contains(read.query, "file_name LIKE") {
		t.Errorf("the search does not look at both names: %q", read.query)
	}
	// The term is a pattern and not a word: the caller escapes it, so a
	// wildcard somebody typed is a character and not the whole library.
	if got := read.args[1]; got != "%death%" {
		t.Errorf("the bound term is %v, want %%death%%", got)
	}
}

// A TERM LONGER THAN A NAME CAN BE IS A 404 WITH AN EMPTY BODY, and the box
// carries a maxlength so it cannot come from the dialog. There is nobody on the
// other end to tell, which is why this is not an alert.
func TestAnOverlongSearchIsRefusedWithoutReadingTheLibrary(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)

	rec := pickerRequest(t, app, app.RoomMapListFragment, "/fragment/room/map-list",
		"&q="+strings.Repeat("a", pages.AssetNameLimit+1))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("the refusal has a body: %s", rec.Body.String())
	}
	for _, call := range db.calls {
		if strings.Contains(call.query, "ORDER BY tile_gen IS NOT NULL") {
			t.Errorf("the library was read anyway: %v", db.queries())
		}
	}
}

// A MAP THAT IS STILL BUILDING IS ON THE SHELF AND IS NOT A BUTTON, and the grid
// polls while it is there. The card says what it is doing, the grid asks again in
// two seconds, and both of those stop by themselves: the poll lives on the grid's
// own markup, so the swap that comes back without a building map comes back
// without an hx-trigger.
func TestAMapThatIsStillBuildingIsShownButCannotBeChosen(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		tableRoomAnswer(),
		buildingMapAnswer(testMapID, "sunless-citadel.png"),
	}}
	app := tableApp(t, db)

	rec := pickerRequest(t, app, app.RoomMapListFragment, "/fragment/room/map-list", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "sunless-citadel.png") {
		t.Errorf("the map is not on the shelf:\n%s", body)
	}
	if strings.Contains(body, `name="asset"`) {
		t.Errorf("a map with no tiles is offered as a choice:\n%s", body)
	}
	if !strings.Contains(body, `hx-trigger="every 2s"`) {
		t.Errorf("the card does not ask again while its tiles are building:\n%s", body)
	}
	// The card polls its own fragment, so a map finishing cannot move the
	// cards around it or throw away somebody's scroll position.
	if !strings.Contains(body, "/fragment/room/map-card?room="+testRoomID.String()) {
		t.Errorf("the card does not name the fragment it refetches:\n%s", body)
	}
}

// AND A CARD WITH NOTHING LEFT TO WAIT FOR DOES NOT POLL. This is the half that
// makes the poll end rather than run for as long as the dialog is open.
func TestACardWithNothingBuildingDoesNotPoll(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		tableRoomAnswer(),
		pickerMapAnswer(testMapID, "Death House", 4000, 3000),
	}}
	app := tableApp(t, db)

	rec := pickerRequest(t, app, app.RoomMapListFragment, "/fragment/room/map-list", "")

	if body := rec.Body.String(); strings.Contains(body, "every 2s") {
		t.Errorf("a finished card polls with nothing to wait for:\n%s", body)
	}
}

// A MAP WHOSE TILING FAILED CAN BE TRIED AGAIN FROM IN HERE. There is no link
// out of this dialog to the asset manager any more, so a failure with no control
// beside it would be a dead end in the middle of a session.
func TestAFailedMapOffersItsRetryInThePicker(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		tableRoomAnswer(),
		failedMapAnswer(testMapID, "castle.png", tiling.MaxAttempts),
	}}
	app := tableApp(t, db)

	rec := pickerRequest(t, app, app.RoomMapListFragment, "/fragment/room/map-list", "")

	body := rec.Body.String()
	if !strings.Contains(body, `/maps/`+testMapID.String()+`"`) {
		t.Errorf("a failed map cannot be tried again:\n%s", body)
	}
	// It answers with this card and nothing else, so the retry replaces the
	// card that was pressed rather than the grid around it.
	if !strings.Contains(body, `hx-target="closest room-map-card"`) {
		t.Errorf("the retry does not replace its own card:\n%s", body)
	}
}

// A FAILURE THE WORKER IS COMING BACK FOR DOES NOT SAY IT GAVE UP, which it did
// for as long as the card read the state and not the attempt count. The sweep
// requeues a failed map with tries left a lease window later, so the first
// failure of three is a wait and not an ending -- and the button offers to skip
// the wait rather than to start something that was not going to happen.
func TestAFailureWithTriesLeftSaysSoAndTheLastOneDoesNot(t *testing.T) {
	for name, tc := range map[string]struct {
		attempts int
		want     string
		notWant  string
	}{
		"the first of three": {attempts: 1, want: "Trying again", notWant: "gave up"},
		"the last of three":  {attempts: tiling.MaxAttempts, want: "gave up", notWant: "Trying again"},
	} {
		t.Run(name, func(t *testing.T) {
			db := &roomDB{rows: 1, answers: []roomAnswer{
				tableRoomAnswer(),
				failedMapAnswer(testMapID, "castle.png", tc.attempts),
			}}
			app := tableApp(t, db)

			rec := pickerRequest(t, app, app.RoomMapListFragment, "/fragment/room/map-list", "")

			body := rec.Body.String()
			if !strings.Contains(body, tc.want) {
				t.Errorf("after %d attempts the card does not say %q:\n%s", tc.attempts, tc.want, body)
			}
			if strings.Contains(body, tc.notWant) {
				t.Errorf("after %d attempts the card still says %q:\n%s", tc.attempts, tc.notWant, body)
			}
		})
	}
}

// THE UPLOAD'S HAPPY PATH IS NOT HERE, because storeMap puts the original in R2
// and App.Storage is a concrete *storage.Client with nothing to stand in for it.
// What the picker adds to that path is the card on the end of it, and that is
// pinned in templ/pages: TestAPickerCardPollsUntilItsTilesAreReady renders a
// pending card and a finished one and checks that the first fetches its own
// replacement and the second does not.

// A PLAYER CANNOT UPLOAD INTO A ROOM THEY ARE ONLY SITTING AT, and the refusal
// comes before the file is read: the route is the GM's picker, so somebody who
// is not the GM has no picker to be uploading from.
func TestAPlayerCannotUploadThroughThePicker(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)
	layer := firstLayer(t, app)

	before := len(db.calls)
	rec := tableRequest(t, app.UploadRoomMap, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/layers/"+layer.String()+"/maps",
		map[string]string{"id": testRoomID.String(), "layer": layer.String()},
		nil, memberSession(testRoomID))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	for _, call := range db.calls[before:] {
		if strings.Contains(call.query, "INSERT INTO assets") {
			t.Errorf("a player's upload wrote a row: %v", db.queries())
		}
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
			"gridLines": {"solid"}, "cellSize": {"4"}, "offsetX": {"0"}, "offsetY": {"0"},
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
			"gridLines": {"dashed"}, "cellSize": {"70"}, "offsetX": {"12"}, "offsetY": {"-4"},
			"color": {"#3355ffcc"}, "snap": {"halfCells"}, "feetPerCell": {"10"},
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
		Lines: room.GridLinesDashed, CellSize: 70, OffsetX: 12, OffsetY: -4,
		Color: "#3355FFCC", Snap: room.SnapHalfCells, FeetPerCell: 10,
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
// why the toggle is read by presence -- a reader that looked for "false" would
// leave drawing switched on for ever.
func TestAnUncheckedToggleReadsAsFalse(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/rooms/x/grid", strings.NewReader(url.Values{
		"gridLines": {"solid"}, "cellSize": {"64"}, "offsetX": {"0"}, "offsetY": {"0"},
		"color": {"#000000FF"}, "snap": {"cells"}, "feetPerCell": {"5"},
		"diagonals": {"equal"}, "monsterHp": {"band"},
	}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	grid, options, problems := gridForm(r)

	if len(problems) != 0 {
		t.Fatalf("a complete form was refused: %v", problems)
	}
	if grid.Lines != room.GridLinesSolid {
		t.Errorf("the line style read as %q", grid.Lines)
	}
	if options.PlayersCanDraw {
		t.Error("an absent drawing toggle read as on")
	}
}

// THE LINE STYLE IS A RADIO GROUP AND NOT A CHECKBOX, so "absent" is not a
// state it has: a browser always sends the one that is checked. What arrives
// without it is a request nobody's form made, and the core refuses it by name
// rather than this handler quietly choosing a style for it.
func TestAGridWithNoLineStyleIsRefused(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)

	rec := tableRequest(t, app.SetRoomGrid, http.MethodPost, "/rooms/"+testRoomID.String()+"/grid",
		map[string]string{"id": testRoomID.String()},
		url.Values{
			"cellSize": {"64"}, "offsetX": {"0"}, "offsetY": {"0"},
			"color": {"#000000FF"}, "snap": {"cells"}, "feetPerCell": {"5"},
			"diagonals": {"equal"}, "monsterHp": {"band"},
		}, session.UserSession{UserID: testOwnerID})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "grid line style") {
		t.Errorf("the message is not the core's:\n%s", body)
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
