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

	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

var (
	testSceneID      = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVTE")
	testOtherSceneID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVTF")
)

func countAnswer(n int64) roomAnswer {
	return roomAnswer{columns: []string{"count"}, values: []driver.Value{n}}
}
func statementsLike(db *roomDB, fragment string) []recordedCall {
	var out []recordedCall
	for _, call := range db.recorded() {
		if strings.Contains(call.query, fragment) {
			out = append(out, call)
		}
	}
	return out
}
func onlyStatement(t *testing.T, db *roomDB, fragment string) recordedCall {
	t.Helper()
	found := statementsLike(db, fragment)
	if len(found) != 1 {
		t.Fatalf("%d statements matching %q, want 1: %v", len(found), fragment, db.queries())
	}
	return found[0]
}
func TestSavingASceneWritesOneRowThatHoldsNobody(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer(), countAnswer(0)}}
	app := tableApp(t, db)
	rec := tableRequest(t, app.SaveScene, http.MethodPost, "/rooms/"+testRoomID.String()+"/scenes",
		map[string]string{"id": testRoomID.String()},
		url.Values{"name": {"  The prepped ambush  "}}, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	insert := onlyStatement(t, db, "INSERT INTO scenes")
	if len(insert.args) != 5 {
		t.Fatalf("the insert carries %d values, want 5: %v", len(insert.args), insert.args)
	}
	if got, _ := insert.args[2].(string); got != "The prepped ambush" {
		t.Errorf("the name was written as %q", got)
	}
	body, ok := insert.args[3].([]byte)
	if !ok {
		t.Fatalf("the body is %T, want bytes", insert.args[3])
	}
	scene, err := room.Unmarshal(body)
	if err != nil {
		t.Fatalf("the body does not read back the way a room snapshot does: %v", err)
	}
	if len(scene.Players) != 0 {
		t.Errorf("the saved scene names %d players", len(scene.Players))
	}
	if scene.Room != (room.RoomInfo{}) {
		t.Errorf("the saved scene carries the room it came from: %+v", scene.Room)
	}
	if len(statementsLike(db, "SET scene_id = ?")) != 1 {
		t.Errorf("the room did not adopt the scene it just saved: %v", db.queries())
	}
	trigger := rec.Header().Get("HX-Trigger")
	for _, want := range []string{"modal:close", "flash:toast", "room:scenes"} {
		if !strings.Contains(trigger, want) {
			t.Errorf("the save did not raise %s: %q", want, trigger)
		}
	}
}
func TestOnlyTheGMSavesAScene(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)
	rec := tableRequest(t, app.SaveScene, http.MethodPost, "/rooms/"+testRoomID.String()+"/scenes",
		map[string]string{"id": testRoomID.String()},
		url.Values{"name": {"The prepped ambush"}}, memberSession(testRoomID))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}
	for _, q := range db.queries() {
		if strings.Contains(q, "CreateScene") || strings.Contains(q, "SetRoomScene") {
			t.Errorf("a player's save reached the database: %v", db.queries())
		}
	}
}
func TestASceneNeedsANameAndLeavesNoRowWithoutOne(t *testing.T) {
	for name, value := range map[string]string{
		"nothing at all": "",
		"only spaces":    "   ",
		"a long one":     strings.Repeat("a", room.NameLimit+1),
	} {
		t.Run(name, func(t *testing.T) {
			db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
			app := tableApp(t, db)
			rec := tableRequest(t, app.SaveScene, http.MethodPost, "/rooms/"+testRoomID.String()+"/scenes",
				map[string]string{"id": testRoomID.String()},
				url.Values{"name": {value}}, session.UserSession{UserID: testOwnerID})
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422; body: %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "errors-"+pages.SceneSavePanel) {
				t.Errorf("the refusal did not land in the form's own error block:\n%s", rec.Body.String())
			}
			if len(statementsLike(db, "INSERT INTO scenes")) != 0 {
				t.Errorf("a refused save still wrote a row: %v", db.queries())
			}
			if rec.Header().Get("HX-Trigger") != "" {
				t.Errorf("a form error also opened the alert modal: %q", rec.Header().Get("HX-Trigger"))
			}
		})
	}
}
func TestAFullShelfRefusesAnotherSceneAndSaysSo(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer(), countAnswer(int64(room.ScenesMax))}}
	app := tableApp(t, db)
	rec := tableRequest(t, app.SaveScene, http.MethodPost, "/rooms/"+testRoomID.String()+"/scenes",
		map[string]string{"id": testRoomID.String()},
		url.Values{"name": {"One too many"}}, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "scenes saved") {
		t.Errorf("the refusal does not say the shelf is full:\n%s", rec.Body.String())
	}
	if len(statementsLike(db, "INSERT INTO scenes")) != 0 {
		t.Errorf("a refused save still wrote a row: %v", db.queries())
	}
}
func TestTheSceneFragmentsAreTheGMsAlone(t *testing.T) {
	for name, handler := range map[string]func(*App) http.HandlerFunc{
		"scenes":     func(a *App) http.HandlerFunc { return a.RoomScenesFragment },
		"scene/save": func(a *App) http.HandlerFunc { return a.RoomSceneSaveFragment },
	} {
		t.Run(name, func(t *testing.T) {
			db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
			app := tableApp(t, db)
			rec := tableRequest(t, handler(app), http.MethodGet,
				"/fragment/room/"+name+"?room="+testRoomID.String(),
				nil, nil, memberSession(testRoomID))
			if rec.Code != http.StatusNotFound {
				t.Errorf("a player got %d from the %s fragment, want 404", rec.Code, name)
			}
		})
	}
}
func sceneBody(t *testing.T, withMap bool) []byte {
	t.Helper()
	s := room.NewState(testRoomID, "prep", room.Env{})
	s.Table.Layers[0].Name = "Ambush point"
	s.Table.Grid.CellSize = 96
	if withMap {
		s.Table.Layers[0].Map = &room.MapRef{
			AssetID: testMapID, Gen: testOtherMap, Width: 1024, Height: 1024, TileSize: 512, MaxZoom: 1,
		}
	}
	body, err := s.ExportScene()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	return body
}
func sceneRowAnswer(id ulid.ULID, name string, autosave bool, body []byte) roomAnswer {
	return roomAnswer{
		columns: []string{"id", "owner_id", "name", "body", "autosave", "preview_id", "created_at", "updated_at"},
		values: []driver.Value{
			id.Bytes(), testOwnerID.Bytes(), name, body, autosave, nil, time.Now(), time.Now(),
		},
	}
}
func roomWithSceneAnswer(scene ulid.ULID) roomAnswer {
	answer := tableRoomAnswer()
	for i, column := range answer.columns {
		if column == "scene_id" {
			answer.values[i] = scene.Bytes()
		}
	}
	return answer
}
func indexOfStatement(db *roomDB, fragment string) int {
	for i, call := range db.recorded() {
		if strings.Contains(call.query, fragment) {
			return i
		}
	}
	return -1
}
func openSceneRequest(t *testing.T, app *App, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()
	return tableRequest(t, app.OpenScene, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/scenes/"+testSceneID.String()+"/open",
		map[string]string{"id": testRoomID.String(), "scene": testSceneID.String()}, nil, sess)
}
func TestOpeningASceneReplacesTheTableAndMarksItOpen(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		tableRoomAnswer(),
		sceneRowAnswer(testSceneID, "The prepped ambush", false, sceneBody(t, true)),
		pyramidAnswer(testOwnerID, testMapGen, 4096, 4096, 512, 3),
	}}
	app := tableApp(t, db)
	rec := openSceneRequest(t, app, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	view := tableView(t, app)
	if len(view.Table.Layers) != 1 || view.Table.Layers[0].Name != "Ambush point" {
		t.Fatalf("the table is still the room's own: %+v", view.Table.Layers)
	}
	if view.Table.Grid.CellSize != 96 {
		t.Errorf("the grid is the room's and not the scene's: %+v", view.Table.Grid)
	}
	ref := view.Table.Layers[0].Map
	if ref == nil {
		t.Fatal("the loaded floor has no map")
	}
	if ref.Gen != testMapGen {
		t.Errorf("the map is on generation %s, want the one the library just answered with, %s", ref.Gen, testMapGen)
	}
	if len(statementsLike(db, "SET scene_id = ?")) != 1 {
		t.Errorf("the room does not remember which scene is open: %v", db.queries())
	}
	if toast := toastFrom(t, rec); !strings.Contains(toast, "The prepped ambush") {
		t.Errorf("the toast does not name the scene: %q", toast)
	}
}
func TestOpeningASceneWhoseMapIsGoneSaysWhichFloorLostIt(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		tableRoomAnswer(),
		sceneRowAnswer(testSceneID, "The prepped ambush", false, sceneBody(t, true)),
	}}
	app := tableApp(t, db)
	rec := openSceneRequest(t, app, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	toast := toastFrom(t, rec)
	if !strings.Contains(toast, "Ambush point") {
		t.Errorf("nothing said which floor lost its map: %q", toast)
	}
	if view := tableView(t, app); view.Table.Layers[0].Map != nil {
		t.Error("the floor still points at a map whose tiles are gone")
	}
}
func TestAnotherUsersSceneIsNotFoundRatherThanForbidden(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)
	rec := openSceneRequest(t, app, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
	if len(statementsLike(db, "SET scene_id = ?")) != 0 {
		t.Errorf("a scene that is not there was marked open anyway: %v", db.queries())
	}
}
func TestOnlyTheGMOpensAScene(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)
	rec := openSceneRequest(t, app, memberSession(testRoomID))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}
	for _, q := range db.queries() {
		if strings.Contains(q, "GetScene") || strings.Contains(q, "SetRoomScene") {
			t.Errorf("a player's open reached the database: %v", db.queries())
		}
	}
}
func openAnotherRequest(t *testing.T, app *App) *httptest.ResponseRecorder {
	t.Helper()
	return tableRequest(t, app.OpenScene, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/scenes/"+testOtherSceneID.String()+"/open",
		map[string]string{"id": testRoomID.String(), "scene": testOtherSceneID.String()}, nil,
		session.UserSession{UserID: testOwnerID})
}
func TestOpeningAnotherSceneWritesBackTheOneThatAutosaves(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		roomWithSceneAnswer(testSceneID),
		sceneRowAnswer(testOtherSceneID, "Town square", false, sceneBody(t, false)),
		sceneRowAnswer(testSceneID, "The prepped ambush", true, sceneBody(t, false)),
	}}
	app := tableApp(t, db)
	rec := openAnotherRequest(t, app)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	back := onlyStatement(t, db, "UPDATE scenes")
	if !strings.Contains(back.query, "SET body = ?") {
		t.Fatalf("the write-back is not a body write: %q", back.query)
	}
	if id, ok := boundRoomID(back.args[2]); !ok || id != testSceneID {
		t.Errorf("the write-back went to %v, want the scene that was open", back.args[2])
	}
	if indexOfStatement(db, "UPDATE scenes") > indexOfStatement(db, "SET scene_id = ?") {
		t.Error("the room adopted the new scene before the old one was written back")
	}
}
func TestOpeningAnotherSceneLeavesOneThatDoesNotAutosaveAlone(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		roomWithSceneAnswer(testSceneID),
		sceneRowAnswer(testOtherSceneID, "Town square", false, sceneBody(t, false)),
		sceneRowAnswer(testSceneID, "The prepped ambush", false, sceneBody(t, false)),
	}}
	app := tableApp(t, db)
	if rec := openAnotherRequest(t, app); rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	if len(statementsLike(db, "UPDATE scenes")) != 0 {
		t.Errorf("six goblins at 3 hit points were written over six at 11: %v", db.queries())
	}
}
func TestARoomWhoseOpenSceneWasDeletedStillOpensAnother(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		roomWithSceneAnswer(testSceneID),
		sceneRowAnswer(testOtherSceneID, "Town square", false, sceneBody(t, false)),
	}}
	app := tableApp(t, db)
	if rec := openAnotherRequest(t, app); rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	if len(statementsLike(db, "UPDATE scenes")) != 0 {
		t.Errorf("a dangling scene_id was written to: %v", db.queries())
	}
}
func TestSaveChangesWritesTheOpenSceneBackWhateverItsFlagSays(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		roomWithSceneAnswer(testSceneID),
		sceneRowAnswer(testSceneID, "The prepped ambush", false, sceneBody(t, false)),
	}}
	app := tableApp(t, db)
	rec := tableRequest(t, app.SaveSceneChanges, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/scenes/"+testSceneID.String()+"/save",
		map[string]string{"id": testRoomID.String(), "scene": testSceneID.String()}, nil,
		session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	onlyStatement(t, db, "UPDATE scenes")
	if toast := toastFrom(t, rec); !strings.Contains(toast, "The prepped ambush") {
		t.Errorf("the toast does not name the scene: %q", toast)
	}
}
func TestOnlyTheOpenSceneCanBeSavedOver(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{roomWithSceneAnswer(testSceneID)}}
	app := tableApp(t, db)
	rec := tableRequest(t, app.SaveSceneChanges, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/scenes/"+testOtherSceneID.String()+"/save",
		map[string]string{"id": testRoomID.String(), "scene": testOtherSceneID.String()}, nil,
		session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
	if len(statementsLike(db, "UPDATE scenes")) != 0 {
		t.Errorf("a scene nobody is looking at was overwritten: %v", db.queries())
	}
}
func clearRequest(t *testing.T, app *App) *httptest.ResponseRecorder {
	t.Helper()
	return tableRequest(t, app.ClearTabletop, http.MethodPost,
		"/rooms/"+testRoomID.String()+"/tabletop/clear",
		map[string]string{"id": testRoomID.String()}, nil, session.UserSession{UserID: testOwnerID})
}
func fogTheFloor(t *testing.T, app *App) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := app.Hub.Dispatch(ctx, testRoomID, room.Actor{ID: testOwnerID, Role: room.RoleGM}, &room.FogAdd{
		Layer:  firstLayer(t, app),
		Kind:   room.ShapeRect,
		Mode:   room.FogHide,
		Points: []int{0, 0, 64, 64},
	})
	if err != nil {
		t.Fatalf("could not put a shape on the floor: %v", err)
	}
}
func TestClearingTheTabletopAutosavesTheSceneThatAsksForIt(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		roomWithSceneAnswer(testSceneID),
		sceneRowAnswer(testSceneID, "The haunted moor", true, sceneBody(t, false)),
	}}
	app := tableApp(t, db)
	fogTheFloor(t, app)
	if rec := clearRequest(t, app); rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	back := onlyStatement(t, db, "UPDATE scenes")
	if id, ok := boundRoomID(back.args[2]); !ok || id != testSceneID {
		t.Errorf("the write-back went to %v, want the scene that was open", back.args[2])
	}
	body, ok := back.args[0].([]byte)
	if !ok {
		t.Fatalf("the body was written as %T", back.args[0])
	}
	kept, err := room.Unmarshal(body)
	if err != nil {
		t.Fatalf("the written body does not decode: %v", err)
	}
	if len(kept.Fog) != 1 {
		t.Error("the table was written back after it was cleared, so the scene kept nothing")
	}
	if len(statementsLike(db, "SET scene_id = NULL")) != 1 {
		t.Errorf("the room still thinks a scene is open: %v", db.queries())
	}
}
func TestClearingTheTabletopForgetsASceneThatDoesNotAutosave(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		roomWithSceneAnswer(testSceneID),
		sceneRowAnswer(testSceneID, "The prepped ambush", false, sceneBody(t, false)),
	}}
	app := tableApp(t, db)
	fogTheFloor(t, app)
	if rec := clearRequest(t, app); rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	if len(statementsLike(db, "SET scene_id = NULL")) != 1 {
		t.Errorf("the room still thinks a scene is open: %v", db.queries())
	}
	if len(statementsLike(db, "UPDATE scenes")) != 0 {
		t.Errorf("clearing the tabletop wrote over a scene that never asked for it: %v", db.queries())
	}
}
func TestClearingTheTabletopWithNoSceneOpenReadsNoScene(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)
	if rec := clearRequest(t, app); rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	if len(statementsLike(db, "FROM scenes")) != 0 {
		t.Errorf("a room with no scene open still went looking for one: %v", db.queries())
	}
	if len(statementsLike(db, "UPDATE scenes")) != 0 {
		t.Errorf("a room with no scene open wrote to one: %v", db.queries())
	}
}
func TestClosingTheRoomWritesTheSceneBackBeforeItCloses(t *testing.T) {
	for name, close := range map[string]func(*App) http.HandlerFunc{
		"close":  func(a *App) http.HandlerFunc { return a.CloseRoom },
		"delete": func(a *App) http.HandlerFunc { return a.DeleteRoom },
	} {
		t.Run(name, func(t *testing.T) {
			db := &roomDB{rows: 1, answers: []roomAnswer{
				roomWithSceneAnswer(testSceneID),
				sceneRowAnswer(testSceneID, "The prepped ambush", true, sceneBody(t, false)),
			}}
			app := tableApp(t, db)
			rec := tableRequest(t, close(app), http.MethodPost, "/rooms/"+testRoomID.String(),
				map[string]string{"id": testRoomID.String()}, nil, session.UserSession{UserID: testOwnerID})
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
			}
			back := indexOfStatement(db, "UPDATE scenes")
			if back < 0 {
				t.Fatalf("the kept scene was not written back: %v", db.queries())
			}
			gone := indexOfStatement(db, "closed_at = NOW()")
			if gone < 0 {
				gone = indexOfStatement(db, "DELETE FROM rooms")
			}
			if back > gone {
				t.Error("the export ran after the room was closed, and a closed room cannot be loaded")
			}
		})
	}
}
func TestMarkingWhereThePartyStartsUsesTheFloorTheGMIsLookingAt(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer(), tableRoomAnswer()}}
	app := tableApp(t, db)
	layer := firstLayer(t, app)
	rec := tableRequest(t, app.SetPartyStart, http.MethodPost, "/rooms/"+testRoomID.String()+"/party-start",
		map[string]string{"id": testRoomID.String()},
		url.Values{"layer": {layer.String()}, "x": {"640"}, "y": {"320"}},
		session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	start := tableView(t, app).Table.Layers[0].PartyStart
	if start == nil || start.X != 640 || start.Y != 320 {
		t.Fatalf("the mark is %+v", start)
	}
	rec = tableRequest(t, app.SetPartyStart, http.MethodPost, "/rooms/"+testRoomID.String()+"/party-start",
		map[string]string{"id": testRoomID.String()},
		url.Values{"layer": {layer.String()}}, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	if start := tableView(t, app).Table.Layers[0].PartyStart; start != nil {
		t.Fatalf("a post with no coordinates left %+v behind", start)
	}
}
func TestThePartyStartIsTheGMsAlone(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)
	rec := tableRequest(t, app.SetPartyStart, http.MethodPost, "/rooms/"+testRoomID.String()+"/party-start",
		map[string]string{"id": testRoomID.String()},
		url.Values{"x": {"640"}, "y": {"320"}}, memberSession(testRoomID))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}
}
func TestTheTableMenuIsRenderedForTheRoleThatAsks(t *testing.T) {
	for name, sess := range map[string]session.UserSession{
		"the GM":   {UserID: testOwnerID},
		"a player": memberSession(testRoomID),
	} {
		t.Run(name, func(t *testing.T) {
			db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
			app := tableApp(t, db)
			rec := tableRequest(t, app.RoomTableMenuFragment, http.MethodGet,
				"/fragment/room/table-menu?room="+testRoomID.String(), nil, nil, sess)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			wheel := strings.Contains(rec.Body.String(), "data-table-menu")
			if name == "the GM" && !wheel {
				t.Errorf("the GM got no wheel:\n%s", rec.Body.String())
			}
			if name == "a player" && wheel {
				t.Errorf("a player got the GM's wheel:\n%s", rec.Body.String())
			}
		})
	}
}
func sceneVerb(t *testing.T, app *App, handler http.HandlerFunc, method string, form url.Values, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()
	return tableRequest(t, handler, method, "/scenes/"+testSceneID.String(),
		map[string]string{"scene": testSceneID.String()}, form, sess)
}
func TestEachSceneVerbIsScopedToItsOwner(t *testing.T) {
	for name, verb := range map[string]struct {
		handler func(*App) http.HandlerFunc
		method  string
		form    url.Values
	}{
		"rename":    {func(a *App) http.HandlerFunc { return a.RenameScene }, http.MethodPatch, url.Values{"name": {"Elsewhere"}}},
		"duplicate": {func(a *App) http.HandlerFunc { return a.DuplicateScene }, http.MethodPost, nil},
		"delete":    {func(a *App) http.HandlerFunc { return a.DeleteScene }, http.MethodDelete, nil},
		"autosave":  {func(a *App) http.HandlerFunc { return a.SetSceneAutosave }, http.MethodPost, url.Values{"autosave": {"on"}}},
	} {
		t.Run(name, func(t *testing.T) {
			db := &roomDB{rows: 0, answers: []roomAnswer{countAnswer(0)}}
			app := tableApp(t, db)
			rec := sceneVerb(t, app, verb.handler(app), verb.method, verb.form, session.UserSession{UserID: testMemberID})
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404 -- a 403 would confirm the row exists; body: %s", rec.Code, rec.Body.String())
			}
		})
	}
}
func TestDeletingASceneForgetsItInEveryRoomThatHadItOpen(t *testing.T) {
	db := &roomDB{rows: 1}
	app := tableApp(t, db)
	rec := sceneVerb(t, app, app.DeleteScene, http.MethodDelete, nil, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	onlyStatement(t, db, "DELETE FROM scenes")
	if len(statementsLike(db, "SET scene_id = NULL")) != 1 {
		t.Errorf("the room still points at a scene that is gone: %v", db.queries())
	}
	if len(statementsLike(db, "UPDATE scenes")) != 0 {
		t.Errorf("deleting a scene wrote to it first: %v", db.queries())
	}
}
func TestDuplicatingCountsAgainstTheShelf(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{countAnswer(int64(room.ScenesMax))}}
	app := tableApp(t, db)
	rec := sceneVerb(t, app, app.DuplicateScene, http.MethodPost, nil, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body: %s", rec.Code, rec.Body.String())
	}
	if len(statementsLike(db, "INSERT INTO scenes")) != 0 {
		t.Errorf("a copy was made over the cap: %v", db.queries())
	}
}
func TestDuplicatingCopiesTheRowUnderANewName(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{countAnswer(2), sceneRowAnswer(testSceneID, "The prepped ambush", true, sceneBody(t, false))}}
	app := tableApp(t, db)
	rec := sceneVerb(t, app, app.DuplicateScene, http.MethodPost, nil, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	copied := onlyStatement(t, db, "INSERT INTO scenes")
	name, _ := copied.args[1].(string)
	if !strings.Contains(name, "The prepped ambush") || name == "The prepped ambush" {
		t.Errorf("the copy is called %q, which does not tell it apart from the original", name)
	}
	if id, ok := boundRoomID(copied.args[0]); !ok || id == testSceneID {
		t.Errorf("the copy reuses the original's id: %v", copied.args[0])
	}
}
func TestRenamingASceneNeedsAName(t *testing.T) {
	db := &roomDB{rows: 1}
	app := tableApp(t, db)
	rec := sceneVerb(t, app, app.RenameScene, http.MethodPatch, url.Values{"name": {"   "}}, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body: %s", rec.Code, rec.Body.String())
	}
	if len(db.recorded()) != 0 {
		t.Errorf("an empty name reached the database: %v", db.queries())
	}
}
