package pages

import (
	"strings"
	"testing"

	"tabletopper/internal/room"
)

// POINTING IS REFUSED TO NOBODY. Ping.Authorize in internal/room is
// requirePlayerLayer alone -- no role test, no room setting -- because pointing
// is how a player says "that door" without being able to move anything. So
// unlike the fog, and even unlike the pen, there is nothing about this tool that
// can be turned off for anybody, and the pill renders it for every role.
func TestEverybodyGetsThePingTool(t *testing.T) {
	for _, role := range []room.Role{room.RoleGM, room.RolePlayer} {
		page := markup(t, Room(testRoomPage(role)))
		if !strings.Contains(page, `data-room-tool="`+RoomToolPing+`"`) {
			t.Errorf("the %s's pill has no ping tool", role)
		}
		if !strings.Contains(page, "data-room-tool-pings") {
			t.Errorf("the %s's ping tool does not say what it does", role)
		}
	}
}

// EXACTLY ONE TOOL POINTS, for the reason exactly one is the ruler and exactly
// one lays down ink: tools.ts finds the mode by the attribute rather than by a
// name written out in both languages, and two of them would make the answer
// whichever came first in the markup.
func TestExactlyOneToolIsThePingTool(t *testing.T) {
	pings := 0
	for _, tool := range RoomTools() {
		if tool.Pings {
			pings++
		}
	}

	if pings != 1 {
		t.Errorf("%d tools carry Pings, want exactly 1", pings)
	}

	page := markup(t, Room(testRoomPage(room.RolePlayer)))
	if got := strings.Count(page, "data-room-tool-pings"); got != 1 {
		t.Errorf("the markup carries data-room-tool-pings %d times, want 1", got)
	}
}

// P FOR PING. The letters are checked against each other in
// TestTheDrawToolHasItsOwnLetter, which walks the whole list; this is the one
// pairing that test cannot assert without knowing about this one.
func TestThePingToolAnswersToP(t *testing.T) {
	for _, tool := range RoomTools() {
		if tool.Name != RoomToolPing {
			continue
		}
		if tool.Key != "p" {
			t.Errorf("the ping tool answers to %q, want %q", tool.Key, "p")
		}

		return
	}

	t.Fatal("there is no ping tool")
}

// A PILL OF SIX BUTTONS IS SIX GLYPHS AND THEY HAVE TO BE SIX DIFFERENT ONES.
//
// THE FAILURE THIS CATCHES IS SILENT AND IT ALMOST HAPPENED. roomToolIcon is a
// switch with a DEFAULT, which is right -- Select's cursor is what a tool with
// nothing better to draw should show -- and it means a tool whose case was
// forgotten renders perfectly, in the wrong picture, as a second Select button
// in the middle of the pill. Nothing else in the build would say so.
func TestEveryToolHasAnIconOfItsOwn(t *testing.T) {
	seen := map[string]string{}

	for _, tool := range RoomTools() {
		icon := markup(t, roomToolIcon(tool.Name))
		if icon == "" {
			t.Errorf("the %s tool renders no icon", tool.Name)

			continue
		}
		if other, taken := seen[icon]; taken {
			t.Errorf("the %s and %s tools draw the same icon", other, tool.Name)
		}
		seen[icon] = tool.Name
	}
}

// MUTING IS EVERY VIEWER'S, like Clear blood and unlike everything else under
// Tabletop. Nothing about it is sent, stored on the server or visible to
// anybody else at the table, so there is no role that should be denied it.
func TestEverybodyGetsTheMutePingsRow(t *testing.T) {
	for _, role := range []room.Role{room.RoleGM, room.RolePlayer} {
		var found bool
		for _, item := range menuNamed(t, testRoomPage(role), "Tabletop").Items {
			if item.ID != roomPingSoundID {
				continue
			}
			found = true

			if item.Label == "" || item.AltLabel == "" {
				t.Errorf("the %s's mute row reads %q / %q, want both", role, item.Label, item.AltLabel)
			}

			// IT CARRIES NO ACTION, and that is the design rather than an
			// omission: public/js/room.js finds a menu item by
			// data-room-action, and everything about this row -- the
			// preference, the wording and the noise -- lives in one bundle.
			// See roomPingSoundID.
			if item.Action != "" {
				t.Errorf("the mute row carries the action %q; it belongs to the room bundle alone", item.Action)
			}
		}

		if !found {
			t.Errorf("the %s's Tabletop menu has no mute row", role)
		}
	}
}

// AND THE MARKUP IS A CONTRACT WITH ping-sound.ts, which finds the row by id and
// shows one of its two readings with [hidden]. A third span in there, or the two
// swapped, and the menu would offer to unmute a sound that is playing.
func TestTheMutePingsRowRendersBothReadingsWithTheSecondHidden(t *testing.T) {
	page := markup(t, Room(testRoomPage(room.RolePlayer)))

	open := strings.Index(page, `id="`+roomPingSoundID+`"`)
	if open < 0 {
		t.Fatalf("the page renders no #%s", roomPingSoundID)
	}
	end := strings.Index(page[open:], "</li>")
	if end < 0 {
		t.Fatal("the mute row never closes")
	}
	row := page[open : open+end]

	if got := strings.Count(row, "<span"); got != 2 {
		t.Fatalf("the mute row holds %d spans, want 2:\n%s", got, row)
	}
	if strings.Contains(row, "data-room-action") {
		t.Errorf("the mute row carries data-room-action:\n%s", row)
	}

	first, second := strings.Index(row, "<span"), strings.LastIndex(row, "<span")
	if strings.Contains(row[first:second], "hidden") {
		t.Errorf("the mute row opens on its hidden reading:\n%s", row)
	}
	if !strings.Contains(row[second:], "hidden") {
		t.Errorf("the mute row shows both readings at once:\n%s", row)
	}
}
