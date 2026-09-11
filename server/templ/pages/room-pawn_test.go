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
func TestThePawnPanelFiltersItsRefetchOnItsOwnID(t *testing.T) {
	body := decoded(t, RoomPawnFragment(testPawnPanel()))
	if !strings.Contains(body, "detail.id === '"+testPawnID+"'") {
		t.Errorf("the panel's trigger does not compare the pawn's own id:\n%s", body)
	}
	if !strings.Contains(body, "!"+typingInPanel) {
		t.Errorf("the panel refetches while somebody is typing in it:\n%s", body)
	}
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
func TestThePawnPanelQueuesItsRefetches(t *testing.T) {
	body := html(t, RoomPawnFragment(testPawnPanel()))
	if !strings.Contains(body, `hx-sync="this:queue last"`) {
		t.Errorf("the panel does not queue its refetches:\n%s", body)
	}
	if strings.Contains(body, "queue replace") || strings.Contains(body, `hx-sync="this:replace"`) {
		t.Error("the panel cancels requests in flight, which htmx reports as a console error")
	}
}
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
func TestTheStatBlockButtonIsTheGMsAlone(t *testing.T) {
	data := testPawnPanel()
	data.Pawn.MonsterID = "01BX5ZZKBKACTAV9WEVGEMMVT9"
	if !strings.Contains(html(t, RoomPawnFragment(data)), "Stat block") {
		t.Error("the GM has no stat block button")
	}
	data.IsGM = false
	data.Pawn.MonsterID = ""
	if strings.Contains(html(t, RoomPawnFragment(data)), "Stat block") {
		t.Error("a player was offered a stat block")
	}
}
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
func TestTheEditorIsInThePanelAndNotBehindAButton(t *testing.T) {
	body := decoded(t, RoomPawnFragment(testPawnPanel()))
	for _, field := range []string{`name="hp"`, `name="maxHp"`, `name="size"`, `name="ac"`, `name="conditionName"`} {
		if !strings.Contains(body, field) {
			t.Errorf("the panel has no %s:\n%s", field, body)
		}
	}
	for _, forbidden := range []string{"pawn/edit", ">Edit<"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the panel still reaches for the edit modal: %q", forbidden)
		}
	}
}
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
	for _, forbidden := range []string{`name="shown"`, `name="layer"`, `aria-label="` + data.RemoveLabel() + `"`} {
		if strings.Contains(player, forbidden) {
			t.Errorf("a player's panel carries %q", forbidden)
		}
	}
}
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
	creature := html(t, RoomPawnFragment(testPawnPanel()))
	for _, forbidden := range []string{`name="width"`, `name="height"`, `name="rotation"`} {
		if strings.Contains(creature, forbidden) {
			t.Errorf("a creature's panel carries %q", forbidden)
		}
	}
}
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
	row := html(t, RoomPawnConditionRow(testPawnID, RoomPawnCondition{Color: "red", Duration: "-1", Clear: "end"}))
	if !strings.Contains(row, data.NamesID()) {
		t.Errorf("a condition row does not name its own pawn's list:\n%s", row)
	}
}
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
func TestTheObjectSizeLimitMatchesTheProtocol(t *testing.T) {
	if ObjectPixelsMax != room.ObjectPixelsMax {
		t.Errorf("the form allows %d pixels and the core allows %d", ObjectPixelsMax, room.ObjectPixelsMax)
	}
}
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
func TestEveryBandHasAWord(t *testing.T) {
	for _, band := range room.HPBand("").Values() {
		if PawnBandText(band) == "" {
			t.Errorf("the band %q prints nothing", band)
		}
	}
	if got := PawnBandText("bloodied"); got != "" {
		t.Errorf("a band this build does not know printed %q", got)
	}
}
func TestACharacterIsNotRenamedFromItsPawnPanel(t *testing.T) {
	data := testPawnPanel()
	data.Pawn.Character = true
	if body := decoded(t, RoomPawnFragment(data)); strings.Contains(body, data.RenamePath()) {
		t.Errorf("a character's panel offers a rename:\n%s", body)
	}
	if body := decoded(t, RoomPawnFragment(data)); !strings.Contains(body, `name="hp"`) {
		t.Error("a character's panel lost its editor with its rename button")
	}
	data.Pawn.Character = false
	if body := decoded(t, RoomPawnFragment(data)); !strings.Contains(body, data.RenamePath()) {
		t.Errorf("a monster's panel has no rename:\n%s", body)
	}
}
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
	if strings.Count(body, "data-tip=") != strings.Count(body, `class="tooltip`) {
		t.Errorf("a data-tip is not inside a tooltip:\n%s", body)
	}
}
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
func TestTheEmptyErrorSlotTakesNoRoom(t *testing.T) {
	body := html(t, PanelFormErrors("pawn-x", nil))
	if body != `<div id="errors-pawn-x" hidden></div>` {
		t.Errorf("the empty error block is %q", body)
	}
	if full := html(t, PanelFormErrors("pawn-x", []string{"No."})); strings.Contains(full, "hidden") {
		t.Errorf("a block with something to say is hidden:\n%s", full)
	}
}
func TestAWindowScrollsDownAndNotAcross(t *testing.T) {
	body := html(t, roomWindowTemplate())
	if !strings.Contains(body, "overflow-x-hidden") || !strings.Contains(body, "overflow-y-auto") {
		t.Errorf("the window body does not clip its horizontal overflow:\n%s", body)
	}
}
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
	if strings.Index(body, `name="layer"`) > strings.Index(body, `name="shown"`) {
		t.Error("the visibility switch comes before the floor select")
	}
}
