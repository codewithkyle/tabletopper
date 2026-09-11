package pages
import (
	"strings"
	"testing"
	"tabletopper/internal/room"
)
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
func TestTheRoomPageCarriesThePingVolume(t *testing.T) {
	data := testRoomPage(room.RolePlayer)
	data.PingVolume = 40
	if got := data.PingVolumeAttr(); got != "40" {
		t.Errorf("the page renders the volume as %q, want %q", got, "40")
	}
	if !strings.Contains(markup(t, Room(data)), `data-ping-volume="40"`) {
		t.Error("the tabletop is not told how loud a ping is")
	}
	data.PingVolume = 400
	if got := data.PingVolumeAttr(); got != "100" {
		t.Errorf("a stored 400 renders as %q, want %q", got, "100")
	}
}
func TestTheRoomCarriesTheScriptTheVolumeSliderNeeds(t *testing.T) {
	page := markup(t, Room(testRoomPage(room.RolePlayer)))
	if !strings.Contains(page, "/js/range-output.js") {
		t.Error("a settings dialog opened over the table would have a dead volume reading")
	}
}
