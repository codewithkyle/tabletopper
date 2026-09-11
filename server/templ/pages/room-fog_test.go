package pages

import (
	"regexp"
	"strings"
	"testing"

	"tabletopper/internal/room"
)

func TestOnlyTheGMGetsTheFogTool(t *testing.T) {
	gm := markup(t, Room(testRoomPage(room.RoleGM)))
	if !strings.Contains(gm, `data-room-tool="`+RoomToolFog+`"`) {
		t.Error("the GM's pill has no fog tool")
	}
	player := markup(t, Room(testRoomPage(room.RolePlayer)))
	if strings.Contains(player, `data-room-tool="`+RoomToolFog+`"`) {
		t.Error("a player's pill has a fog tool on it")
	}
	if strings.Contains(player, "data-fog-shape") {
		t.Error("a player's page carries the fog options pill")
	}
}
func TestExactlyOneToolIsTheFogTool(t *testing.T) {
	fogs := 0
	for _, tool := range RoomTools() {
		if tool.Fogs {
			fogs++
		}
	}
	if fogs != 1 {
		t.Errorf("%d tools carry Fogs, want exactly 1", fogs)
	}
	page := markup(t, Room(testRoomPage(room.RoleGM)))
	if got := strings.Count(page, "data-room-tool-fogs"); got != 1 {
		t.Errorf("the markup carries data-room-tool-fogs %d times, want 1", got)
	}
}
func TestTheFogOptionsPillStartsOnOneOfEach(t *testing.T) {
	page := markup(t, Room(testRoomPage(room.RoleGM)))
	options := regexp.MustCompile(`data-fog-options[^>]*>`).FindString(page)
	if options == "" {
		t.Fatal("there is no fog options pill")
	}
	if !strings.Contains(options, "hidden") {
		t.Error("the options pill is not hidden while another tool is chosen")
	}
	for attr, want := range map[string]string{"data-fog-shape": "rect", "data-fog-mode": "reveal"} {
		pressed := regexp.MustCompile(attr+`="([a-z]+)"[^>]*aria-pressed="true"`).FindAllStringSubmatch(page, -1)
		if len(pressed) != 1 {
			t.Fatalf("%d %s buttons are pressed, want exactly 1", len(pressed), attr)
		}
		if pressed[0][1] != want {
			t.Errorf("%s opens on %q, want %q", attr, pressed[0][1], want)
		}
	}
}
func TestTheFogMenuActsOnTheViewedFloorBehindAConfirm(t *testing.T) {
	data := testRoomPage(room.RoleGM)
	items := menuNamed(t, data, "Fog").Items
	if len(items) != 2 {
		t.Fatalf("the Fog menu has %d items, want 2: %v", len(items), itemLabels(t, data, "Fog"))
	}
	for _, item := range items {
		if item.Disabled {
			t.Errorf("%q is still disabled", item.Label)
		}
		if item.Post == "" {
			t.Errorf("%q posts nowhere", item.Label)
		}
		if !item.Layered {
			t.Errorf("%q is not layered, so it would act on the active floor", item.Label)
		}
		if item.Confirm == "" {
			t.Errorf("%q throws away every shape on the floor with no confirmation", item.Label)
		}
	}
	page := markup(t, Room(data))
	want := 0
	for _, menu := range data.Menus() {
		for _, item := range menu.Items {
			if item.Layered {
				want++
			}
		}
	}
	if got := strings.Count(page, "data-room-layered"); got != want {
		t.Errorf("the markup carries data-room-layered %d times; %d items are layered", got, want)
	}
	if !strings.Contains(page, `hx-vals="`+emptyVals+`"`) {
		t.Error("a layered item's hx-vals is not valid JSON, so htmx would throw before the request")
	}
}
func TestAPlayerHasNoFogMenu(t *testing.T) {
	for _, label := range menuLabels(testRoomPage(room.RolePlayer)) {
		if label == "Fog" {
			t.Error("a player's bar has a Fog menu")
		}
	}
}
func TestTheGridWindowCarriesThePrefillSwitch(t *testing.T) {
	page := renderToString(t, RoomGrid(RoomGridData{
		RoomID: testTableRoomID, CellSize: 64, FeetPerCell: 5, Color: "#000000FF",
		FogPrefill: true,
	}))
	field := regexp.MustCompile(`<input[^>]*name="fogPrefill"[^>]*>`).FindString(page)
	if field == "" {
		t.Fatalf("the window has no prefill switch:\n%s", page)
	}
	if !strings.Contains(field, "checked") {
		t.Error("the switch does not read the room's own value")
	}
	off := renderToString(t, RoomGrid(RoomGridData{
		RoomID: testTableRoomID, CellSize: 64, FeetPerCell: 5, Color: "#000000FF",
	}))
	if strings.Contains(regexp.MustCompile(`<input[^>]*name="fogPrefill"[^>]*>`).FindString(off), "checked") {
		t.Error("the switch is checked for a room that has it off")
	}
}
