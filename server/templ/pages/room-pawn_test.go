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
			Conditions: []RoomPawnCondition{
				{ID: "01BX5ZZKBKACTAV9WEVGEMMVT5", Name: "Prone", Color: "red", Duration: "-1", Clear: "end"},
			},
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

// A form in a panel that refetches would throw away what was half-typed in it,
// which is exactly why the form used to be a modal. The panel's own trigger
// filter is the answer: it declines every refetch while focus is inside it,
// which was written for one hit-point field and now covers the whole editor.
func TestTheEditorIsInThePanelAndNotBehindAButton(t *testing.T) {
	body := decoded(t, RoomPawnFragment(testPawnPanel()))

	for _, field := range []string{`name="name"`, `name="size"`, `name="maxHp"`, `name="ac"`, `name="conditionName"`} {
		if !strings.Contains(body, field) {
			t.Errorf("the panel has no %s:\n%s", field, body)
		}
	}

	// The modal is gone, and with it every way of asking for one.
	for _, forbidden := range []string{"data-modal-open", "modal:close", "pawn/edit", ">Edit<"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the panel still reaches for the edit modal: %q", forbidden)
		}
	}
}

// Visibility and the floor are the GM's, and the second is not a matter of
// taste: PawnSetLayer refuses a player, because a player who sent their own
// pawn upstairs would stop being sent it and would be holding a pawn they can
// no longer see.
func TestTheGMsControlsRenderOnlyForTheGM(t *testing.T) {
	data := testPawnPanel()
	data.LayerID = "01BX5ZZKBKACTAV9WEVGEMMVT1"
	data.Layers = []RoomPawnLayer{
		{ID: "01BX5ZZKBKACTAV9WEVGEMMVT1", Name: "Ground floor"},
		{ID: "01BX5ZZKBKACTAV9WEVGEMMVT2", Name: "Cellar"},
	}

	gm := html(t, RoomPawnFragment(data))
	if !strings.Contains(gm, `name="shown"`) {
		t.Error("the GM has no visibility toggle")
	}
	if !strings.Contains(gm, `name="layer"`) {
		t.Error("the GM has no floor select")
	}
	if !strings.Contains(gm, ">Remove<") {
		t.Error("the GM has no way to remove the pawn")
	}

	data.IsGM = false
	data.Layers = nil

	player := html(t, RoomPawnFragment(data))
	// ">Remove<" and not "Remove": every condition row carries a "Remove
	// condition" control, which is the player's to press.
	for _, forbidden := range []string{`name="shown"`, `name="layer"`, ">Remove<"} {
		if strings.Contains(player, forbidden) {
			t.Errorf("a player's panel carries %q", forbidden)
		}
	}
}

// An object is measured in pixels instead of by a creature size and cannot be
// poisoned, which is the protocol's rule -- PawnSetConditions refuses an object
// outright -- and the panel must not offer what the server will not take.
func TestTheObjectPanelHasASizeInPixelsAndNoConditions(t *testing.T) {
	data := testPawnPanel()
	data.Pawn.Object = true
	data.Pawn.Width = "128"
	data.Pawn.Height = "256"
	data.Pawn.SizeValue = ""

	body := html(t, RoomPawnFragment(data))
	if !strings.Contains(body, `name="width"`) || !strings.Contains(body, `name="height"`) {
		t.Errorf("an object's panel has no size fields:\n%s", body)
	}
	if strings.Contains(body, `name="size"`) {
		t.Error("an object's panel offers a creature size")
	}
	if strings.Contains(body, `name="conditionName"`) {
		t.Error("an object's panel offers conditions, which the core refuses")
	}
	if !strings.Contains(body, `name="rotation"`) {
		t.Errorf("an object's panel has no angle:\n%s", body)
	}

	// AND A CREATURE'S PANEL HAS NONE OF THE THREE. PawnUpdate refuses a width,
	// a height or an angle on anything that is not an object, so a field here
	// would be a save that comes back refused.
	creature := html(t, RoomPawnFragment(testPawnPanel()))
	for _, forbidden := range []string{`name="width"`, `name="height"`, `name="rotation"`} {
		if strings.Contains(creature, forbidden) {
			t.Errorf("a creature's panel carries %q", forbidden)
		}
	}
}

// TWO PANELS ARE OPEN AT ONCE AND NEITHER MAY OWN A BARE id. A <label for> that
// named "name" would put the caret in the other window's field, an Add
// condition aimed at "pawn-conditions" would append the row to whichever panel
// happened to be first in the document, and two <datalist id="condition-names">
// is invalid markup that stops working the moment one window is closed.
func TestEveryIDInThePanelCarriesThePawnsOwn(t *testing.T) {
	data := testPawnPanel()
	body := html(t, RoomPawnFragment(data))

	for _, bare := range []string{`id="name"`, `id="size"`, `id="maxHp"`, `id="ac"`, `id="pawn-conditions"`, `id="condition-names"`, `id="pawn-form"`} {
		if strings.Contains(body, bare) {
			t.Errorf("the panel carries a shared id: %q", bare)
		}
	}
	for _, own := range []string{data.Field("name"), data.ConditionsID(), data.NamesID(), data.FormID()} {
		if !strings.Contains(body, `"`+own+`"`) {
			t.Errorf("the panel does not carry %q", own)
		}
	}

	// AND THE ROW FRAGMENT AGREES WITH THE PANEL IT LANDS IN. The Add button
	// fetches a row on its own and htmx appends it; a row pointing at another
	// pawn's datalist would offer the twenty names from the wrong window, or
	// none once that window was closed.
	row := html(t, RoomPawnConditionRow(testPawnID, RoomPawnCondition{Color: "red", Duration: "-1", Clear: "end"}))
	if !strings.Contains(row, data.NamesID()) {
		t.Errorf("a condition row does not name its own pawn's list:\n%s", row)
	}
}

// Both forms in the panel swap the panel, because both are mutations answering
// with what they changed -- which is the case the fragment rules name for a
// route outside /fragment/. A Save that answered 204 would leave the window
// showing what the pawn was until the socket came back round.
func TestBothOfThePanelsFormsSwapThePanel(t *testing.T) {
	data := testPawnPanel()
	body := decoded(t, RoomPawnFragment(data))

	if strings.Count(body, `hx-target="#`+data.ElementID()+`"`) != 2 {
		t.Errorf("the two forms do not both answer with the panel:\n%s", body)
	}
	if strings.Count(body, "target:#errors-"+data.Panel()) != 2 {
		t.Errorf("the two forms do not share one error slot:\n%s", body)
	}
}

// The size line is one sentence with two kinds of answer, and an unturned token
// says nothing about its angle -- zero degrees is the absence of a rotation
// rather than a fact about the wagon, and the clause would be on every one.
func TestAnObjectsSizeLinePrintsItsAngleOnlyWhenItHasOne(t *testing.T) {
	if got := PawnPixelsText(200, 140, 0); got != "200 by 140 pixels" {
		t.Errorf("an unturned object reads %q", got)
	}
	if got := PawnPixelsText(200, 140, 30); got != "200 by 140 pixels, turned 30 degrees" {
		t.Errorf("a turned object reads %q", got)
	}
	if got := PawnPixelsText(0, 140, 30); got != "" {
		t.Errorf("an object with no recorded size reads %q, want silence", got)
	}
}

// The form's max attribute and the core's limit are one number in two places,
// and a form that accepts what the server refuses is a save that fails with a
// message about a limit the field said was fine.
func TestTheObjectSizeLimitMatchesTheProtocol(t *testing.T) {
	if ObjectPixelsMax != room.ObjectPixelsMax {
		t.Errorf("the form allows %d pixels and the core allows %d", ObjectPixelsMax, room.ObjectPixelsMax)
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
