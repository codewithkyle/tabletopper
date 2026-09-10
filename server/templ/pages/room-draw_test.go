package pages

import (
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
