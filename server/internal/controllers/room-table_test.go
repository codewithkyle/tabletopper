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

var (
	testMapID    = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVTA")
	testMapGen   = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVTB")
	testOtherMap = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVTC")
)

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
func readyMapAnswer(id ulid.ULID, name string, w, h int) roomAnswer {
	return roomAnswer{
		columns: []string{"id", "name", "width", "height"},
		values:  []driver.Value{id.Bytes(), name, int64(w), int64(h)},
	}
}

var pickerMapColumns = []string{"id", "name", "file_name", "width", "height", "tile_gen", "tile_state", "tile_attempts"}

func pickerMapAnswer(id ulid.ULID, name string, w, h int) roomAnswer {
	return roomAnswer{
		columns: pickerMapColumns,
		values:  []driver.Value{id.Bytes(), name, name + ".png", int64(w), int64(h), testMapGen.Bytes(), nil, int64(0)},
	}
}
func buildingMapAnswer(id ulid.ULID, name string) roomAnswer {
	return roomAnswer{
		columns: pickerMapColumns,
		values:  []driver.Value{id.Bytes(), name, name, nil, nil, nil, "pending", int64(0)},
	}
}
func failedMapAnswer(id ulid.ULID, name string, attempts int) roomAnswer {
	return roomAnswer{
		columns: pickerMapColumns,
		values:  []driver.Value{id.Bytes(), name, name, nil, nil, nil, "failed", int64(attempts)},
	}
}
func tableRoomAnswer() roomAnswer {
	return getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false)
}
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
func TestTheTableFragmentsAreTheGMsAlone(t *testing.T) {
	for name, handler := range map[string]func(*App) http.HandlerFunc{
		"layers":   func(a *App) http.HandlerFunc { return a.RoomLayersFragment },
		"grid":     func(a *App) http.HandlerFunc { return a.RoomGridFragment },
		"settings": func(a *App) http.HandlerFunc { return a.RoomSettingsFragment },
		"maps":     func(a *App) http.HandlerFunc { return a.RoomMapsFragment },
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
	trigger := rec.Header().Get("HX-Trigger")
	if !strings.Contains(trigger, "alert") {
		t.Errorf("no alert on the refusal: %q", trigger)
	}
	if !strings.Contains(trigger, "change the active layer") {
		t.Errorf("the alert does not say what was refused: %q", trigger)
	}
}
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
	if trigger := rec.Header().Get("HX-Trigger"); !strings.Contains(trigger, "no longer in your library") {
		t.Errorf("the refusal reads %q", trigger)
	}
}
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
func pickerRequest(t *testing.T, app *App, handler http.HandlerFunc, path string, query string) *httptest.ResponseRecorder {
	t.Helper()
	layer := firstLayer(t, app)
	url := path + "?room=" + testRoomID.String() + "&layer=" + layer.String() + query
	return tableRequest(t, handler, http.MethodGet, url, nil, nil, session.UserSession{UserID: testOwnerID})
}
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
	if !strings.Contains(body, `hx-post="/rooms/`+testRoomID.String()+`/layers/`+firstLayer(t, app).String()+`/map"`) {
		t.Errorf("the card posts somewhere else:\n%s", body)
	}
}
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
	if got := read.args[1]; got != "%death%" {
		t.Errorf("the bound term is %v, want %%death%%", got)
	}
}
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
	if !strings.Contains(body, "/fragment/room/map-card?room="+testRoomID.String()) {
		t.Errorf("the card does not name the fragment it refetches:\n%s", body)
	}
}
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
	if !strings.Contains(body, `hx-target="closest room-map-card"`) {
		t.Errorf("the retry does not replace its own card:\n%s", body)
	}
}
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
func TestABadCellSizeComesBackIntoTheFormsErrorBlock(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)
	rec := tableRequest(t, app.SetRoomGrid, http.MethodPost, "/rooms/"+testRoomID.String()+"/grid",
		map[string]string{"id": testRoomID.String()},
		url.Values{
			"gridType": {"square"}, "gridLines": {"solid"}, "cellSize": {"4"},
			"offsetX": {"0"}, "offsetY": {"0"},
			"color": {"#000000FF"}, "snap": {"cells"}, "feetPerCell": {"5"}, "units": {"feet"},
			"diagonals": {"equal"},
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
func TestASavedGridClearsTheMessageTheLastAttemptLeft(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)
	rec := tableRequest(t, app.SetRoomGrid, http.MethodPost, "/rooms/"+testRoomID.String()+"/grid",
		map[string]string{"id": testRoomID.String()},
		url.Values{
			"gridType": {"hexPointy"}, "gridLines": {"dashed"}, "cellSize": {"70"},
			"offsetX": {"12"}, "offsetY": {"-4"},
			"color": {"#3355ffcc"}, "snap": {"halfCells"}, "feetPerCell": {"10"}, "units": {"miles"},
			"diagonals": {"alternating"}, "pawnLabels": {"none"},
		}, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Could not save") {
		t.Errorf("a good save still shows an error:\n%s", rec.Body.String())
	}
	view := tableView(t, app)
	want := room.Grid{
		Type: room.GridHexPointy, Lines: room.GridLinesDashed, CellSize: 70, OffsetX: 12, OffsetY: -4,
		Color: "#3355FFCC", Snap: room.SnapHalfCells, FeetPerCell: 10, Units: room.UnitsMiles,
		Diagonals: room.DiagonalsAlternating,
	}
	if view.Table.Grid != want {
		t.Errorf("the grid is %+v, want %+v", view.Table.Grid, want)
	}
	if view.Table.PawnLabels != room.LabelsDefault {
		t.Errorf("a stray pawnLabels in the grid form changed the table options to %q", view.Table.PawnLabels)
	}
}
func TestASavedSettingsFormLeavesTheGridAlone(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)
	before := tableView(t, app).Table.Grid
	rec := tableRequest(t, app.SetRoomSettings, http.MethodPost, "/rooms/"+testRoomID.String()+"/settings",
		map[string]string{"id": testRoomID.String()},
		url.Values{
			"pawnLabels": {"none"}, "initiativeGrouping": {"individual"},
			"playersCanDraw": {"on"}, "fogPrefill": {"on"},
			"cellSize": {"9999"}, "gridLines": {"off"},
		}, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	view := tableView(t, app)
	if view.Table.PawnLabels != room.LabelsNone || view.Table.InitiativeGrouping != room.GroupIndividual {
		t.Errorf("the options are %q / %q", view.Table.PawnLabels, view.Table.InitiativeGrouping)
	}
	if !view.Table.PlayersCanDraw || !view.Table.FogPrefill {
		t.Errorf("the toggles are %v / %v", view.Table.PlayersCanDraw, view.Table.FogPrefill)
	}
	if view.Table.Grid != before {
		t.Errorf("a stray grid field in the settings form moved the grid to %+v, want %+v", view.Table.Grid, before)
	}
}
func TestABadSettingIsRefusedIntoTheSettingsErrorBlock(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)
	rec := tableRequest(t, app.SetRoomSettings, http.MethodPost, "/rooms/"+testRoomID.String()+"/settings",
		map[string]string{"id": testRoomID.String()},
		url.Values{"pawnLabels": {"loud"}, "initiativeGrouping": {"grouped"}},
		session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "pawn label setting") {
		t.Errorf("the message is not the core's:\n%s", body)
	}
	if rec.Header().Get("HX-Trigger") != "" {
		t.Errorf("a form error also opened the alert modal: %q", rec.Header().Get("HX-Trigger"))
	}
}
func tableView(t *testing.T, app *App) *hub.TableView {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	view, ok := app.Hub.Table(ctx, testRoomID)
	if !ok {
		t.Fatal("the table is gone")
	}
	return view
}
func TestAnUncheckedToggleReadsAsFalse(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/rooms/x/settings", strings.NewReader(url.Values{
		"pawnLabels": {"default"}, "initiativeGrouping": {"grouped"},
	}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	options := tableOptionsForm(r)
	if options.PawnLabels != room.LabelsDefault {
		t.Errorf("the labels read as %q", options.PawnLabels)
	}
	if options.PlayersCanDraw || options.FogPrefill {
		t.Errorf("an absent toggle read as on: %+v", options)
	}
}
func TestACompleteGridFormIsAccepted(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/rooms/x/grid", strings.NewReader(url.Values{
		"gridType": {"hexFlat"}, "gridLines": {"solid"}, "cellSize": {"64"},
		"offsetX": {"0"}, "offsetY": {"0"},
		"color": {"#000000FF"}, "snap": {"cells"}, "feetPerCell": {"5"}, "units": {"kilometres"},
		"diagonals": {"equal"},
	}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	grid, problems := gridForm(r)
	if len(problems) != 0 {
		t.Fatalf("a complete form was refused: %v", problems)
	}
	if grid.Lines != room.GridLinesSolid {
		t.Errorf("the line style read as %q", grid.Lines)
	}
	if grid.Type != room.GridHexFlat {
		t.Errorf("the grid type read as %q", grid.Type)
	}
	if grid.Units != room.UnitsKilometres {
		t.Errorf("the unit read as %q", grid.Units)
	}
}
func TestAGridWithNoLineStyleIsRefused(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)
	rec := tableRequest(t, app.SetRoomGrid, http.MethodPost, "/rooms/"+testRoomID.String()+"/grid",
		map[string]string{"id": testRoomID.String()},
		url.Values{
			"gridType": {"square"}, "cellSize": {"64"}, "offsetX": {"0"}, "offsetY": {"0"},
			"color": {"#000000FF"}, "snap": {"cells"}, "feetPerCell": {"5"}, "units": {"feet"},
			"diagonals": {"equal"},
		}, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "grid line style") {
		t.Errorf("the message is not the core's:\n%s", body)
	}
}
func TestAGridWithNoTypeOrNoUnitIsRefused(t *testing.T) {
	for field, want := range map[string]string{"gridType": "grid type", "units": "distance unit"} {
		form := url.Values{
			"gridType": {"square"}, "gridLines": {"solid"}, "cellSize": {"64"},
			"offsetX": {"0"}, "offsetY": {"0"},
			"color": {"#000000FF"}, "snap": {"cells"}, "feetPerCell": {"5"}, "units": {"feet"},
			"diagonals": {"equal"},
		}
		form.Del(field)
		db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
		app := tableApp(t, db)
		rec := tableRequest(t, app.SetRoomGrid, http.MethodPost, "/rooms/"+testRoomID.String()+"/grid",
			map[string]string{"id": testRoomID.String()}, form, session.UserSession{UserID: testOwnerID})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("a form with no %s returned %d, want 422; body: %s", field, rec.Code, rec.Body.String())
		}
		if body := rec.Body.String(); !strings.Contains(body, want) {
			t.Errorf("a form with no %s says:\n%s", field, body)
		}
	}
}
func TestAColourWithoutAHashIsStillAColour(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/rooms/x/grid", strings.NewReader(url.Values{
		"gridType": {"square"}, "cellSize": {"64"}, "offsetX": {"0"}, "offsetY": {"0"},
		"color": {"  3355ffcc  "},
		"snap":  {"cells"}, "feetPerCell": {"5"}, "units": {"feet"}, "diagonals": {"equal"},
	}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	grid, problems := gridForm(r)
	if len(problems) != 0 {
		t.Fatalf("refused: %v", problems)
	}
	if grid.Color != "#3355FFCC" {
		t.Errorf("colour = %q, want #3355FFCC", grid.Color)
	}
}
func TestAFieldThatIsNotANumberIsCaughtBeforeTheCoreSeesIt(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/rooms/x/grid", strings.NewReader(url.Values{
		"gridType": {"square"}, "cellSize": {"sixty four"}, "offsetX": {"0"}, "offsetY": {"0"},
		"color": {"#000000FF"},
		"snap":  {"cells"}, "feetPerCell": {"5"}, "units": {"feet"}, "diagonals": {"equal"},
	}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_, problems := gridForm(r)
	if len(problems) != 1 || !strings.Contains(problems[0], "Cell size") {
		t.Errorf("problems = %v, want one about the cell size", problems)
	}
}
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
