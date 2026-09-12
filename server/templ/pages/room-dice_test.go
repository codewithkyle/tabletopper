package pages

import (
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
