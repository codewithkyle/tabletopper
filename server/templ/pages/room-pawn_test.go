package pages

import (
	"bytes"
	"context"
	stdhtml "html"
	"strings"
	"testing"

	"tabletopper/internal/room"

	"github.com/a-h/templ"
)

const (
	testPawnRoomID = "01BX5ZZKBKACTAV9WEVGEMMVT0"
	testPawnID     = "01BX5ZZKBKACTAV9WEVGEMMVT7"
	testOtherPawn  = "01BX5ZZKBKACTAV9WEVGEMMVT8"
)

func html(t *testing.T, c templ.Component) string {
	t.Helper()

	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render failed: %v", err)
	}

	return buf.String()
}

// decoded is the markup as the BROWSER reads it rather than as templ wrote it.
//
// AN htmx TRIGGER FILTER IS JAVASCRIPT INSIDE AN ATTRIBUTE, so templ escapes
// its quotes and its ampersands and the HTML parser puts them back before htmx
// ever sees the string. A test asserting on the raw output would be asserting
// on templ's escaping rules, and would fail the day they changed while the
// filter went on working perfectly.
func decoded(t *testing.T, c templ.Component) string {
	t.Helper()

	return stdhtml.UnescapeString(html(t, c))
}

func testPawnPanel() RoomPawnData {
	return RoomPawnData{
		RoomID:  testPawnRoomID,
		IsGM:    true,
		CanEdit: true,
		Pawn: RoomPawn{
			ID:      testPawnID,
			Name:    "Goblin",
			HP:      "4 / 7",
			HPValue: "4",
			MaxHP:   "7",
			AC:      "15",
			Size:    "Small",
		},
	}
}

// TEN PAWN WINDOWS OPEN AND ONE GOBLIN TAKING DAMAGE MUST BE ONE REQUEST.
//
// panels.ts raises room:pawn carrying the id that changed, and this filter is
// the other half: every open panel hears the event and all but one decline.
// Without it, a fight is a GET per open window per round, which is exactly the
// cost the refetch pattern was chosen on the assumption of NOT paying.
//
// The activeElement half is what keeps a refetch from eating a keystroke. The
// panel carries one editable field -- hit points, the value that changes every
// round -- and a swap while somebody is typing into it replaces the field and
// loses what was in it.
func TestThePawnPanelFiltersItsRefetchOnItsOwnID(t *testing.T) {
	body := decoded(t, RoomPawnFragment(testPawnPanel()))

	if !strings.Contains(body, "detail.id === '"+testPawnID+"'") {
		t.Errorf("the panel's trigger does not compare the pawn's own id:\n%s", body)
	}
	if !strings.Contains(body, "!this.contains(document.activeElement)") {
		t.Error("the panel refetches while focus is inside it, which eats a keystroke in the hit-point field")
	}
	if !strings.Contains(body, "from:window") {
		t.Error("the panel listens on itself rather than on window, where panels.ts dispatches")
	}
}

// A BURST OF EVENTS IS A BURST OF GETS WHOSE ANSWERS CAN LAND IN EITHER ORDER.
// The socket is ordered; a pair of HTTP requests is not. "queue last" runs them
// one at a time and keeps only the newest pending one.
//
// NEVER "replace": it cancels the request in flight, and htmx reports every
// cancellation as a console error -- including on the ordinary page load, where
// the window's own content fetch is still open when the first snapshot fires.
func TestThePawnPanelQueuesItsRefetches(t *testing.T) {
	body := html(t, RoomPawnFragment(testPawnPanel()))

	if !strings.Contains(body, `hx-sync="this:queue last"`) {
		t.Errorf("the panel does not queue its refetches:\n%s", body)
	}
	if strings.Contains(body, "queue replace") || strings.Contains(body, `hx-sync="this:replace"`) {
		t.Error("the panel cancels requests in flight, which htmx reports as a console error")
	}
}

// Two panels are open as a matter of course, so every id in one has to carry
// the pawn's own. An error slot shared between two windows is a POST whose
// errors land in somebody else's.
func TestTwoPawnPanelsShareNoIDs(t *testing.T) {
	first := testPawnPanel()
	second := testPawnPanel()
	second.Pawn.ID = testOtherPawn

	if first.ElementID() == second.ElementID() {
		t.Error("two panels share an element id")
	}
	if first.Panel() == second.Panel() {
		t.Error("two panels share an error slot")
	}

	body := html(t, RoomPawnFragment(second))
	if strings.Contains(body, testPawnID) {
		t.Error("a panel rendered another pawn's id")
	}
}

// The stat block is the GM's. The room's monster-health setting exists so a
// table can hide a monster's hit points; a stat block carries those, its armour
// class and its legendary actions, so a button offering one to a player would
// contradict in one window the setting the GM chose in another.
func TestTheStatBlockButtonIsTheGMsAlone(t *testing.T) {
	data := testPawnPanel()
	data.Pawn.MonsterID = "01BX5ZZKBKACTAV9WEVGEMMVT9"

	if !strings.Contains(html(t, RoomPawnFragment(data)), "Stat block") {
		t.Error("the GM has no stat block button")
	}

	// A player's projected pawn never carries a monster id -- pawnView only
	// sets it for RoleGM -- so the button cannot render for them even if the
	// flag were wrong.
	data.IsGM = false
	data.Pawn.MonsterID = ""
	if strings.Contains(html(t, RoomPawnFragment(data)), "Stat block") {
		t.Error("a player was offered a stat block")
	}
}

// A player who may not edit sees the reading and not the field, and a monster
// the room projects as a band shows the word rather than a blank.
func TestAReaderSeesTheReadingAndNotTheField(t *testing.T) {
	data := testPawnPanel()
	data.IsGM = false
	data.CanEdit = false
	data.Pawn.HP = ""
	data.Pawn.HPValue = ""
	data.Pawn.MaxHP = ""
	data.Pawn.Band = "Bloodied"

	body := html(t, RoomPawnFragment(data))
	if strings.Contains(body, `name="hp"`) {
		t.Error("a reader was given the hit-point field")
	}
	if !strings.Contains(body, "Bloodied") {
		t.Errorf("the band is not shown:\n%s", body)
	}
	if strings.Contains(body, "4 / 7") {
		t.Error("a projected pawn's panel printed numbers")
	}
}

func testPawnForm() RoomPawnFormData {
	return RoomPawnFormData{
		RoomID: testPawnRoomID,
		IsGM:   true,
		Shown:  true,
		Pawn: RoomPawn{
			ID:        testPawnID,
			Name:      "Goblin",
			MaxHP:     "7",
			AC:        "15",
			SizeValue: "small",
		},
		LayerID: "01BX5ZZKBKACTAV9WEVGEMMVT1",
		Layers: []RoomPawnLayer{
			{ID: "01BX5ZZKBKACTAV9WEVGEMMVT1", Name: "Ground floor"},
			{ID: "01BX5ZZKBKACTAV9WEVGEMMVT2", Name: "Cellar"},
		},
	}
}

// Visibility and the floor are the GM's, and the second is not a matter of
// taste: PawnSetLayer refuses a player, because a player who sent their own
// pawn upstairs would stop being sent it and would be holding a pawn they can
// no longer see.
func TestTheGMsControlsRenderOnlyForTheGM(t *testing.T) {
	gm := html(t, RoomPawnForm(testPawnForm()))
	if !strings.Contains(gm, `name="shown"`) {
		t.Error("the GM has no visibility toggle")
	}
	if !strings.Contains(gm, `name="layer"`) {
		t.Error("the GM has no floor select")
	}
	if !strings.Contains(gm, "Remove") {
		t.Error("the GM has no way to remove the pawn")
	}

	data := testPawnForm()
	data.IsGM = false
	data.Layers = nil

	player := html(t, RoomPawnForm(data))
	for _, forbidden := range []string{`name="shown"`, `name="layer"`, "Remove"} {
		if strings.Contains(player, forbidden) {
			t.Errorf("a player's form carries %q", forbidden)
		}
	}
}

// An object has a rectangle instead of a size and cannot be poisoned, which is
// the protocol's rule -- PawnSetConditions refuses an object outright -- and
// the form must not offer what the server will not take.
func TestTheObjectFormHasAFootprintAndNoConditions(t *testing.T) {
	data := testPawnForm()
	data.Pawn.Object = true
	data.Pawn.FootprintW = "2"
	data.Pawn.FootprintH = "4"
	data.Pawn.SizeValue = ""

	body := html(t, RoomPawnForm(data))
	if !strings.Contains(body, `name="footprintW"`) || !strings.Contains(body, `name="footprintH"`) {
		t.Errorf("an object's form has no footprint fields:\n%s", body)
	}
	if strings.Contains(body, `name="size"`) {
		t.Error("an object's form offers a creature size")
	}
	if strings.Contains(body, `name="conditionName"`) {
		t.Error("an object's form offers conditions, which the core refuses")
	}
}

// The Close button inside the form cannot be a <form method="dialog">, because
// forms do not nest. It dispatches the event instead, and Close comes first.
func TestTheFormClosesWithoutNestingAForm(t *testing.T) {
	body := html(t, RoomPawnForm(testPawnForm()))

	if !strings.Contains(body, "modal:close") {
		t.Error("the form has no way out")
	}
	if strings.Contains(body, `method="dialog"`) {
		t.Error("the form nests a form, which the browser drops")
	}
	if strings.Index(body, "Close") > strings.Index(body, ">Save<") {
		t.Error("Save comes before Close; every dialog in this app puts Close first")
	}
}

// The form's max attribute and the core's limit are one number in two places,
// and a form that accepts what the server refuses is a save that fails with a
// message about a limit the field said was fine.
func TestTheFootprintLimitMatchesTheProtocol(t *testing.T) {
	if FootprintCellsMax != room.FootprintMax {
		t.Errorf("the form allows %d cells and the core allows %d", FootprintCellsMax, room.FootprintMax)
	}
}

// The eight colours the select offers are the eight the protocol validates, so
// a chip cannot be saved in a colour the server will refuse -- and the swatch
// has a branch for each, because a class name built in Go is never emitted by
// Tailwind.
func TestTheConditionColoursAreTheProtocolsOwn(t *testing.T) {
	valid := room.ConditionColor("").Values()
	if len(ConditionColors) != len(valid) {
		t.Fatalf("the form offers %d colours and the protocol knows %d", len(ConditionColors), len(valid))
	}

	for _, color := range ConditionColors {
		if !room.ConditionColor(color).Valid() {
			t.Errorf("the form offers %q, which the core refuses", color)
		}

		body := html(t, pawnConditionSwatch(color))
		if !strings.Contains(body, "bg-") {
			t.Errorf("%q draws no swatch; a colour with no branch renders nothing", color)
		}
	}
}
