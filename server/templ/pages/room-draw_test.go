package pages

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"tabletopper/internal/room"
)

// THE PEN IS EVERYBODY'S, unlike the fog. Whether a player may actually draw is
// Table.PlayersCanDraw, which the core reads and refuses against with the alert
// modal -- so a player at a table where drawing is off finds the button and is
// told why. A button that appeared and disappeared as the GM changed their mind
// would be a control moving underneath a hand that was using it, and the pill
// has no room to explain a grey circle.
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

// EXACTLY ONE TOOL LAYS DOWN INK, for the reason exactly one is the ruler and
// exactly one is the fog: tools.ts finds the mode by the attribute rather than
// by a name written out in both languages, and two of them would make the
// answer whichever came first in the markup.
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

// A MODE WITH NO LETTER IS A MODE NOBODY REACHES FOR, and the pen is reached
// for constantly. The letter is rendered onto the button rather than spelled in
// TypeScript, so this is what says it is there at all.
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

// CLEAR DRAWING IS THE GM'S AND IS LAYERED. The second of those is the one
// worth a test: without data-room-layered the room bundle never finds the item,
// the hx-vals stays "{}", and the route quietly falls back to the active floor
// -- which is the right floor most of the time and the wrong one exactly when a
// GM is tidying up the floor they are looking at.
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

// THE ONE DESTRUCTIVE ITEM IN A MENU IS DRAWN IN THE ERROR COLOUR AND SITS
// LAST, and in the Tabletop menu that item is Clear tabletop. Clear drawing
// empties one floor's lines and is recoverable by drawing them again; the one
// below it empties every floor of everything.
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

// THE OPTIONS PILL STARTS ON THE PEN AND STARTS HIDDEN, and it is every role's
// -- unlike the fog's, which a player's page does not render at all.
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

// BOTH FOLDED CONTROLS START FOLDED AND SAY WHAT THEY OPEN. A panel that
// rendered open would be a hundred and seventy-six pixels of picker over the
// corner of the map on every page load.
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

// THE PANELS OPEN TO THE LEFT OF THE PILL, AND WHERE THEY OPEN IS THE TEMPLATE'S
// TO SAY. draw-tool.ts measures nothing and writes no position -- it only
// toggles [hidden] -- so if these classes go, the panel appears on top of the
// pill and nothing in TypeScript would notice.
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

// THE SLIDER STOPS WHERE THE SERVER DOES. A stroke wider than StrokeWidthMax is
// refused by the core, so a slider that went past it would be a control whose
// top end raises an alert modal.
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

	// AND IT RENDERS THE WIDTH THE PEN OPENS ON, because draw-tool.ts READS
	// this value rather than writing one -- so the slider and the pen cannot
	// open on two different numbers. A slider with no value would leave the pen
	// on its own no-markup fallback.
	if !strings.Contains(slider, `value="`+DrawWidthDefault+`"`) {
		t.Errorf("the slider renders no starting width: %s", slider)
	}
	if !strings.Contains(page, `data-draw-width-value`) {
		t.Error("the slider has no number beside it")
	}
}

// THE COLOUR PICKER HAS NO ALPHA, which is the whole reason it is a different
// element from the grid's. Two overlapping segments of a translucent line blend
// twice and show a darker dot at every joint; see render/stroke-pass.ts.
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

// A COLOUR PICKER MUST NOT BE TOLD HOW TO LAY ITSELF OUT, and this is the test
// for a bug that shipped once and failed silently.
//
// vanilla-colorful sizes its two halves with `:host{display:flex;
// flex-direction:column}` inside its shadow root, and a class on the host from
// OUTSIDE that root beats a :host rule whatever the specificity. So a `block`
// on the element turns the column off, `flex-grow` on the saturation square
// stops meaning anything, and the picker collapses to a thin hue strip with its
// two pointers floating on it. Nothing errors, no test that only looked for the
// element would notice, and the CSS selector diff is clean because `block` was
// already in the build.
//
// Sizing it is fine and is how both pickers get their height. Telling it its
// display is not.
func TestNeitherColorPickerIsGivenADisplayUtility(t *testing.T) {
	// Every Tailwind utility that would replace the flex column. `flex` itself
	// is what the element already is, so it is not in the list.
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

	// Both pages render one, so a regex that stopped matching would otherwise
	// leave this test passing while it checked nothing.
	if seen != 2 {
		t.Fatalf("found %d colour pickers across the two pages, want 2", seen)
	}
}
