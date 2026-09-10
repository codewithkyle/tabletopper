package pages

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"tabletopper/internal/room"
)

const testTableRoomID = "01BX5ZZKBKACTAV9WEVGEMMVT0"

func testLayer(name string, pawns int) RoomLayer {
	return RoomLayer{ID: "01BX5ZZKBKACTAV9WEVGEMMVT1", Name: name, Pawns: pawns}
}

// THE CONFIRM TEXT IS THE ONLY WARNING A GM GETS BEFORE AN ENCOUNTER IS
// DELETED. TableRemoveLayer drops every pawn, fog shape and stroke on the
// layer, and the number in this sentence is the difference between somebody
// pressing Delete and somebody stopping.
func TestTheDeleteWarningCountsWhatItWouldTakeWithIt(t *testing.T) {
	data := RoomLayersData{RoomID: testTableRoomID}

	empty := data.RemovePrompt(testLayer("Cellar", 0))
	if strings.Contains(empty, "pawn") {
		t.Errorf("an empty layer threatens pawns: %q", empty)
	}
	if !strings.Contains(empty, "Cellar") {
		t.Errorf("the warning does not name the layer: %q", empty)
	}

	// One pawn is singular. A warning that reads "the 1 pawns on it" is a
	// warning somebody stops reading.
	if got := data.RemovePrompt(testLayer("Cellar", 1)); !strings.Contains(got, "the 1 pawn on it") {
		t.Errorf("one pawn reads %q", got)
	}

	if got := data.RemovePrompt(testLayer("Cellar", 9)); !strings.Contains(got, "the 9 pawns on it") {
		t.Errorf("nine pawns read %q", got)
	}
}

// The row prints nothing at all for an empty layer rather than "0 pawns",
// which would be drawing attention to the absence of something nobody asked
// about.
func TestAnEmptyLayerSaysNothingAboutPawns(t *testing.T) {
	if got := testLayer("Cellar", 0).PawnLabel(); got != "" {
		t.Errorf("PawnLabel = %q, want empty", got)
	}
	if got := testLayer("Cellar", 2).PawnLabel(); got != "2 pawns" {
		t.Errorf("PawnLabel = %q", got)
	}
}

// A layer whose map has gone is a third state, not a layer with no map: one
// renders "No map" and offers Choose, the other says the map is missing. They
// look identical on a canvas -- an empty floor -- and only one of them is
// something the GM did on purpose.
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

// EVERY MUTATION IN THE MANAGER IS A URL AND THE TEMPLATE BUILDS NONE OF THEM.
// A path assembled in markup is a path nothing can test, and these are the
// eight the window posts to.
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

	// The picker is a modal fragment, so it has to be under /fragment/ or
	// content-modal.js refuses to open it.
	if !strings.HasPrefix(data.ChooseMapPath(l), "/fragment/") {
		t.Errorf("the picker is not a fragment: %q", data.ChooseMapPath(l))
	}
}

// Up is one place further from the bottom and Down is one closer, and the
// index the core takes is counted from the bottom -- so a sign wrong here
// reorders the stack the other way and looks like the buttons are swapped.
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

// THE SIZE WARNING IS THE ONE THING IN THE MANAGER THAT IS ABOUT THE ROOM AND
// NOT ABOUT ONE LAYER. The grid is room-wide, so a floor exported at another
// scale misaligns rather than erroring, and this is where that gets said.
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

// THE GRID FORM IS A FORM WITH FIELDS, so a refusal belongs above the field
// rather than in the alert modal -- which means the panel trio, and a 422 that
// the noSwap list would otherwise swallow whole.
func TestTheGridFormRoutesItsRejectionToItsErrorBlock(t *testing.T) {
	page := renderToString(t, RoomGrid(RoomGridData{RoomID: testTableRoomID, CellSize: 64, FeetPerCell: 5, Color: "#000000FF"}))

	block := "#errors-" + RoomGridPanel
	for _, want := range []string{
		`hx-post="/rooms/` + testTableRoomID + `/grid"`,
		`hx-target="` + block + `"`,
		`hx-swap="outerHTML"`,
		`hx-status:422="target:` + block + `,swap:outerHTML"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the grid form is missing %s:\n%s", want, page)
		}
	}

	// The block the trio points at has to be on the page, or every refusal
	// swaps into nothing and the form looks like it saved.
	if !strings.Contains(page, `id="errors-`+RoomGridPanel+`"`) {
		t.Error("the form has no error block to swap into")
	}
}

// THE GRID FORM MUST NOT LISTEN FOR room:tabletop, and this is a regression
// guard rather than a description: its own save raises that event, so a form
// that refetched on it would replace itself a few milliseconds after every
// change and take the focus of whatever the GM had tabbed into with it. The
// layer manager can afford the refetch because what changes there is
// structural; a form's fields are only ever changed by the person looking.
func TestTheGridFormDoesNotRedrawItselfOnItsOwnSave(t *testing.T) {
	page := renderToString(t, RoomGrid(RoomGridData{RoomID: testTableRoomID, CellSize: 64, FeetPerCell: 5, Color: "#000000FF"}))

	if strings.Contains(page, "room:tabletop") {
		t.Errorf("the grid form refetches on its own save:\n%s", page)
	}

	// The manager does, and for the opposite reason: a layer added in another
	// tab, or a pawn count that moved, has to arrive.
	manager := renderToString(t, RoomLayers(RoomLayersData{RoomID: testTableRoomID}))
	if !strings.Contains(manager, "room:tabletop from:window") {
		t.Errorf("the layer manager does not refetch:\n%s", manager)
	}
}

// The four closed sets are the protocol's own values, and a form offering a
// fifth would send something the core refuses with "that is not a snapping
// mode" -- a message about the app rather than about anything the GM did.
func TestTheGridFormOffersExactlyTheProtocolsChoices(t *testing.T) {
	for name, pair := range map[string][2][]string{
		"gridLines":          {values(GridLineChoices()), room.GridLines("").Values()},
		"snap":               {values(GridSnapChoices()), room.Snap("").Values()},
		"diagonals":          {values(GridDiagonalChoices()), room.Diagonals("").Values()},
		"pawnLabels":         {values(PawnLabelChoices()), room.PawnLabels("").Values()},
		"initiativeGrouping": {values(InitiativeGroupingChoices()), room.InitiativeGrouping("").Values()},
	} {
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

// THE FORM HAS TO OPEN ON THE STYLE THE TABLE IS ACTUALLY ON. Nothing fails if
// it does not: the window shows solid, the GM changes something else, and the
// change carries a line style they never chose back to the table. So the one
// thing worth pinning is that the field reaches the radio.
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

// THE PICKER IS NOT A FIELD AND MUST NEVER BECOME ONE. It is a custom element
// bundled into room.js; the server has never heard of it, and everything the
// form posts still comes out of the text box beside it. A browser running no
// scripts gets an unupgraded tag it ignores and a colour it can still edit by
// hand.
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

	// The native input is what could not do alpha, which is the whole reason
	// the picker is here. One left behind would post a second colour.
	if strings.Contains(page, `type="color"`) {
		t.Errorf("the native colour input is still in the form:\n%s", page)
	}
	if strings.Contains(page, `name="color"`) && strings.Count(page, `name="color"`) != 1 {
		t.Errorf("%d colour fields, want 1:\n%s", strings.Count(page, `name="color"`), page)
	}
}

// BOTH HALVES OF THE CONTROL OPEN ON THE COLOUR THE TABLE IS ON. The picker
// reads its own attribute and the chip is painted by the template, because the
// panel is a fragment htmx swaps in with no script of its own to run -- a chip
// left to the client would be blank until the GM touched something.
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

	// An empty colour would reach the picker as a colour made of NaN, so the
	// default the core would have written stands in for it.
	if got := (RoomGridData{}).PickerColor(); got != room.DefaultGridColor {
		t.Errorf("a table with no colour opens the picker on %q, want %q", got, room.DefaultGridColor)
	}
	if got := (RoomGridData{Color: "#3366CCB3"}).PickerColor(); got != "#3366CCB3" {
		t.Errorf("PickerColor = %q", got)
	}
}

// THE PICKER IS FOLDED AWAY AND THE SWATCH IS WHAT UNFOLDS IT. This window is
// 300 by 420 and already scrolls; 176 pixels of permanent picker would push
// half the settings below the fold for a setting a GM changes once a campaign.
// The `hidden` is what color.ts toggles, and it is the only way it can -- a
// class name written in server/js is never emitted by Tailwind.
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

	// The button names the picker, so the two ids have to agree. They come
	// from the same constant, and this is the guard on somebody hard-coding
	// one of them later.
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

// THE TWO CONFIGURATION DIALOGS ARE WINDOWS AND NOT MODALS, decided after the
// first pass built them as modals. A window holds a fragment URL and no markup,
// so what this pins is the URL and the id -- the id being the key the GM's
// layout is remembered under, which must not become the URL or a GM's window
// positions would be per room.
func TestTheGMsTabletopMenuOpensTwoWindows(t *testing.T) {
	items := menuNamed(t, testRoomPage(room.RoleGM), "Tabletop").Items

	for _, want := range []struct {
		label string
		id    string
		url   string
	}{
		{"Layers", "layers", "/fragment/room/layers?room="},
		{"Grid & settings", "grid", "/fragment/room/grid?room="},
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

// A PLAYER'S TABLETOP MENU IS THE LINES THAT ASK THE ROOM FOR NOTHING. It used
// to be five greyed lines saying "these exist and are not yours", which is a
// wall rather than information -- there is no version of this app where a player
// opens Grid & settings. What is left is what was always theirs: their own view
// of their own floor.
//
// THE RULE RATHER THAN THE LIST IS WHAT THIS PINS. What is in here changes
// something no other person at the table can see -- the blood is drawn by this
// browser out of hit points it watched change -- so a second line under this
// heading is fine if and only if it is the same kind of thing. One that posted,
// opened a window, or wanted confirming would be a command to the room wearing a
// preference's clothes.
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

// labelsOf is a menu's lines, for a failure message.
func labelsOf(items []RoomMenuItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Label)
	}

	return out
}

// THE ANSWER MUST NOT RE-ARM THE TRIGGER THAT ASKED FOR IT, and this is a
// regression guard on a bug that shipped: the span asks for itself on load and
// replaces itself with the reply, so a reply carrying "load" fires the moment
// htmx processes it and the browser spends the rest of the session fetching
// this one string -- a request per round trip, the loading bar up for good, and
// the whole page under a wait cursor, because html[state="loading"] * sets one.
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

// THE MENU IS THREE ITEMS AND TWO OF THEM ARE THE GM'S. A player right-clicking
// a monster is asking to read it, which is the item everybody gets; moving a
// pawn between floors and taking it off the table are refused server-side for
// anybody else, so rendering them for a player would be offering a 403.
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

// THE REMOVAL IS AN htmx BUTTON SO THAT IT GOES THROUGH THE CONFIRM MODAL.
// hx-confirm is read off the element making the request, so a DELETE built in
// the client would skip the dialog -- and window.confirm is banned. The
// attribute here is only the fallback wording: pawn-menu.ts rewrites it with
// the pawn's name each time the menu opens, because this removes the one under
// the pointer rather than the selection.
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

// THE FLOOR MOVE IS A BUTTON NOBODY SEES, for the reason the Delete key's is:
// the rows in the submenu are cloned per layer and a cloned element has never
// been through htmx, so it cannot carry a route. It carries a floor id, and
// pressing it writes that id onto the one button that HAS been processed.
func TestThePawnMenuMovesFloorsThroughAHiddenButton(t *testing.T) {
	gm := renderToString(t, roomPawnMenu(testRoomPage(room.RoleGM)))

	if !strings.Contains(gm, `hx-post="/rooms/`+testTableRoomID+`/pawns/layer"`) {
		t.Errorf("the menu does not post to the layer route:\n%s", gm)
	}
	if !strings.Contains(gm, "data-pawn-menu-move hidden") {
		t.Errorf("the move button is not hidden:\n%s", gm)
	}
}

// THE FLOORS ARE A <template> AND THE LIST SHIPS EMPTY, which is the whole
// reason this component exists rather than a select: server/js is not a
// Tailwind source, so a class named in pawn-menu.ts would never be emitted. The
// row is styled here and cloned there, and the client writes nothing into it
// but text, a data attribute and [hidden].
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

	// The list is filled when the menu opens, so no floor may be named in the
	// page render: the marker attribute is bare here and carries an id only
	// once a row has been cloned.
	if strings.Contains(gm, "data-pawn-menu-layer=") {
		t.Errorf("a floor was rendered into the page:\n%s", gm)
	}
}

// IT ARRIVES CLOSED. The menu is one element reused for every pawn, so the
// render is the state before anybody has asked it anything -- and [hidden] is
// what pawn-menu.ts toggles, because it cannot write a class name.
func TestThePawnMenuStartsHidden(t *testing.T) {
	gm := renderToString(t, roomPawnMenu(testRoomPage(room.RoleGM)))

	if !strings.Contains(gm, "data-pawn-menu hidden") {
		t.Errorf("the menu is on screen before anybody asked for it:\n%s", gm)
	}
}

// A LONG NAME IS TRIMMED RATHER THAN ALLOWED TO SET THE MENU'S WIDTH. DaisyUI
// makes every row a flex item of a wrapping column, and a row with no width of
// its own is as wide as its content -- so a name nobody shortened widens the
// heading row, stretches every row below it to match, and puts the hover
// backgrounds, the floor list and the Here badge out past the border. The three
// classes below are the whole fix and none of them is decoration.
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
