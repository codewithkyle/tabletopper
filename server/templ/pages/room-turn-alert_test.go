package pages

import (
	"strings"
	"testing"

	"tabletopper/internal/room"
)

func turnAlertBanner(t *testing.T, data RoomPageData) string {
	t.Helper()
	page := markup(t, Room(data))
	at := strings.Index(page, "<div data-on-deck ")
	if at < 0 {
		t.Fatalf("the room page carries no on-deck banner:\n%s", page)
	}
	rest := page[at:]
	return rest[:strings.Index(rest, "</div>")]
}
func tabletopTag(t *testing.T, data RoomPageData) string {
	t.Helper()
	page := markup(t, Room(data))
	at := strings.Index(page, `id="tabletop"`)
	if at < 0 {
		t.Fatal("the room page has no tabletop")
	}
	rest := page[at:]
	return rest[:strings.Index(rest, ">")]
}
func TestBothSeatsGetTheOnDeckBanner(t *testing.T) {
	for _, role := range []room.Role{room.RoleGM, room.RolePlayer} {
		banner := turnAlertBanner(t, testRoomPage(role))
		if !strings.Contains(banner, "data-on-deck-line") {
			t.Errorf("the %s's banner has nowhere to say who is on deck:\n%s", role, banner)
		}
		if !strings.Contains(banner, "You're on deck") {
			t.Errorf("the %s's banner does not say what it is:\n%s", role, banner)
		}
	}
}
func TestTheBannerStartsHiddenAndLetsTheTableThrough(t *testing.T) {
	banner := turnAlertBanner(t, testRoomPage(room.RolePlayer))
	if !strings.Contains(banner, "data-on-deck hidden") {
		t.Errorf("the banner is on screen before anybody is on deck:\n%s", banner)
	}
	if !strings.Contains(banner, "pointer-events-none") {
		t.Errorf("the banner swallows clicks meant for the tabletop:\n%s", banner)
	}
	if !strings.Contains(banner, "data-on-deck-close") {
		t.Errorf("the banner cannot be dismissed:\n%s", banner)
	}
	if !strings.Contains(banner, `aria-label="Dismiss"`) {
		t.Errorf("the dismiss control is an unlabelled icon:\n%s", banner)
	}
}
func TestTheBannerSitsUnderTheInitiativeStripRatherThanOverIt(t *testing.T) {
	page := markup(t, Room(testRoomPage(room.RolePlayer)))
	strip := strings.Index(page, `id="`+RoomInitiativeID+`"`)
	banner := strings.Index(page, "<div data-on-deck ")
	if strip < 0 || banner < 0 {
		t.Fatalf("strip at %d, banner at %d", strip, banner)
	}
	if banner < strip {
		t.Error("the banner is rendered above the strip, so it would cover the portraits")
	}
	if got := strings.Count(page, "absolute inset-x-0 top-0 z-20"); got != 1 {
		t.Errorf("%d elements are pinned to the top of the table, want the one column holding both", got)
	}
}
func TestTheRoomPageCarriesWhatTheOnDeckAlertObeys(t *testing.T) {
	data := testRoomPage(room.RolePlayer)
	data.TurnAlert = true
	data.TurnNotify = true
	data.TurnVolume = 40
	on := tabletopTag(t, data)
	for _, want := range []string{"data-turn-alert", "data-turn-notify", `data-turn-volume="40"`} {
		if !strings.Contains(on, want) {
			t.Errorf("the tabletop is missing %s:\n%s", want, on)
		}
	}
	off := tabletopTag(t, testRoomPage(room.RolePlayer))
	if strings.Contains(off, "data-turn-notify") {
		t.Errorf("a stored false still asked the page to raise notifications:\n%s", off)
	}
	if strings.Contains(off, "data-turn-alert") {
		t.Errorf("a stored false still armed the alert:\n%s", off)
	}
}
func TestAStoredVolumeOutsideTheSliderComesBackOntoIt(t *testing.T) {
	data := testRoomPage(room.RolePlayer)
	data.TurnVolume = 400
	if got := data.TurnVolumeAttr(); got != "100" {
		t.Errorf("a stored 400 renders as %q, want %q", got, "100")
	}
}
