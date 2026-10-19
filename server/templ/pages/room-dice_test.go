package pages

import (
	"encoding/json"
	"strings"
	"testing"

	"tabletopper/internal/room"
	"tabletopper/internal/uievents"
)

const testDiceRoomID = "01BX5ZZKBKACTAV9WEVGEMMVW0"

func diceTray(rolls ...RoomDiceRoll) RoomDiceData {
	return RoomDiceData{RoomID: testDiceRoomID, Rolls: rolls}
}
func attackRoll() RoomDiceRoll {
	return RoomDiceRoll{
		ID: "01ROLL", Who: "Ilyana", Label: "Longsword", Expr: "1d20 + 7",
		Dice:  []RoomDie{{Value: 17, Kept: true, Sign: 1}},
		Mod:   7,
		Total: 24,
	}
}

func TestTheLogRefetchesItselfAndLeavesTheFormAlone(t *testing.T) {
	markup := decoded(t, RoomDice(diceTray(attackRoll())))
	if !strings.Contains(markup, `hx-trigger="`+uievents.Rolls+` from:window"`) {
		t.Errorf("the log does not listen for %q:\n%s", uievents.Rolls, markup)
	}
	if !strings.Contains(markup, `hx-select="#`+RoomDiceLogID+`"`) {
		t.Error("the log does not select itself out of the response, so a refetch would wipe what is being typed")
	}
	if !strings.Contains(markup, `hx-sync="this:queue last"`) {
		t.Error("the log does not queue its refetches")
	}
	if !strings.Contains(markup, `hx-swap="none"`) {
		t.Error("the form swaps its own response; the socket is what repaints the log")
	}
}
func TestARollReadsAsWhatWasRolled(t *testing.T) {
	markup := decoded(t, roomDiceRow(attackRoll()))
	for _, want := range []string{"Ilyana", "Longsword", "1d20 + 7", ">17<", "+ 7", ">24<"} {
		if !strings.Contains(markup, want) {
			t.Errorf("the row is missing %q:\n%s", want, markup)
		}
	}
}
func TestTheFlourishFollowsTheDiceAndNotTheTotal(t *testing.T) {
	crit := attackRoll()
	crit.Crit = true
	fumble := attackRoll()
	fumble.Fumble = true
	cases := map[string]struct {
		roll RoomDiceRoll
		want string
		deny string
	}{
		"a crit is marked":        {crit, "text-success", "text-error"},
		"a fumble is marked":      {fumble, "text-error", "text-success"},
		"an ordinary roll is not": {attackRoll(), "", ""},
	}
	for name, tc := range cases {
		markup := decoded(t, roomDiceRow(tc.roll))
		if tc.want != "" && !strings.Contains(markup, tc.want) {
			t.Errorf("%s: the total is not %s:\n%s", name, tc.want, markup)
		}
		if tc.deny != "" && strings.Contains(markup, tc.deny) {
			t.Errorf("%s: the total also carries %s", name, tc.deny)
		}
		if tc.want == "" && (strings.Contains(markup, "text-success") || strings.Contains(markup, "text-error")) {
			t.Errorf("%s: an ordinary roll flourished:\n%s", name, markup)
		}
	}
}
func TestADroppedDieIsShownStruckThroughRatherThanHidden(t *testing.T) {
	roll := RoomDiceRoll{
		ID: "01ROLL", Who: "Kyle", Expr: "4d6kh3", Total: 12,
		Dice: []RoomDie{
			{Value: 5, Kept: true, Sign: 1},
			{Value: 4, Kept: true, Sign: 1},
			{Value: 3, Kept: true, Sign: 1},
			{Value: 2, Kept: false, Sign: 1},
		},
	}
	markup := decoded(t, roomDiceRow(roll))
	if !strings.Contains(markup, "line-through") {
		t.Errorf("the dropped die is not struck through:\n%s", markup)
	}
	if !strings.Contains(markup, ">2<") {
		t.Errorf("the dropped die is missing from the row entirely:\n%s", markup)
	}
}
func TestASubtractedDieCarriesItsSign(t *testing.T) {
	roll := RoomDiceRoll{
		ID: "01ROLL", Who: "Kyle", Expr: "2d6 - 1d4", Total: 6,
		Dice: []RoomDie{
			{Value: 4, Kept: true, Sign: 1},
			{Value: 5, Kept: true, Sign: 1},
			{Value: 3, Kept: true, Sign: -1},
		},
	}
	if markup := decoded(t, roomDiceRow(roll)); !strings.Contains(markup, ">-3<") {
		t.Errorf("the subtracted die does not show its sign:\n%s", markup)
	}
}
func TestASecretRollSaysSoAndAnOpenOneDoesNot(t *testing.T) {
	secret := attackRoll()
	secret.Secret = true
	if markup := decoded(t, roomDiceRow(secret)); !strings.Contains(markup, "Secret") {
		t.Errorf("a secret roll is not marked in its own roller's log:\n%s", markup)
	}
	if markup := decoded(t, roomDiceRow(attackRoll())); strings.Contains(markup, "Secret") {
		t.Errorf("an open roll was marked secret:\n%s", markup)
	}
}
func TestAdvantageIsNamedOnTheRow(t *testing.T) {
	high := attackRoll()
	high.Adv = room.AdvHigh
	low := attackRoll()
	low.Adv = room.AdvLow
	if markup := decoded(t, roomDiceRow(high)); !strings.Contains(markup, "Advantage") {
		t.Errorf("an advantage roll does not say so:\n%s", markup)
	}
	if markup := decoded(t, roomDiceRow(low)); !strings.Contains(markup, "Disadvantage") {
		t.Errorf("a disadvantage roll does not say so:\n%s", markup)
	}
	if markup := decoded(t, roomDiceRow(attackRoll())); strings.Contains(markup, "Advantage") {
		t.Errorf("a straight roll was labelled:\n%s", markup)
	}
}
func TestAnEmptyTraySaysSo(t *testing.T) {
	if markup := decoded(t, RoomDice(diceTray())); !strings.Contains(markup, "Nothing has been rolled yet") {
		t.Errorf("the empty tray says nothing:\n%s", markup)
	}
}
func TestTheModifierIsWrittenWithItsSignAndOmittedAtZero(t *testing.T) {
	for _, tc := range []struct {
		mod  int
		want string
	}{{7, "+ 7"}, {-2, "- 2"}, {0, ""}} {
		roll := attackRoll()
		roll.Mod = tc.mod
		if got := roll.ModText(); got != tc.want {
			t.Errorf("a modifier of %d reads %q, want %q", tc.mod, got, tc.want)
		}
	}
}
func TestTheGameMasterIsNamedAsTheGameMaster(t *testing.T) {
	if got := RollerName(true, "Kyle"); got != GameMasterName {
		t.Errorf("the GM is credited as %q, want %q", got, GameMasterName)
	}
	if got := RollerName(false, "Ilyana"); got != "Ilyana" {
		t.Errorf("a player is credited as %q, want Ilyana", got)
	}
}
func TestASavedRollRidesOnTheTraysAdvantageAndSecrecy(t *testing.T) {
	data := diceTray()
	data.Saved = []RoomSavedRoll{
		{ID: "01BX5ZZKBKACTAV9WEVGEMMVR0", Label: "Longsword", Expr: "1d8 + 4"},
		{ID: "01BX5ZZKBKACTAV9WEVGEMMVR1", Expr: "1d20"},
	}
	markup := decoded(t, RoomDice(data))
	form, end := strings.Index(markup, "<form"), strings.Index(markup, "</form>")
	list := strings.Index(markup, `id="`+RoomSavedRollsID+`"`)
	if form < 0 || list < form || end < list {
		t.Fatal("the saved rolls sit outside the form, so a click would not carry the advantage and secret settings")
	}
	first := strings.Index(markup, `data-saved-roll="01BX5ZZKBKACTAV9WEVGEMMVR0"`)
	second := strings.Index(markup, `data-saved-roll="01BX5ZZKBKACTAV9WEVGEMMVR1"`)
	if first < list || second < first {
		t.Error("the saved rolls are out of order")
	}
	for _, want := range []string{
		`hx-post="/rooms/` + testDiceRoomID + `/dice" hx-vals="{"expr":"1d8 + 4","label":"Longsword"}"`,
		`hx-post="/rooms/` + testDiceRoomID + `/dice" hx-vals="{"expr":"1d20","label":""}"`,
		`aria-label="Remove Longsword" hx-delete="/saved-rolls/01BX5ZZKBKACTAV9WEVGEMMVR0" hx-target="closest [data-saved-roll]" hx-swap="delete"`,
		`aria-label="Remove 1d20"`,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("the tray is missing %s:\n%s", want, markup)
		}
	}
	if strings.Count(markup, `type="button"`) != 5 {
		t.Error("a button in the tray would submit the form; only Roll may")
	}
}
func TestTheSaveButtonWaitsForDiceAndThenAsksForALabel(t *testing.T) {
	markup := decoded(t, RoomDice(diceTray()))
	roll, save := strings.Index(markup, ">Roll</button>"), strings.Index(markup, "data-save-roll=")
	if roll < 0 || save < roll || strings.Index(markup, "</div>") < save {
		t.Fatal("the save button does not sit beside Roll in the dice row")
	}
	button := markup[save : strings.Index(markup[save:], "</button>")+save]
	for _, want := range []string{
		`data-save-roll="/fragment/room/dice/save?room=` + testDiceRoomID + `"`,
		`aria-label="Save this roll"`,
		` disabled`,
		`hx-on:click="window.dispatchEvent(new CustomEvent('modal:open', { detail: { url: this.dataset.saveRoll + '&expr=' + encodeURIComponent(this.form.expr.value), size: 'sm' } }))"`,
	} {
		if !strings.Contains(button, want) {
			t.Errorf("the save button is missing %s:\n%s", want, button)
		}
	}
	if !strings.Contains(markup, `hx-on:input="this.form.querySelector('[data-save-roll]').disabled = !this.value.trim()"`) {
		t.Error("typing dice does not wake the save button")
	}
	if strings.Contains(markup, `name="label"`) {
		t.Error("the tray still asks for a label; the modal does that now")
	}
}
func TestTheSavedRollsRefetchThemselvesWhenOneIsSaved(t *testing.T) {
	markup := decoded(t, RoomDice(diceTray()))
	list := markup[strings.Index(markup, `id="`+RoomSavedRollsID+`"`):]
	list = list[:strings.Index(list, ">")]
	for _, want := range []string{
		`hx-get="/fragment/room/dice?room=` + testDiceRoomID + `"`,
		`hx-trigger="` + uievents.SavedRolls + ` from:window"`,
		`hx-select="#` + RoomSavedRollsID + `"`,
		`hx-swap="outerHTML"`,
		`class="hidden flex-wrap gap-1 has-[[data-saved-roll]]:flex"`,
	} {
		if !strings.Contains(list, want) {
			t.Errorf("the saved rolls are missing %s: %s", want, list)
		}
	}
	if strings.Contains(markup, "one click") {
		t.Error("the tray still explains saved rolls when there are none")
	}
}
func TestTheSaveModalClosesBeforeItSaves(t *testing.T) {
	markup := decoded(t, RoomSaveRoll(RoomSaveRollData{RoomID: testDiceRoomID, Expr: "1d8 + 4"}))
	for _, want := range []string{
		`hx-post="/rooms/` + testDiceRoomID + `/saved-rolls"`,
		`hx-target="#errors-` + SaveRollPanel + `"`,
		`hx-status:422="target:#errors-` + SaveRollPanel + `,swap:outerHTML"`,
		`<input type="hidden" name="expr" value="1d8 + 4">`,
		`name="label"`,
		`maxlength="64"`,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("the modal is missing %s:\n%s", want, markup)
		}
	}
	closeAt, saveAt := strings.Index(markup, ">Close</button>"), strings.Index(markup, ">Save roll</button>")
	if closeAt < 0 || saveAt < closeAt {
		t.Error("Close does not come before Save roll")
	}
	if !strings.Contains(markup, `class="btn border-base-content/50" hx-on:click="window.dispatchEvent(new CustomEvent('modal:close'))"`) {
		t.Error("Close is not the solid neutral button that dismisses the modal")
	}
	if strings.Contains(markup, "btn-ghost") || strings.Contains(markup, "btn-soft") {
		t.Error("a button in the modal is not solid")
	}
	refused := decoded(t, RoomSaveRoll(RoomSaveRollData{RoomID: testDiceRoomID, Expr: "2d6 * 3", Problems: []string{"The dice tray does not multiply or divide."}}))
	if !strings.Contains(refused, "multiply") || !strings.Contains(refused, `class="btn btn-primary" disabled`) {
		t.Errorf("a refused roll is offered for saving:\n%s", refused)
	}
}
func TestASavedRollsValuesSurviveAnyLabel(t *testing.T) {
	saved := RoomSavedRoll{ID: "01BX5ZZKBKACTAV9WEVGEMMVR0", Label: `Bob's "big" axe \ </script>`, Expr: "2d6 + 3"}
	var got map[string]string
	if err := json.Unmarshal([]byte(saved.RollVals()), &got); err != nil {
		t.Fatalf("hx-vals is not JSON: %v", err)
	}
	if got["label"] != saved.Label || got["expr"] != saved.Expr {
		t.Errorf("hx-vals carried %v", got)
	}
}
