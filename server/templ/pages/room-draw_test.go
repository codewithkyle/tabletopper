package pages

import (
	"regexp"
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
