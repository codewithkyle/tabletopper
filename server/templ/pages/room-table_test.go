package pages

import (
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

// The three closed sets are the protocol's own values, and a form offering a
// fourth would send something the core refuses with "that is not a snapping
// mode" -- a message about the app rather than about anything the GM did.
func TestTheGridFormOffersExactlyTheProtocolsChoices(t *testing.T) {
	for name, pair := range map[string][2][]string{
		"snap":      {values(GridSnapChoices()), room.Snap("").Values()},
		"diagonals": {values(GridDiagonalChoices()), room.Diagonals("").Values()},
		"monsterHp": {values(GridHPChoices()), room.HPVisibility("").Values()},
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

// A player gets the same four lines, disabled. The bar's shape does not change
// with who is looking -- only the Room menu does that -- and a player who opens
// this menu is told these exist and are not theirs.
func TestAPlayersTabletopMenuOpensNothing(t *testing.T) {
	for _, item := range menuNamed(t, testRoomPage(room.RolePlayer), "Tabletop").Items {
		if !item.Disabled {
			t.Errorf("%q is live for a player", item.Label)
		}
		if item.Window.ID != "" || item.Post != "" {
			t.Errorf("%q does something for a player", item.Label)
		}
	}
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
