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
// The activeElement half is what keeps a refetch from eating a keystroke, and it
// asks about a TYPING field rather than about the panel. A swap replaces the box
// somebody is halfway through filling in; a select or a checkbox has nothing
// half-entered to lose, and those are exactly the controls whose own save has to
// bring the panel back -- ticking "players can see this pawn" changes the Hidden
// badge in the header, and a refetch declined because the checkbox still had
// focus would leave the badge contradicting the box beside it.
func TestThePawnPanelFiltersItsRefetchOnItsOwnID(t *testing.T) {
	body := decoded(t, RoomPawnFragment(testPawnPanel()))

	if !strings.Contains(body, "detail.id === '"+testPawnID+"'") {
		t.Errorf("the panel's trigger does not compare the pawn's own id:\n%s", body)
	}
	if !strings.Contains(body, "!"+typingInPanel) {
		t.Errorf("the panel refetches while somebody is typing in it:\n%s", body)
	}
	// NEITHER A SQUARE BRACKET NOR A COMMA MAY APPEAR IN THE FILTER. It is
	// delimited by the brackets around it and the attribute is split on commas,
	// so a CSS selector written in here -- 'input[type=text],textarea' -- ends
	// the filter early and leaves the rest parsed as trigger modifiers. Nothing
	// errors; the panel just stops refetching.
	if strings.ContainsAny(typingInPanel, "[],") {
		t.Errorf("the filter carries a character that ends it early: %q", typingInPanel)
	}
	if !strings.Contains(typingInPanel, "'text'") || !strings.Contains(typingInPanel, "'number'") {
		t.Errorf("the filter does not cover both kinds of box the panel has: %q", typingInPanel)
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
	data.Pawn.Band = "Very bloody"

	body := html(t, RoomPawnFragment(data))
	if strings.Contains(body, `name="hp"`) {
		t.Error("a reader was given the hit-point field")
	}
	if !strings.Contains(body, "Very bloody") {
		t.Errorf("the band is not shown:\n%s", body)
	}
	if strings.Contains(body, "4 / 7") {
		t.Error("a projected pawn's panel printed numbers")
	}
}

// A form in a panel that refetches would throw away what was half-typed in it,
// which is exactly why the form used to be a modal. The panel's own trigger
// filter is the answer: it declines a refetch while a box in it has the caret,
// which was written for one hit-point field and now covers the whole editor.
func TestTheEditorIsInThePanelAndNotBehindAButton(t *testing.T) {
	body := decoded(t, RoomPawnFragment(testPawnPanel()))

	for _, field := range []string{`name="hp"`, `name="maxHp"`, `name="size"`, `name="ac"`, `name="conditionName"`} {
		if !strings.Contains(body, field) {
			t.Errorf("the panel has no %s:\n%s", field, body)
		}
	}

	// The edit modal is gone. The one dialog the panel still opens is the
	// rename, which is a different URL and is checked below.
	for _, forbidden := range []string{"pawn/edit", ">Edit<"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the panel still reaches for the edit modal: %q", forbidden)
		}
	}
}

// NOTHING IN THE PANEL IS SAVED BY PRESSING ANYTHING, and the missing button is
// the point rather than an omission: a Save below the fold of a 320 pixel window
// is a Save nobody finds, and the reader concludes the fields do not work.
//
// The two forms save on different events, which is why they are two. The editor
// debounces a keystroke; the hit-point boxes take arithmetic and wait for the
// entry to be finished, which is what change means.
func TestNothingInThePanelIsSavedByPressingAnything(t *testing.T) {
	body := decoded(t, RoomPawnFragment(testPawnPanel()))

	if strings.Contains(body, `type="submit"`) || strings.Contains(body, ">Save<") {
		t.Errorf("the panel still has a save button:\n%s", body)
	}
	if !strings.Contains(body, `hx-trigger="`+pawnSaveTrigger+`"`) {
		t.Errorf("the editor does not autosave:\n%s", body)
	}
	if !strings.Contains(body, `hx-trigger="change"`) {
		t.Errorf("the hit-point boxes do not save on change:\n%s", body)
	}
}

// THE TWO NUMBERS ARE ONE CONTROL. "4 / 7" is how a table says it, so they are
// one form on one route -- which is also what lets raising a maximum and healing
// to it be a single entry, since PawnUpdate clamps once after applying both.
//
// BOTH BOXES TAKE ARITHMETIC, which is the data-hp-math attribute js/room/hp.ts
// looks for. A box that had lost it would go on working and would stop doing
// sums, which is exactly the kind of quiet regression a marker like this exists
// to make loud.
func TestTheHitPointRowIsOneFormWithBothNumbers(t *testing.T) {
	data := testPawnPanel()
	body := decoded(t, RoomPawnFragment(data))

	if strings.Count(body, "data-hp-math") != 2 {
		t.Errorf("the two hit-point boxes do not both do arithmetic:\n%s", body)
	}
	if strings.Count(body, `hx-post="`+data.HPPath()+`"`) != 1 {
		t.Errorf("the hit-point row is not one form on the hit-point route:\n%s", body)
	}
	if strings.Contains(body, `type="number" inputmode="numeric" class="input input-sm validator peer w-full" value="7"`) {
		t.Error("the maximum is still a plain number field, which cannot take a sum")
	}
}

// THE NAME IS A HEADING WITH A BUTTON BESIDE IT AND NOT A FIELD. It is changed
// once in a session and read every second of it, and an autosaving text field
// would rename the pawn on every pause mid-word -- four renames to get to
// "Goblin archer", each one an event to everybody at the table.
func TestTheNameIsChangedInADialogAndNotInThePanel(t *testing.T) {
	data := testPawnPanel()
	body := decoded(t, RoomPawnFragment(data))

	if strings.Contains(body, `name="name"`) {
		t.Errorf("the panel still carries a name field:\n%s", body)
	}
	if !strings.Contains(body, `data-modal-open="`+data.RenamePath()+`"`) {
		t.Errorf("the panel has no rename button:\n%s", body)
	}
	if !strings.HasPrefix(data.RenamePath(), "/fragment/") {
		t.Errorf("the rename dialog is not a fragment, so the modal refuses it: %q", data.RenamePath())
	}

	// The dialog opens on the name the pawn has now, which is the whole reason
	// it is a fragment rather than markup on the page.
	dialog := decoded(t, RoomPawnRename(RoomPawnRenameData{
		RoomID: testPawnRoomID, PawnID: testPawnID, Name: "Goblin",
	}))
	if !strings.Contains(dialog, `value="Goblin"`) {
		t.Errorf("the rename dialog is not prefilled:\n%s", dialog)
	}
	if !strings.Contains(dialog, ">Close<") {
		t.Error("the rename dialog has no labelled Close beside its affirmative action")
	}
	if !strings.Contains(dialog, `hx-post="/rooms/`+testPawnRoomID+"/pawns/"+testPawnID+`/name"`) {
		t.Errorf("the rename dialog posts somewhere other than the rename route:\n%s", dialog)
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
	if !strings.Contains(gm, `aria-label="`+data.RemoveLabel()+`"`) {
		t.Error("the GM has no way to remove the pawn")
	}

	data.IsGM = false
	data.Layers = nil

	player := html(t, RoomPawnFragment(data))
	// The whole aria-label and not the word: every condition row carries a
	// "Remove condition" control, which is the player's to press.
	for _, forbidden := range []string{`name="shown"`, `name="layer"`, `aria-label="` + data.RemoveLabel() + `"`} {
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

	for _, bare := range []string{`id="hp"`, `id="size"`, `id="maxHp"`, `id="ac"`, `id="pawn-conditions"`, `id="condition-names"`, `id="pawn-form"`} {
		if strings.Contains(body, bare) {
			t.Errorf("the panel carries a shared id: %q", bare)
		}
	}
	for _, own := range []string{data.Field("hp"), data.Field("maxHp"), data.ConditionsID(), data.NamesID(), data.FormID()} {
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

// NEITHER FORM SWAPS THE PANEL, and that is what makes autosaving safe. Both
// post while somebody is still working in the window -- one a few hundred
// milliseconds after a keystroke, the other the moment a box is left -- so a
// reply that replaced the panel would take away the field they had moved on to.
// Both answer with the one error slot instead, and the socket is what brings
// every open copy back into step.
func TestNeitherFormSwapsThePanel(t *testing.T) {
	data := testPawnPanel()
	body := decoded(t, RoomPawnFragment(data))

	if strings.Contains(body, `hx-target="#`+data.ElementID()+`"`) {
		t.Errorf("a save swaps the panel it is being typed into:\n%s", body)
	}
	if strings.Count(body, `hx-target="#errors-`+data.Panel()+`"`) != 2 {
		t.Errorf("the two forms do not both answer with the error slot:\n%s", body)
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

// EVERY BAND HAS A WORD, and a band added to the protocol without one here is a
// pawn window that says nothing about a monster's health while the panel on the
// table says "Bloody" -- the reader's own copy of the panel, disagreeing with
// it. The TypeScript half of the pair is js/room/overlay.test.ts.
func TestEveryBandHasAWord(t *testing.T) {
	for _, band := range room.HPBand("").Values() {
		if PawnBandText(band) == "" {
			t.Errorf("the band %q prints nothing", band)
		}
	}

	// A band from a server a version ahead prints nothing rather than its own
	// value: "veryBloody" over a goblin reads as a bug, and an empty line reads
	// as a pawn nobody has filled in.
	if got := PawnBandText("bloodied"); got != "" {
		t.Errorf("a band this build does not know printed %q", got)
	}
}

// A PLAYER'S CHARACTER HAS NO RENAME BUTTON. The name came off the sheet its
// player wrote and every other player reads it in the turn order; the one
// plausible reason to change it here is a typo, which is a thing to fix on the
// sheet where it will still be right next session. A goblin is the opposite
// case, which is what the button is for.
func TestACharacterIsNotRenamedFromItsPawnPanel(t *testing.T) {
	data := testPawnPanel()
	data.Pawn.Character = true

	if body := decoded(t, RoomPawnFragment(data)); strings.Contains(body, data.RenamePath()) {
		t.Errorf("a character's panel offers a rename:\n%s", body)
	}

	// Everything else about the panel is unchanged: this is one button, not a
	// read-only pawn.
	if body := decoded(t, RoomPawnFragment(data)); !strings.Contains(body, `name="hp"`) {
		t.Error("a character's panel lost its editor with its rename button")
	}

	data.Pawn.Character = false
	if body := decoded(t, RoomPawnFragment(data)); !strings.Contains(body, data.RenamePath()) {
		t.Errorf("a monster's panel has no rename:\n%s", body)
	}
}

// EVERY ICON BUTTON IN THE PANEL CARRIES A TOOLTIP AND AN ACCESSIBLE NAME, and
// the two are different strings on purpose. The tip is read beside the thing it
// points at, so "Remove" is unambiguous; a screen reader announces the button
// with no such context, and eight goblins on a table means eight buttons that
// would otherwise all announce as "Remove".
func TestEveryIconButtonInThePanelIsLabelledTwice(t *testing.T) {
	data := testPawnPanel()
	data.Pawn.MonsterID = "01BX5ZZKBKACTAV9WEVGEMMVT9"

	body := html(t, RoomPawnFragment(data))

	for tip, label := range map[string]string{
		"Stat block":       data.StatBlockLabel(),
		"Rename":           data.RenameLabel(),
		"Remove":           data.RemoveLabel(),
		"Remove condition": "Remove condition",
	} {
		if !strings.Contains(body, `data-tip="`+tip+`"`) {
			t.Errorf("no tooltip %q in the panel:\n%s", tip, body)
		}
		if !strings.Contains(body, `aria-label="`+label+`"`) {
			t.Errorf("no accessible name %q in the panel", label)
		}
	}

	// A tip with no tooltip class around it renders nothing at all, which is
	// the failure a data-tip typo produces and the one nobody notices.
	if strings.Count(body, "data-tip=") != strings.Count(body, `class="tooltip`) {
		t.Errorf("a data-tip is not inside a tooltip:\n%s", body)
	}
}

// THE VISIBILITY CONTROL SAYS WHICH STATE IT IS IN, in words, and it swaps them
// without waiting for the socket. A switch labelled "Players can see this pawn"
// makes the reader work out what the position of the switch means; two words
// behind peer-checked do not, and the answer is right the instant it is
// clicked rather than a round trip later.
func TestTheVisibilityControlReadsVisibleOrHidden(t *testing.T) {
	body := html(t, RoomPawnFragment(testPawnPanel()))

	if !strings.Contains(body, "toggle") {
		t.Errorf("the visibility control is not a switch:\n%s", body)
	}
	for _, word := range []string{">Visible<", ">Hidden<"} {
		if !strings.Contains(body, word) {
			t.Errorf("the switch never says %q:\n%s", word, body)
		}
	}
	if !strings.Contains(body, "peer-checked:hidden") || !strings.Contains(body, "peer-checked:inline") {
		t.Error("the two words are not swapped by the switch's own state")
	}
}

// SPACE IN A 260 PIXEL WINDOW IS THE SCARCE THING, so every control in this
// panel is the extra-small one -- the size the tabletop settings window already
// uses. A single `input-sm` here is a row a third taller than the rows above
// and below it, which is how a panel drifts back to being too tall.
func TestEveryControlInThePanelIsCompact(t *testing.T) {
	data := testPawnPanel()
	data.LayerID = "01BX5ZZKBKACTAV9WEVGEMMVT1"
	data.Layers = []RoomPawnLayer{
		{ID: "01BX5ZZKBKACTAV9WEVGEMMVT1", Name: "Ground floor"},
		{ID: "01BX5ZZKBKACTAV9WEVGEMMVT2", Name: "Cellar"},
	}
	data.Pawn.MonsterID = "01BX5ZZKBKACTAV9WEVGEMMVT9"

	for _, body := range []string{
		html(t, RoomPawnFragment(data)),
		html(t, RoomPawnConditionRow(data.Pawn.ID, data.Pawn.Conditions[0])),
	} {
		for _, bulky := range []string{"input-sm", "select-sm", "btn-sm", "input-md", "select-md", "fieldset-legend"} {
			if strings.Contains(body, bulky) {
				t.Errorf("the panel carries %q:\n%s", bulky, body)
			}
		}
	}
}

// The empty error block is display:none rather than a zero-height element,
// because its parents are flex columns with a gap: an invisible child still
// takes a gap on each side of itself, which is two gaps of nothing above the
// hit-point row on every panel that has nothing to complain about.
func TestTheEmptyErrorSlotTakesNoRoom(t *testing.T) {
	body := html(t, PanelFormErrors("pawn-x", nil))
	if body != `<div id="errors-pawn-x" hidden></div>` {
		t.Errorf("the empty error block is %q", body)
	}

	if full := html(t, PanelFormErrors("pawn-x", []string{"No."})); strings.Contains(full, "hidden") {
		t.Errorf("a block with something to say is hidden:\n%s", full)
	}
}

// A WINDOW SCROLLS DOWN AND NEVER ACROSS.
//
// A panel in a window is a column of controls whose width the reader chose by
// dragging an edge, so a horizontal scrollbar is never the answer to anything:
// it is how a panel that failed to reflow reports itself. Clipping makes that a
// visible bug in the panel rather than a bar the reader has to use.
func TestAWindowScrollsDownAndNotAcross(t *testing.T) {
	body := html(t, roomWindowTemplate())

	if !strings.Contains(body, "overflow-x-hidden") || !strings.Contains(body, "overflow-y-auto") {
		t.Errorf("the window body does not clip its horizontal overflow:\n%s", body)
	}
}

// WHICH IS WHY EVERY TOOLTIP IN A WINDOW POINTS LEFT. DaisyUI positions a tip
// absolutely inside the element it belongs to and leaves it in the layout at
// zero opacity, so a tip centred over a button at the panel's right edge
// overhangs that edge whether or not anybody is hovering -- and an overhang
// inside a scroll container is horizontal overflow. Pointing left puts the whole
// tip over the panel, where there is always room for it.
func TestEveryTooltipInAWindowPointsIntoThePanel(t *testing.T) {
	data := testPawnPanel()
	data.Pawn.MonsterID = "01BX5ZZKBKACTAV9WEVGEMMVT9"

	for name, body := range map[string]string{
		"the pawn panel": html(t, RoomPawnFragment(data)),
		"the chrome":     html(t, roomWindowTemplate()),
		"a condition":    html(t, RoomPawnConditionRow(data.Pawn.ID, data.Pawn.Conditions[0])),
	} {
		if strings.Count(body, `class="tooltip`) != strings.Count(body, "tooltip-left") {
			t.Errorf("%s carries a tooltip that is not tooltip-left:\n%s", name, body)
		}
	}
}

// The condition row stacks rather than overflowing: five controls need about
// 260 pixels of fixed width between them, which is more than a 320 pixel window
// has once its padding and its scrollbar are taken off. A fixed grid track is
// what it used to be and is what cannot shrink.
func TestAConditionRowCanShrinkToTheWindow(t *testing.T) {
	body := html(t, RoomPawnConditionRow(testPawnID, RoomPawnCondition{
		ID: "01BX5ZZKBKACTAV9WEVGEMMVT5", Name: "Prone", Color: "red", Duration: "-1", Clear: "end",
	}))

	if strings.Contains(body, "grid-cols-[") {
		t.Errorf("the condition row is a fixed grid and cannot reflow:\n%s", body)
	}
	if !strings.Contains(body, "@sm:flex-row") {
		t.Errorf("the condition row never becomes one line:\n%s", body)
	}
}

// VISIBILITY AND THE FLOOR ARE THE TWO CONTROLS A GM REACHES FOR MID-FIGHT, so
// they are the first row of the editor rather than the last. Under the
// conditions they sat below the fold of the default window on any pawn carrying
// two or three of them, which reads as a control that does not exist.
func TestTheGMsTwoLiveControlsComeBeforeTheConditions(t *testing.T) {
	data := testPawnPanel()
	data.LayerID = "01BX5ZZKBKACTAV9WEVGEMMVT1"
	data.Layers = []RoomPawnLayer{
		{ID: "01BX5ZZKBKACTAV9WEVGEMMVT1", Name: "Ground floor"},
		{ID: "01BX5ZZKBKACTAV9WEVGEMMVT2", Name: "Cellar"},
	}

	body := html(t, RoomPawnFragment(data))

	conditions := strings.Index(body, `name="conditionName"`)
	if conditions < 0 {
		t.Fatalf("the panel has no conditions:\n%s", body)
	}
	for _, control := range []string{`name="shown"`, `name="layer"`} {
		at := strings.Index(body, control)
		if at < 0 {
			t.Fatalf("the GM has no %s control:\n%s", control, body)
		}
		if at > conditions {
			t.Errorf("%s is below the conditions", control)
		}
	}

	// The floor first and the switch after it: the select is the one that can
	// use a long row, and a floor name is as long as the GM made it.
	if strings.Index(body, `name="layer"`) > strings.Index(body, `name="shown"`) {
		t.Error("the visibility switch comes before the floor select")
	}
}
