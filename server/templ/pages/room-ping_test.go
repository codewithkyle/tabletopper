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

// AND THERE IS NO MUTE IN THE ROOM. One was built here first -- a Tabletop menu
// row toggling a localStorage flag -- and it was removed when the volume became
// an account setting: a slider whose bottom stop is silence already answers "at
// all" as well as "how loud", and two controls over one setting are two things
// that can disagree about it. See PingVolume in internal/prefs and the Sounds
// fieldset in account.templ.
func TestTheRoomOffersNoSecondPingControl(t *testing.T) {
	for _, role := range []room.Role{room.RoleGM, room.RolePlayer} {
		for _, menu := range testRoomPage(role).Menus() {
			for _, item := range menu.Items {
				if strings.Contains(strings.ToLower(item.Label), "ping") {
					t.Errorf("the %s's %s menu offers %q", role, menu.Label, item.Label)
				}
			}
		}
	}
}

// THE TABLETOP READS THE SETTING OFF THE PAGE, and it is a VALUE rather than a
// bare attribute like its two neighbours -- so an absent one has to mean full
// volume. A page from a build that did not render it must not be a page where
// pings went quiet.
func TestTheRoomPageCarriesThePingVolume(t *testing.T) {
	data := testRoomPage(room.RolePlayer)
	data.PingVolume = 40

	if got := data.PingVolumeAttr(); got != "40" {
		t.Errorf("the page renders the volume as %q, want %q", got, "40")
	}
	if !strings.Contains(markup(t, Room(data)), `data-ping-volume="40"`) {
		t.Error("the tabletop is not told how loud a ping is")
	}

	// CLAMPED ON THE WAY OUT, because the column is a byte and this is the read
	// path: a stored 200 should render a room at full volume rather than a
	// slider position nothing in the client would believe.
	data.PingVolume = 400
	if got := data.PingVolumeAttr(); got != "100" {
		t.Errorf("a stored 400 renders as %q, want %q", got, "100")
	}
}

// THE SETTINGS DIALOG OPENS OVER THE TABLE, from Settings in the Help menu, so
// the room has to carry the script that makes its volume slider read out. It is
// in the base layout rather than on the two pages that can open the dialog,
// because a reading that stopped moving is a failure nobody would report -- the
// number is still there and still correct until the thumb is touched.
func TestTheRoomCarriesTheScriptTheVolumeSliderNeeds(t *testing.T) {
	page := markup(t, Room(testRoomPage(room.RolePlayer)))

	if !strings.Contains(page, "/js/range-output.js") {
		t.Error("a settings dialog opened over the table would have a dead volume reading")
	}
}
