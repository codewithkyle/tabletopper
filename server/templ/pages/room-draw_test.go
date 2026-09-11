package pages
import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"tabletopper/internal/room"
)
func TestEverybodyGetsTheDrawTool(t *testing.T) {
	for _, role := range []room.Role{room.RoleGM, room.RolePlayer} {
		page := markup(t, Room(testRoomPage(role)))
		if !strings.Contains(page, `data-room-tool="`+RoomToolDraw+`"`) {
			t.Errorf("the %s's pill has no draw tool", role)
		}
		if !strings.Contains(page, "data-room-tool-draws") {
			t.Errorf("the %s's draw tool does not say what it does", role)
		}
	}
}
func TestExactlyOneToolIsTheDrawTool(t *testing.T) {
	draws := 0
	for _, tool := range RoomTools() {
		if tool.Draws {
			draws++
		}
	}
	if draws != 1 {
		t.Errorf("%d tools carry Draws, want exactly 1", draws)
	}
	page := markup(t, Room(testRoomPage(room.RolePlayer)))
	if got := strings.Count(page, "data-room-tool-draws"); got != 1 {
		t.Errorf("the markup carries data-room-tool-draws %d times, want 1", got)
	}
}
func TestTheDrawToolHasItsOwnLetter(t *testing.T) {
	keys := map[string]string{}
	for _, tool := range RoomTools() {
		if tool.Key == "" {
			continue
		}
		if other, taken := keys[tool.Key]; taken {
			t.Errorf("%q and %q both answer to %q", other, tool.Name, tool.Key)
		}
		keys[tool.Key] = tool.Name
	}
	if keys["d"] != RoomToolDraw {
		t.Errorf("the letter d belongs to %q, want %q", keys["d"], RoomToolDraw)
	}
}
func TestClearDrawingIsAConfirmedLayeredGMItem(t *testing.T) {
	data := testRoomPage(room.RoleGM)
	var found bool
	for _, item := range menuNamed(t, data, "Tabletop").Items {
		if item.Label != "Clear drawing" {
			continue
		}
		found = true
		if item.Post == "" {
			t.Error("Clear drawing posts nowhere")
		}
		if !item.Layered {
			t.Error("Clear drawing is not layered, so it would act on the active floor")
		}
		if item.Confirm == "" {
			t.Error("Clear drawing throws away every line on the floor with no confirmation")
		}
	}
	if !found {
		t.Fatalf("the GM's Tabletop menu has no Clear drawing: %v", itemLabels(t, data, "Tabletop"))
	}
	player := markup(t, Room(testRoomPage(room.RolePlayer)))
	if strings.Contains(player, "Clear drawing") {
		t.Error("a player's Tabletop menu offers Clear drawing")
	}
}
func TestClearDrawingIsNotTheDangerousItem(t *testing.T) {
	items := testRoomPage(room.RoleGM).tabletopMenu().Items
	last := items[len(items)-1]
	if last.Label != "Clear tabletop" || !last.Danger {
		t.Errorf("the last Tabletop item is %q (danger=%v), want Clear tabletop", last.Label, last.Danger)
	}
	for _, item := range items {
		if item.Label == "Clear drawing" && item.Danger {
			t.Error("Clear drawing is marked as the menu's destructive item")
		}
	}
}
func TestTheDrawOptionsPillStartsOnThePenForEveryRole(t *testing.T) {
	for _, role := range []room.Role{room.RoleGM, room.RolePlayer} {
		page := markup(t, Room(testRoomPage(role)))
		options := regexp.MustCompile(`data-draw-options[^>]*>`).FindString(page)
		if options == "" {
			t.Fatalf("the %s has no draw options pill", role)
		}
		if !strings.Contains(options, "hidden") {
			t.Errorf("the %s's options pill is not hidden while another tool is chosen", role)
		}
		pressed := regexp.MustCompile(`data-draw-mode="([a-z]+)"[^>]*aria-pressed="true"`).FindAllStringSubmatch(page, -1)
		if len(pressed) != 1 {
			t.Fatalf("%d draw modes are pressed for the %s, want exactly 1", len(pressed), role)
		}
		if pressed[0][1] != "pen" {
			t.Errorf("the pill opens on %q, want \"pen\"", pressed[0][1])
		}
	}
}
func TestTheDrawPanelsStartFoldedAndAreLabelled(t *testing.T) {
	page := markup(t, Room(testRoomPage(room.RoleGM)))
	for _, panel := range []struct{ name, id string }{
		{"color", DrawColorPanelID},
		{"width", DrawWidthPanelID},
	} {
		markup := regexp.MustCompile(`data-draw-popout="` + panel.name + `"[^>]*>`).FindString(page)
		if markup == "" {
			t.Fatalf("there is no %s panel", panel.name)
		}
		if !strings.Contains(markup, "hidden") {
			t.Errorf("the %s panel does not start folded: %s", panel.name, markup)
		}
		button := regexp.MustCompile(`data-draw-open="` + panel.name + `"[^>]*>`).FindString(page)
		if button == "" {
			t.Fatalf("there is no button for the %s panel", panel.name)
		}
		if !strings.Contains(button, `aria-expanded="false"`) {
			t.Errorf("the %s button does not start collapsed: %s", panel.name, button)
		}
		if !strings.Contains(button, `aria-controls="`+panel.id+`"`) {
			t.Errorf("the %s button does not say which panel it opens: %s", panel.name, button)
		}
		if !strings.Contains(page, `id="`+panel.id+`"`) {
			t.Errorf("nothing on the page has the id %q the button points at", panel.id)
		}
	}
}
func TestTheDrawPanelsOpenBesideThePill(t *testing.T) {
	page := markup(t, Room(testRoomPage(room.RoleGM)))
	for _, name := range []string{"color", "width"} {
		panel := regexp.MustCompile(`<div[^>]*data-draw-popout="` + name + `"[^>]*>`).FindString(page)
		for _, want := range []string{"absolute", "right-full", "top-1/2", "-translate-y-1/2"} {
			if !strings.Contains(panel, want) {
				t.Errorf("the %s panel has no %s, so it would not sit beside the pill: %s", name, want, panel)
			}
		}
	}
}
func TestTheBrushSliderStopsAtTheProtocolsWidth(t *testing.T) {
	page := markup(t, Room(testRoomPage(room.RoleGM)))
	slider := regexp.MustCompile(`<input[^>]*data-draw-width[^>]*>`).FindString(page)
	if slider == "" {
		t.Fatal("there is no brush size slider")
	}
	if !strings.Contains(slider, `max="`+strconv.Itoa(room.StrokeWidthMax)+`"`) {
		t.Errorf("the slider does not stop at StrokeWidthMax: %s", slider)
	}
	if !strings.Contains(slider, `min="1"`) {
		t.Errorf("the slider goes below one map pixel: %s", slider)
	}
	if !strings.Contains(slider, `value="`+DrawWidthDefault+`"`) {
		t.Errorf("the slider renders no starting width: %s", slider)
	}
	if !strings.Contains(page, `data-draw-width-value`) {
		t.Error("the slider has no number beside it")
	}
}
func TestTheDrawPickerHasNoAlpha(t *testing.T) {
	page := markup(t, Room(testRoomPage(room.RoleGM)))
	if !strings.Contains(page, "<hex-color-picker") {
		t.Error("the drawing pill has no colour picker")
	}
	panel := regexp.MustCompile(`data-draw-popout="color"[\s\S]*?</div>`).FindString(page)
	if strings.Contains(panel, "hex-alpha-color-picker") {
		t.Error("the drawing pill uses the alpha picker")
	}
}
func TestNeitherColorPickerIsGivenADisplayUtility(t *testing.T) {
	breaking := []string{
		"block", "inline", "inline-block", "inline-flex",
		"grid", "inline-grid", "contents", "flow-root", "table", "list-item",
	}
	rendered := map[string]string{
		"room": markup(t, Room(testRoomPage(room.RoleGM))),
		"grid": markup(t, RoomGrid(RoomGridData{})),
	}
	seen := 0
	for where, page := range rendered {
		for _, picker := range regexp.MustCompile(`<hex(-alpha)?-color-picker[^>]*>`).FindAllString(page, -1) {
			seen++
			class := regexp.MustCompile(`class="([^"]*)"`).FindStringSubmatch(picker)
			if class == nil {
				continue
			}
			for _, word := range strings.Fields(class[1]) {
				if slices.Contains(breaking, word) {
					t.Errorf("the %s picker carries %q, which overrides its own :host display: %s", where, word, picker)
				}
			}
		}
	}
	if seen != 2 {
		t.Fatalf("found %d colour pickers across the two pages, want 2", seen)
	}
}
