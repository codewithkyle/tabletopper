package pages
import (
	"strings"
	"testing"
	"tabletopper/internal/room"
)
const testRoundRoomID = "01BX5ZZKBKACTAV9WEVGEMMVR0"
func TestTheRoundCounterPrintsTheRound(t *testing.T) {
	markup := html(t, RoomInitiativeRound(RoomInitiativeRoundData{RoomID: testRoundRoomID, Round: "3"}))
	if !strings.Contains(markup, "Round") || !strings.Contains(markup, ">3<") {
		t.Errorf("the counter does not print the round:\n%s", markup)
	}
}
func TestAnEmptyTrackerPrintsNoRound(t *testing.T) {
	markup := html(t, RoomInitiativeRound(RoomInitiativeRoundData{RoomID: testRoundRoomID}))
	if strings.Contains(markup, "Round") {
		t.Errorf("an empty tracker was given a counter anyway:\n%s", markup)
	}
	if !strings.Contains(markup, RoomInitiativeRoundID) || !strings.Contains(markup, "hx-get") {
		t.Errorf("the empty counter cannot hear the next event:\n%s", markup)
	}
}
func TestTheRoundCountersRefetchIsDeclaredInItsOwnMarkup(t *testing.T) {
	page := RoomInitiativeRoundData{RoomID: testRoundRoomID}
	answer := RoomInitiativeRoundData{RoomID: testRoundRoomID, Fetched: true}
	if !strings.HasPrefix(page.Trigger(), "load, ") {
		t.Errorf("the page render does not fetch once on load: %q", page.Trigger())
	}
	if strings.Contains(answer.Trigger(), "load") {
		t.Errorf("the answer re-arms its own load trigger: %q", answer.Trigger())
	}
	if !strings.Contains(answer.Trigger(), "room:initiative from:window") {
		t.Errorf("the counter does not listen for the tracker: %q", answer.Trigger())
	}
	if strings.ContainsAny(answer.Trigger(), "[]") {
		t.Errorf("the counter carries a filter it has no use for: %q", answer.Trigger())
	}
	markup := html(t, RoomInitiativeRound(answer))
	if !strings.Contains(markup, `hx-sync="this:queue last"`) {
		t.Error("the counter does not queue its refetches")
	}
	if !strings.Contains(markup, `hx-swap="outerHTML"`) {
		t.Error("the counter does not replace itself")
	}
}
func TestTheRoundCounterLeadsTheRightHandEndOfTheBar(t *testing.T) {
	for _, role := range []room.Role{room.RoleGM, room.RolePlayer} {
		page := markup(t, Room(testRoomPage(role)))
		round := strings.Index(page, RoomInitiativeRoundID)
		floor := strings.Index(page, floorAnchor(role))
		if round < 0 || floor < 0 {
			t.Fatalf("%s's bar is missing the round (%d) or the floor (%d)", role, round, floor)
		}
		if round > floor {
			t.Errorf("%s's round counter sits after the floor element", role)
		}
		if n := strings.Count(page[round:floor], "ml-auto"); n != 1 {
			t.Errorf("%s's bar has %d auto margins between the round and the floor, want one", role, n)
		}
		tag := page[floor:]
		tag = tag[:strings.Index(tag, ">")]
		if strings.Contains(tag, "ml-auto") {
			t.Errorf("%s's floor element still carries the auto margin: %s", role, tag)
		}
	}
}
func floorAnchor(role room.Role) string {
	if role == room.RoleGM {
		return "data-layer-bar"
	}
	return "room-layer-name"
}
func TestNextTurnNamesItsKey(t *testing.T) {
	var next RoomMenuItem
	for _, item := range menuNamed(t, testRoomPage(room.RoleGM), "Initiative").Items {
		if item.Label == "Next turn" {
			next = item
		}
	}
	if next.Key != "N" {
		t.Errorf("Next turn names the key %q, want N", next.Key)
	}
	page := markup(t, Room(testRoomPage(room.RoleGM)))
	if !strings.Contains(page, `<span>Next turn</span> <kbd class="kbd kbd-xs ml-auto">N</kbd>`) {
		t.Errorf("Next turn does not print its key:\n%s", page)
	}
	if strings.Count(page, "kbd kbd-xs ml-auto") != 1 {
		t.Errorf("%d menu items print a key, want one", strings.Count(page, "kbd kbd-xs ml-auto"))
	}
}
