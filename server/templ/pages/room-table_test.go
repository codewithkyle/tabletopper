package pages

import (
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"tabletopper/internal/room"
	"tabletopper/internal/uievents"
)

const testTableRoomID = "01BX5ZZKBKACTAV9WEVGEMMVT0"

func testLayer(name string, pawns int) RoomLayer {
	return RoomLayer{ID: "01BX5ZZKBKACTAV9WEVGEMMVT1", Name: name, Pawns: pawns}
}
func TestTheDeleteWarningCountsWhatItWouldTakeWithIt(t *testing.T) {
	data := RoomLayersData{RoomID: testTableRoomID}
	empty := data.RemovePrompt(testLayer("Cellar", 0))
	if strings.Contains(empty, "pawn") {
		t.Errorf("an empty layer threatens pawns: %q", empty)
	}
	if !strings.Contains(empty, "Cellar") {
		t.Errorf("the warning does not name the layer: %q", empty)
	}
	if got := data.RemovePrompt(testLayer("Cellar", 1)); !strings.Contains(got, "the 1 pawn on it") {
		t.Errorf("one pawn reads %q", got)
	}
	if got := data.RemovePrompt(testLayer("Cellar", 9)); !strings.Contains(got, "the 9 pawns on it") {
		t.Errorf("nine pawns read %q", got)
	}
}
func TestAnEmptyLayerSaysNothingAboutPawns(t *testing.T) {
	if got := testLayer("Cellar", 0).PawnLabel(); got != "" {
		t.Errorf("PawnLabel = %q, want empty", got)
	}
	if got := testLayer("Cellar", 2).PawnLabel(); got != "2 pawns" {
		t.Errorf("PawnLabel = %q", got)
	}
}
func TestAMissingMapIsNotTheSameAsNoMap(t *testing.T) {
	none := RoomLayer{}
	if none.HasMap() || none.MapMissing() {
		t.Error("a layer with no map claims to have one")
	}
	gone := RoomLayer{MapID: "01BX5ZZKBKACTAV9WEVGEMMVT2"}
	if !gone.HasMap() || !gone.MapMissing() {
		t.Error("a layer whose asset has gone is not reported as missing")
	}
	here := RoomLayer{MapID: "01BX5ZZKBKACTAV9WEVGEMMVT2", MapName: "Death House"}
	if !here.HasMap() || here.MapMissing() {
		t.Error("a layer with a map is reported as missing")
	}
}
func TestTheManagerPointsAtTheLayerRoutes(t *testing.T) {
	data := RoomLayersData{RoomID: testTableRoomID}
	l := testLayer("Cellar", 0)
	base := "/rooms/" + testTableRoomID + "/layers"
	for name, got := range map[string]string{
		"self":     data.Path(),
		"add":      data.AddPath(),
		"layer":    data.LayerPath(l),
		"name":     data.NamePath(l),
		"move":     data.MovePath(l),
		"activate": data.ActivatePath(l),
		"map":      data.MapPath(l),
		"picker":   data.ChooseMapPath(l),
	} {
		if !strings.Contains(got, testTableRoomID) {
			t.Errorf("the %s path does not name the room: %q", name, got)
		}
	}
	if data.AddPath() != base {
		t.Errorf("add posts to %q, want %q", data.AddPath(), base)
	}
	if data.LayerPath(l) != base+"/"+l.ID {
		t.Errorf("the layer is at %q", data.LayerPath(l))
	}
	if !strings.HasPrefix(data.ChooseMapPath(l), "/fragment/") {
		t.Errorf("the picker is not a fragment: %q", data.ChooseMapPath(l))
	}
}
func TestUpAndDownMoveOneStepEachWay(t *testing.T) {
	data := RoomLayersData{RoomID: testTableRoomID}
	l := RoomLayer{Index: 2}
	if got := data.MoveVals(l, 1); !strings.Contains(got, `"3"`) {
		t.Errorf("up sends %q, want index 3", got)
	}
	if got := data.MoveVals(l, -1); !strings.Contains(got, `"1"`) {
		t.Errorf("down sends %q, want index 1", got)
	}
}
func TestOnlyTheOddlySizedFloorIsWarnedAbout(t *testing.T) {
	data := RoomLayersData{
		RoomID: testTableRoomID,
		Layers: []RoomLayer{
			{ID: "a", Name: "Ground floor", MapID: "m1", MapName: "Ground", Width: 4000, Height: 3000},
			{ID: "b", Name: "First floor", MapID: "m2", MapName: "First", Width: 4000, Height: 3000},
			{ID: "c", Name: "Cellar", MapID: "m3", MapName: "Cellar", Width: 2048, Height: 2048, Mismatch: true},
		},
	}
	page := renderToString(t, RoomLayers(data))
	if got := strings.Count(page, "will not line up"); got != 1 {
		t.Errorf("%d size warnings, want 1:\n%s", got, page)
	}
}
func testGridData() RoomGridData {
	return RoomGridData{RoomID: testTableRoomID, CellSize: 64, FeetPerCell: 5, Color: "#000000FF"}
}
func testSettingsData() RoomSettingsData {
	return RoomSettingsData{RoomID: testTableRoomID, PawnLabels: "default", InitiativeGrouping: "grouped"}
}
func TestBothTableFormsRouteTheirRejectionToTheirOwnErrorBlock(t *testing.T) {
	if RoomGridPanel == RoomSettingsPanel {
		t.Fatalf("both windows swap into #errors-%s, so whichever is open second wins", RoomGridPanel)
	}
	for name, form := range map[string]struct {
		page  string
		panel string
		path  string
	}{
		"grid":     {renderToString(t, RoomGrid(testGridData())), RoomGridPanel, "/rooms/" + testTableRoomID + "/grid"},
		"settings": {renderToString(t, RoomSettings(testSettingsData())), RoomSettingsPanel, "/rooms/" + testTableRoomID + "/settings"},
	} {
		block := "#errors-" + form.panel
		for _, want := range []string{
			`hx-post="` + form.path + `"`,
			`hx-target="` + block + `"`,
			`hx-swap="outerHTML"`,
			`hx-status:422="target:` + block + `,swap:outerHTML"`,
		} {
			if !strings.Contains(form.page, want) {
				t.Errorf("the %s form is missing %s:\n%s", name, want, form.page)
			}
		}
		if !strings.Contains(form.page, `id="errors-`+form.panel+`"`) {
			t.Errorf("the %s form has no error block to swap into", name)
		}
	}
}
func TestNeitherTableFormRedrawsItselfOnItsOwnSave(t *testing.T) {
	grid := renderToString(t, RoomGrid(testGridData()))
	settings := renderToString(t, RoomSettings(testSettingsData()))
	for name, page := range map[string]string{"grid": grid, "settings": settings} {
		if strings.Contains(page, uievents.Tabletop) {
			t.Errorf("the %s form refetches on its own save:\n%s", name, page)
		}
	}
	manager := renderToString(t, RoomLayers(RoomLayersData{RoomID: testTableRoomID}))
	if !strings.Contains(manager, uievents.Tabletop+" from:window") {
		t.Errorf("the layer manager does not refetch:\n%s", manager)
	}
}
func TestTheGridRedrawsWhenASceneLandsUnderIt(t *testing.T) {
	grid := renderToString(t, RoomGrid(testGridData()))
	if !strings.Contains(grid, uievents.Resync+" from:window") {
		t.Errorf("a scene load leaves the grid window showing the grid it replaced:\n%s", grid)
	}
	if !strings.Contains(grid, `hx-get="/fragment/room/grid?room=`+testTableRoomID+`"`) {
		t.Errorf("the grid window has nowhere to refetch from:\n%s", grid)
	}
	if strings.Contains(grid, `hx-trigger="load`) || strings.Contains(grid, `hx-trigger="`+uievents.Resync+`, load`) {
		t.Errorf("the grid refetches itself the moment it is swapped in, which never stops:\n%s", grid)
	}
}
func TestTheGridFormOffersExactlyTheProtocolsChoices(t *testing.T) {
	assertChoices(t, map[string][2][]string{
		"gridLines": {values(GridLineChoices()), room.GridLines("").Values()},
		"snap":      {values(GridSnapChoices()), room.Snap("").Values()},
		"diagonals": {values(GridDiagonalChoices()), room.Diagonals("").Values()},
	})
}
func TestTheSettingsFormOffersExactlyTheProtocolsChoices(t *testing.T) {
	assertChoices(t, map[string][2][]string{
		"pawnLabels":         {values(PawnLabelChoices()), room.PawnLabels("").Values()},
		"initiativeGrouping": {values(InitiativeGroupingChoices()), room.InitiativeGrouping("").Values()},
	})
}
func assertChoices(t *testing.T, pairs map[string][2][]string) {
	t.Helper()
	for name, pair := range pairs {
		got, want := pair[0], pair[1]
		if len(got) != len(want) {
			t.Errorf("%s offers %v, the protocol takes %v", name, got, want)
			continue
		}
		for _, v := range want {
			if !contains(got, v) {
				t.Errorf("%s does not offer %q; it offers %v", name, v, got)
			}
		}
	}
}
func TestNeitherTableFormCarriesTheOthersFields(t *testing.T) {
	for name, pair := range map[string][2][]string{
		"grid":     {fieldNames(renderToString(t, RoomGrid(testGridData()))), []string{"cellSize", "color", "diagonals", "feetPerCell", "gridLines", "offsetX", "offsetY", "snap"}},
		"settings": {fieldNames(renderToString(t, RoomSettings(testSettingsData()))), []string{"fogPrefill", "initiativeGrouping", "pawnLabels", "playersCanDraw"}},
	} {
		if !slices.Equal(pair[0], pair[1]) {
			t.Errorf("the %s form posts %v, want %v", name, pair[0], pair[1])
		}
	}
}
func fieldNames(page string) []string {
	found := map[string]bool{}
	for _, m := range regexp.MustCompile(`name="([^"]+)"`).FindAllStringSubmatch(page, -1) {
		found[m[1]] = true
	}
	out := slices.Collect(maps.Keys(found))
	slices.Sort(out)
	return out
}
func TestTheGridFormOpensOnTheLineStyleTheTableIsOn(t *testing.T) {
	page := renderToString(t, RoomGrid(RoomGridData{
		RoomID: testTableRoomID, Lines: "dashed", CellSize: 64, FeetPerCell: 5, Color: "#000000FF",
	}))
	if !strings.Contains(page, `value="dashed" checked`) {
		t.Errorf("a dashed grid does not open on Dashed:\n%s", page)
	}
	if strings.Contains(page, `value="solid" checked`) {
		t.Errorf("a dashed grid also opens on Solid:\n%s", page)
	}
}
func TestTheColourIsPostedByTheTextFieldAndNotByThePicker(t *testing.T) {
	page := renderToString(t, RoomGrid(RoomGridData{
		RoomID: testTableRoomID, CellSize: 64, FeetPerCell: 5, Color: "#3366CCB3",
	}))
	for _, want := range []string{
		`name="color"`,
		`value="#3366CCB3"`,
		`pattern="` + GridColorPat + `"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the colour field is missing %s:\n%s", want, page)
		}
	}
	if strings.Contains(page, `type="color"`) {
		t.Errorf("the native colour input is still in the form:\n%s", page)
	}
	if strings.Contains(page, `name="color"`) && strings.Count(page, `name="color"`) != 1 {
		t.Errorf("%d colour fields, want 1:\n%s", strings.Count(page, `name="color"`), page)
	}
}
func TestTheColourControlOpensOnTheTablesColour(t *testing.T) {
	page := renderToString(t, RoomGrid(RoomGridData{
		RoomID: testTableRoomID, CellSize: 64, FeetPerCell: 5, Color: "#3366CCB3",
	}))
	if !strings.Contains(page, `color="#3366CCB3"`) {
		t.Errorf("the picker does not open on the table's colour:\n%s", page)
	}
	if !strings.Contains(page, "background-color:#3366CCB3") {
		t.Errorf("the swatch is not painted by the template:\n%s", page)
	}
	if got := (RoomGridData{}).PickerColor(); got != room.DefaultGridColor {
		t.Errorf("a table with no colour opens the picker on %q, want %q", got, room.DefaultGridColor)
	}
	if got := (RoomGridData{Color: "#3366CCB3"}).PickerColor(); got != "#3366CCB3" {
		t.Errorf("PickerColor = %q", got)
	}
}
func TestThePickerIsFoldedAwayUntilTheSwatchIsPressed(t *testing.T) {
	page := renderToString(t, RoomGrid(RoomGridData{
		RoomID: testTableRoomID, CellSize: 64, FeetPerCell: 5, Color: "#3366CCB3",
	}))
	if !strings.Contains(page, `data-color-picker hidden`) {
		t.Errorf("the picker is open on load:\n%s", page)
	}
	if !strings.Contains(page, `aria-expanded="false"`) {
		t.Errorf("the swatch does not say it is a disclosure:\n%s", page)
	}
	if !strings.Contains(page, `aria-controls="`+GridColorPickerID+`"`) {
		t.Errorf("the swatch does not name the picker:\n%s", page)
	}
	if !strings.Contains(page, `id="`+GridColorPickerID+`"`) {
		t.Errorf("the picker does not carry the id the swatch names:\n%s", page)
	}
}
func values(choices []Choice) []string {
	out := make([]string, 0, len(choices))
	for _, c := range choices {
		out = append(out, c.Value)
	}
	return out
}
func contains(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}
func TestTheGMsTabletopMenuOpensThreeWindows(t *testing.T) {
	items := menuNamed(t, testRoomPage(room.RoleGM), "Tabletop").Items
	for _, item := range items {
		if item.Label == "Grid & settings" {
			t.Error("the grid and the four table options are still one window")
		}
	}
	for _, want := range []struct {
		label string
		id    string
		url   string
	}{
		{"Layers", "layers", "/fragment/room/layers?room="},
		{"Grid", "grid", "/fragment/room/grid?room="},
		{"Table settings", "settings", "/fragment/room/settings?room="},
	} {
		found := false
		for _, item := range items {
			if item.Label != want.label {
				continue
			}
			found = true
			if item.Window.ID != want.id {
				t.Errorf("%s opens window %q, want %q", want.label, item.Window.ID, want.id)
			}
			if !strings.HasPrefix(item.Window.URL, want.url) {
				t.Errorf("%s fetches %q, want a %q URL", want.label, item.Window.URL, want.url)
			}
			if item.Window.Width <= 0 || item.Window.Height <= 0 {
				t.Errorf("%s opens at no size", want.label)
			}
		}
		if !found {
			t.Errorf("the Tabletop menu has no %q", want.label)
		}
	}
}
func TestAPlayersTabletopMenuIsTheirsAndTouchesNothing(t *testing.T) {
	items := menuNamed(t, testRoomPage(room.RolePlayer), "Tabletop").Items
	want := []string{"Clear blood"}
	if !slices.Equal(labelsOf(items), want) {
		t.Fatalf("a player's Tabletop menu is %v, want %v", labelsOf(items), want)
	}
	for _, item := range items {
		if item.Post != "" || item.Window.ID != "" || item.Modal.URL != "" || item.Href != "" {
			t.Errorf("%s asks the room for something: %+v", item.Label, item)
		}
		if item.Disabled {
			t.Errorf("a player's %s is disabled; these are the lines they can use", item.Label)
		}
		if item.Confirm != "" {
			t.Errorf("%s is confirmed at %q; it destroys nothing that was ever sent", item.Label, item.Confirm)
		}
	}
}
func labelsOf(items []RoomMenuItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Label)
	}
	return out
}
func TestTheLayerNameAsksOnceAndThenOnlyListens(t *testing.T) {
	page := renderToString(t, RoomLayerName(RoomLayerNameData{RoomID: testTableRoomID}))
	if !strings.Contains(page, `hx-trigger="load, room:tabletop from:window"`) {
		t.Errorf("the page render never asks for the name:\n%s", page)
	}
	answer := renderToString(t, RoomLayerName(RoomLayerNameData{RoomID: testTableRoomID, Fetched: true}))
	if strings.Contains(answer, "load") {
		t.Errorf("the answer arms itself again:\n%s", answer)
	}
	if !strings.Contains(answer, `hx-trigger="room:tabletop from:window"`) {
		t.Errorf("the answer stopped listening for the socket:\n%s", answer)
	}
}
func TestThePawnMenuOffersReadingToEverybodyAndTheRestToTheGM(t *testing.T) {
	gm := renderToString(t, roomPawnMenu(testRoomPage(room.RoleGM)))
	for _, want := range []string{"Open details", "Move to floor", "Remove pawn"} {
		if !strings.Contains(gm, want) {
			t.Errorf("the GM's pawn menu has no %q:\n%s", want, gm)
		}
	}
	player := renderToString(t, roomPawnMenu(testRoomPage(room.RolePlayer)))
	if !strings.Contains(player, "Open details") {
		t.Errorf("a player cannot open a pawn from its menu:\n%s", player)
	}
	for _, gone := range []string{"Move to floor", "Remove pawn", "hx-delete", "hx-post", "<template"} {
		if strings.Contains(player, gone) {
			t.Errorf("a player's pawn menu carries %q:\n%s", gone, player)
		}
	}
}
func TestThePawnMenuRemovesThroughTheConfirmModal(t *testing.T) {
	gm := renderToString(t, roomPawnMenu(testRoomPage(room.RoleGM)))
	for _, want := range []string{
		`hx-delete="/rooms/` + testTableRoomID + `/pawns"`,
		"hx-confirm=",
		`data-confirm-label="Remove"`,
		`hx-vals=""`,
	} {
		if !strings.Contains(gm, want) {
			t.Errorf("the removal is missing %s:\n%s", want, gm)
		}
	}
}
func TestThePawnMenuMovesFloorsThroughAHiddenButton(t *testing.T) {
	gm := renderToString(t, roomPawnMenu(testRoomPage(room.RoleGM)))
	if !strings.Contains(gm, `hx-post="/rooms/`+testTableRoomID+`/pawns/layer"`) {
		t.Errorf("the menu does not post to the layer route:\n%s", gm)
	}
	if !strings.Contains(gm, "data-pawn-menu-move hidden") {
		t.Errorf("the move button is not hidden:\n%s", gm)
	}
}
func TestThePawnMenusFloorsAreClonedFromMarkupAndNotBuiltInJS(t *testing.T) {
	gm := renderToString(t, roomPawnMenu(testRoomPage(room.RoleGM)))
	if !strings.Contains(gm, `<ul data-pawn-menu-layers`) {
		t.Errorf("there is no list for the floors to go in:\n%s", gm)
	}
	if !strings.Contains(gm, `<template data-pawn-menu-template>`) {
		t.Errorf("there is no row template to clone:\n%s", gm)
	}
	for _, want := range []string{"data-pawn-menu-layer", "data-pawn-menu-layer-name", "data-pawn-menu-here"} {
		if !strings.Contains(gm, want) {
			t.Errorf("the row template has no %s:\n%s", want, gm)
		}
	}
	if strings.Contains(gm, "data-pawn-menu-layer=") {
		t.Errorf("a floor was rendered into the page:\n%s", gm)
	}
}
func TestThePawnMenuStartsHidden(t *testing.T) {
	gm := renderToString(t, roomPawnMenu(testRoomPage(room.RoleGM)))
	if !strings.Contains(gm, "data-pawn-menu hidden") {
		t.Errorf("the menu is on screen before anybody asked for it:\n%s", gm)
	}
}
func TestThePawnMenusHeadingIsTrimmedRatherThanWidening(t *testing.T) {
	gm := renderToString(t, roomPawnMenu(testRoomPage(room.RoleGM)))
	heading := regexp.MustCompile(`data-pawn-menu-name class="([^"]*)"`).FindStringSubmatch(gm)
	if heading == nil {
		t.Fatalf("the menu has no heading row to trim:\n%s", gm)
	}
	for _, class := range []string{"block", "w-full", "truncate"} {
		if !slices.Contains(strings.Fields(heading[1]), class) {
			t.Errorf("the heading is missing %q, so a long name sets the width of the whole menu: %q", class, heading[1])
		}
	}
}
