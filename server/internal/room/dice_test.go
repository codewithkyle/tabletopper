package room

import (
	"strings"
	"testing"
)

func diceEnv(values ...int) Env {
	i := 0
	return Env{Dice: func(sides int) int {
		if i >= len(values) {
			panic("the test rolled more dice than it supplied")
		}
		v := values[i]
		i++
		return v
	}}
}
func rolled(t *testing.T, expr string, adv int, values ...int) Result {
	t.Helper()
	r, err := rollDice(expr, adv, diceEnv(values...))
	if err != nil {
		t.Fatalf("%q: %v", expr, err)
	}
	return r
}
func refuseDice(t *testing.T, expr string, adv int) *Error {
	t.Helper()
	r, err := rollDice(expr, adv, diceEnv())
	if err == nil {
		t.Fatalf("%q: expected a refusal, got %+v", expr, r)
	}
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("%q: expected a *room.Error, got %T: %v", expr, err, err)
	}
	if e.Code != CodeInvalid {
		t.Fatalf("%q: expected %s, got %s (%s)", expr, CodeInvalid, e.Code, e.Message)
	}
	return e
}
func faces(r Result) []int {
	out := make([]int, 0, len(r.Dice))
	for _, d := range r.Dice {
		out = append(out, d.Value)
	}
	return out
}
func kept(r Result) []int {
	out := make([]int, 0, len(r.Dice))
	for _, d := range r.Dice {
		if d.Kept {
			out = append(out, d.Value)
		}
	}
	return out
}
func equalInts(t *testing.T, what string, got, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", what, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s = %v, want %v", what, got, want)
		}
	}
}

func TestAnExpressionRollsWhatItNames(t *testing.T) {
	t.Run("dice and a modifier are added up and kept apart", func(t *testing.T) {
		r := rolled(t, "1d20 + 5", AdvNone, 17)
		equalInts(t, "faces", faces(r), []int{17})
		if r.Mod != 5 || r.Total != 22 {
			t.Fatalf("mod = %d and total = %d, want 5 and 22", r.Mod, r.Total)
		}
	})
	t.Run("a die records its sides and that it counted", func(t *testing.T) {
		r := rolled(t, "1d8", AdvNone, 3)
		d := r.Dice[0]
		if d.Sides != 8 || d.Value != 3 || d.Sign != 1 || !d.Kept {
			t.Fatalf("die = %+v, want an 8 sided 3 that counted once", d)
		}
	})
	t.Run("spacing and capitals do not matter", func(t *testing.T) {
		r := rolled(t, "  2D6   +   1  ", AdvNone, 4, 5)
		if r.Total != 10 {
			t.Fatalf("total = %d, want 10", r.Total)
		}
	})
	t.Run("a die can be written without a count", func(t *testing.T) {
		r := rolled(t, "d20", AdvNone, 11)
		equalInts(t, "faces", faces(r), []int{11})
		if r.Total != 11 {
			t.Fatalf("total = %d, want 11", r.Total)
		}
	})
	t.Run("a modifier can stand on its own", func(t *testing.T) {
		r := rolled(t, "7", AdvNone)
		if len(r.Dice) != 0 || r.Mod != 7 || r.Total != 7 {
			t.Fatalf("result = %+v, want a bare 7", r)
		}
	})
	t.Run("modifiers accumulate apart from the dice", func(t *testing.T) {
		r := rolled(t, "2d6 + 3 - 1", AdvNone, 4, 4)
		if r.Mod != 2 || r.Total != 10 {
			t.Fatalf("mod = %d and total = %d, want 2 and 10", r.Mod, r.Total)
		}
	})
	t.Run("a subtracted die is marked and taken off the total", func(t *testing.T) {
		r := rolled(t, "2d6 - 1d4", AdvNone, 4, 5, 3)
		if r.Total != 6 {
			t.Fatalf("total = %d, want 6", r.Total)
		}
		if r.Dice[2].Sign != -1 {
			t.Fatalf("the subtracted die has sign %d, want -1", r.Dice[2].Sign)
		}
	})
	t.Run("an expression can open with a sign", func(t *testing.T) {
		r := rolled(t, "-1d4 + 10", AdvNone, 3)
		if r.Total != 7 {
			t.Fatalf("total = %d, want 7", r.Total)
		}
	})
}
func TestKeepingDiceDropsTheRestWithoutLosingThem(t *testing.T) {
	t.Run("keeping the highest drops the lowest", func(t *testing.T) {
		r := rolled(t, "4d6kh3", AdvNone, 5, 3, 2, 1)
		equalInts(t, "faces", faces(r), []int{5, 3, 2, 1})
		equalInts(t, "kept", kept(r), []int{5, 3, 2})
		if r.Total != 10 {
			t.Fatalf("total = %d, want 10", r.Total)
		}
	})
	t.Run("keeping the lowest drops the highest", func(t *testing.T) {
		r := rolled(t, "2d20kl1", AdvNone, 18, 4)
		equalInts(t, "kept", kept(r), []int{4})
		if r.Total != 4 {
			t.Fatalf("total = %d, want 4", r.Total)
		}
	})
	t.Run("a keep without a number keeps one", func(t *testing.T) {
		r := rolled(t, "2d20kh", AdvNone, 9, 15)
		equalInts(t, "kept", kept(r), []int{15})
	})
	t.Run("a tie is settled by the order they were rolled", func(t *testing.T) {
		r := rolled(t, "3d6kh1", AdvNone, 4, 4, 2)
		if !r.Dice[0].Kept || r.Dice[1].Kept || r.Dice[2].Kept {
			t.Fatalf("kept = %v, want only the first four", faces(r))
		}
	})
	t.Run("keeping every die changes nothing", func(t *testing.T) {
		r := rolled(t, "3d6kh3", AdvNone, 1, 2, 3)
		equalInts(t, "kept", kept(r), []int{1, 2, 3})
	})
}
func TestAdvantageAppliesToTheFirstBareTwentyOnly(t *testing.T) {
	t.Run("advantage rolls a second die and keeps the higher", func(t *testing.T) {
		r := rolled(t, "1d20 + 5", AdvHigh, 8, 19)
		equalInts(t, "faces", faces(r), []int{8, 19})
		equalInts(t, "kept", kept(r), []int{19})
		if r.Total != 24 {
			t.Fatalf("total = %d, want 24", r.Total)
		}
	})
	t.Run("disadvantage keeps the lower", func(t *testing.T) {
		r := rolled(t, "1d20 + 5", AdvLow, 8, 19)
		equalInts(t, "kept", kept(r), []int{8})
		if r.Total != 13 {
			t.Fatalf("total = %d, want 13", r.Total)
		}
	})
	t.Run("a roll with no twenty is left alone", func(t *testing.T) {
		r := rolled(t, "2d6", AdvHigh, 3, 4)
		equalInts(t, "faces", faces(r), []int{3, 4})
		if r.Total != 7 {
			t.Fatalf("total = %d, want 7", r.Total)
		}
	})
	t.Run("only the first twenty is doubled", func(t *testing.T) {
		r := rolled(t, "1d20 + 1d20", AdvHigh, 5, 18, 11)
		equalInts(t, "faces", faces(r), []int{5, 18, 11})
		equalInts(t, "kept", kept(r), []int{18, 11})
		if r.Total != 29 {
			t.Fatalf("total = %d, want 29", r.Total)
		}
	})
	t.Run("a term that already keeps is left alone", func(t *testing.T) {
		r := rolled(t, "2d20kh1", AdvLow, 6, 14)
		equalInts(t, "kept", kept(r), []int{14})
	})
	t.Run("a count other than one is left alone", func(t *testing.T) {
		r := rolled(t, "3d20", AdvHigh, 2, 3, 4)
		equalInts(t, "faces", faces(r), []int{2, 3, 4})
	})
	t.Run("anything but straight, advantage or disadvantage is refused", func(t *testing.T) {
		refuseDice(t, "1d20", 2)
		refuseDice(t, "1d20", -2)
	})
}
func TestTheFlourishReadsTheDiceAndNotTheTotal(t *testing.T) {
	t.Run("a kept natural twenty is a crit however small the total", func(t *testing.T) {
		r := rolled(t, "1d20 - 40", AdvNone, 20)
		if !r.Crit() || r.Fumble() {
			t.Fatalf("crit = %v and fumble = %v, want true and false", r.Crit(), r.Fumble())
		}
	})
	t.Run("a kept natural one is a fumble however large the total", func(t *testing.T) {
		r := rolled(t, "1d20 + 40", AdvNone, 1)
		if r.Crit() || !r.Fumble() {
			t.Fatalf("crit = %v and fumble = %v, want false and true", r.Crit(), r.Fumble())
		}
	})
	t.Run("a dropped twenty is neither", func(t *testing.T) {
		r := rolled(t, "2d20kl1", AdvNone, 20, 3)
		if r.Crit() || r.Fumble() {
			t.Fatalf("a dropped twenty flourished")
		}
	})
	t.Run("only a twenty sided die counts", func(t *testing.T) {
		r := rolled(t, "2d10", AdvNone, 10, 1)
		if r.Crit() || r.Fumble() {
			t.Fatalf("a d10 flourished")
		}
	})
	t.Run("a subtracted twenty is neither", func(t *testing.T) {
		r := rolled(t, "10 - 1d20", AdvNone, 20)
		if r.Crit() {
			t.Fatalf("a subtracted twenty crit")
		}
	})
	t.Run("one of each is both", func(t *testing.T) {
		r := rolled(t, "1d20 + 1d20", AdvNone, 1, 20)
		if !r.Crit() || !r.Fumble() {
			t.Fatalf("crit = %v and fumble = %v, want both", r.Crit(), r.Fumble())
		}
	})
}
func TestTheTrayRefusesWhatItDoesNotDo(t *testing.T) {
	t.Run("multiplication and division are named in the refusal", func(t *testing.T) {
		for _, expr := range []string{"2d6 * 3", "2d6 / 3"} {
			e := refuseDice(t, expr, AdvNone)
			if !strings.Contains(e.Message, "multiply") {
				t.Fatalf("%q said %q, want it to name multiplying", expr, e.Message)
			}
		}
	})
	t.Run("nothing to roll is refused", func(t *testing.T) {
		refuseDice(t, "", AdvNone)
		refuseDice(t, "   ", AdvNone)
	})
	t.Run("half an expression is refused", func(t *testing.T) {
		for _, expr := range []string{"1d", "1d20+", "+", "-", "1d20k", "abc", "d20abc", "1d20kh1kh1", "()", "1d20+(2)"} {
			refuseDice(t, expr, AdvNone)
		}
	})
	t.Run("a die count stops at its limits", func(t *testing.T) {
		rolled(t, "100d2", AdvNone, make([]int, 100)...)
		refuseDice(t, "0d6", AdvNone)
		refuseDice(t, "101d6", AdvNone)
	})
	t.Run("a die has a sensible number of sides", func(t *testing.T) {
		refuseDice(t, "1d1", AdvNone)
		refuseDice(t, "1d0", AdvNone)
		refuseDice(t, "1d1001", AdvNone)
	})
	t.Run("keeping is bounded by what was rolled", func(t *testing.T) {
		refuseDice(t, "4d6kh5", AdvNone)
		refuseDice(t, "4d6kh0", AdvNone)
	})
	t.Run("a modifier stops at its limit", func(t *testing.T) {
		rolled(t, "9999", AdvNone)
		refuseDice(t, "10000", AdvNone)
	})
	t.Run("an expression stops at its length and its parts", func(t *testing.T) {
		refuseDice(t, strings.Repeat("1", DiceExprLimit+1), AdvNone)
		refuseDice(t, strings.TrimSuffix(strings.Repeat("1+", DiceTermsMax+1), "+"), AdvNone)
	})
}
func TestTheDefaultSourceRollsInRange(t *testing.T) {
	for range 200 {
		r, err := rollDice("4d6kh3 + 1d20", AdvNone, Env{})
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range r.Dice {
			if d.Value < 1 || d.Value > d.Sides {
				t.Fatalf("a d%d rolled %d", d.Sides, d.Value)
			}
		}
	}
}

func TestARollIsRememberedForTheWholeTable(t *testing.T) {
	t.Run("it lands in the log with the character's name", func(t *testing.T) {
		w := newWorld(t)
		w.apply(&DiceRoll{Expr: "1D20 +  7", Label: "  Longsword  "}, w.pc)
		if len(w.s.Rolls) != 1 {
			t.Fatalf("the log holds %d rolls, want 1", len(w.s.Rolls))
		}
		r := w.s.Rolls[0]
		if r.Name != "Ilyana" || r.GM {
			t.Fatalf("the roll is credited to %q with gm = %v, want Ilyana and false", r.Name, r.GM)
		}
		if r.By != testPlayerID {
			t.Fatalf("the roll is by %s, want %s", r.By, testPlayerID)
		}
		if r.Label != "Longsword" {
			t.Fatalf("label = %q, want it trimmed to Longsword", r.Label)
		}
		if r.Expr != "1d20 + 7" {
			t.Fatalf("expr = %q, want the spacing and capitals tidied", r.Expr)
		}
		if len(r.Dice) != 1 || r.Mod != 7 {
			t.Fatalf("result = %+v, want one die and a modifier of 7", r.Result)
		}
	})
	t.Run("a player without a character is credited by name", func(t *testing.T) {
		w := newWorld(t)
		w.apply(&PlayerJoin{Player: Player{ID: testID(1100), Name: "Wren", Role: RolePlayer}}, w.gm)
		w.apply(&DiceRoll{Expr: "1d20"}, Actor{ID: testID(1100), Role: RolePlayer})
		if got := w.s.Rolls[0].Name; got != "Wren" {
			t.Fatalf("the roll is credited to %q, want Wren", got)
		}
	})
	t.Run("the GM is marked so the log can say so", func(t *testing.T) {
		w := newWorld(t)
		w.apply(&DiceRoll{Expr: "1d20"}, w.gm)
		if r := w.s.Rolls[0]; !r.GM || r.Name != "Kyle" {
			t.Fatalf("the GM's roll is %q with gm = %v, want Kyle and true", r.Name, r.GM)
		}
	})
	t.Run("advantage is recorded so the row can show it", func(t *testing.T) {
		w := newWorld(t)
		w.apply(&DiceRoll{Expr: "1d20", Adv: AdvHigh}, w.pc)
		r := w.s.Rolls[0]
		if r.Adv != AdvHigh || len(r.Dice) != 2 {
			t.Fatalf("adv = %d over %d dice, want %d over 2", r.Adv, len(r.Dice), AdvHigh)
		}
	})
	t.Run("everybody at the table sees it", func(t *testing.T) {
		w := newWorld(t)
		ch := w.change(&DiceRoll{Expr: "1d20"}, w.pc)
		for _, role := range []Role{RoleGM, RolePlayer} {
			equalStrings(t, string(role), changeTypesOf(ch.changes(role)), []string{"rolls.upserted"})
		}
		if len(w.s.Project(RolePlayer).Rolls) != 1 {
			t.Fatalf("a player's projection dropped the roll")
		}
	})
	t.Run("a refusal leaves the log alone", func(t *testing.T) {
		w := newWorld(t)
		w.refuse(&DiceRoll{Expr: "2d6 * 3"}, w.pc, CodeInvalid)
		w.refuse(&DiceRoll{Expr: "1d20", Label: strings.Repeat("a", DiceLabelLimit+1)}, w.pc, CodeInvalid)
		if len(w.s.Rolls) != 0 {
			t.Fatalf("the log holds %d rolls after two refusals, want 0", len(w.s.Rolls))
		}
	})
	t.Run("a label of exactly the limit is accepted", func(t *testing.T) {
		w := newWorld(t)
		w.apply(&DiceRoll{Expr: "1d20", Label: strings.Repeat("a", DiceLabelLimit)}, w.pc)
	})
}
func TestTheLogKeepsOnlyTheLastRolls(t *testing.T) {
	w := newWorld(t)
	for range RollsMax {
		w.apply(&DiceRoll{Expr: "1d6"}, w.pc)
	}
	oldest := w.s.Rolls[0].ID
	ch := w.change(&DiceRoll{Expr: "1d6"}, w.pc)
	if len(w.s.Rolls) != RollsMax {
		t.Fatalf("the log holds %d rolls, want %d", len(w.s.Rolls), RollsMax)
	}
	if w.s.Rolls[0].ID == oldest {
		t.Fatalf("the oldest roll was not evicted")
	}
	equalStrings(t, "changes", changeTypesOf(ch.changes(RolePlayer)), []string{"rolls.removed", "rolls.upserted"})
}
func TestASnapshotWrittenBeforeDiceStillLoads(t *testing.T) {
	blob := []byte(`{"schema":3,"seq":4,"room":{"id":"010000000000000000000000Z8","name":"Old","locked":false},"players":[],"pawns":[],"fog":[],"strokes":[]}`)
	s, err := Unmarshal(blob)
	if err != nil {
		t.Fatalf("a snapshot from before the dice tray no longer loads: %v", err)
	}
	if s.Rolls == nil || len(s.Rolls) != 0 {
		t.Fatalf("rolls = %v, want an empty log", s.Rolls)
	}
}

func TestASecretRollNeverTouchesTheRoom(t *testing.T) {
	t.Run("the shared log stays empty and nothing is derived", func(t *testing.T) {
		w := newWorld(t)
		ch := w.change(&DiceRoll{Expr: "1d20 + 7", Secret: true}, w.pc)
		if len(w.s.Rolls) != 0 {
			t.Fatalf("a secret roll put %d rolls in the shared log", len(w.s.Rolls))
		}
		for _, role := range []Role{RoleGM, RolePlayer} {
			equalStrings(t, string(role), changeTypesOf(ch.changes(role)), nil)
		}
	})
	t.Run("only the player who rolled is told", func(t *testing.T) {
		w := newWorld(t)
		ch := w.change(&DiceRoll{Expr: "1d20", Secret: true}, w.pc)
		equalStrings(t, "the roller", eventTypesOf(ch.sent(w.pc)), []string{"rolled"})
		equalStrings(t, "the GM", eventTypesOf(ch.sent(w.gm)), nil)
		equalStrings(t, "another player", eventTypesOf(ch.sent(w.other)), nil)
	})
	t.Run("the GM rolls behind the screen on the same terms", func(t *testing.T) {
		w := newWorld(t)
		ch := w.change(&DiceRoll{Expr: "1d20", Secret: true}, w.gm)
		equalStrings(t, "the GM", eventTypesOf(ch.sent(w.gm)), []string{"rolled"})
		equalStrings(t, "a player", eventTypesOf(ch.sent(w.pc)), nil)
		equalStrings(t, "another player", eventTypesOf(ch.sent(w.other)), nil)
	})
	t.Run("the event carries the whole roll to its roller", func(t *testing.T) {
		w := newWorld(t)
		sigs := w.apply(&DiceRoll{Expr: "1d20 + 7", Label: "Stealth", Secret: true}, w.pc)
		if len(sigs) != 1 || sigs[0].To != ToSender {
			t.Fatalf("signals = %v, want one to the sender", summary(sigs))
		}
		ev, ok := sigs[0].Event.(*Rolled)
		if !ok {
			t.Fatalf("the signal carries %T, want a *Rolled", sigs[0].Event)
		}
		if !ev.Roll.Secret || ev.Roll.Name != "Ilyana" || ev.Roll.Label != "Stealth" {
			t.Fatalf("the roll is %+v, want a secret Stealth roll by Ilyana", ev.Roll)
		}
	})
	t.Run("nothing in the shared log is ever marked secret", func(t *testing.T) {
		w := newWorld(t)
		w.apply(&DiceRoll{Expr: "1d20"}, w.pc)
		w.apply(&DiceRoll{Expr: "1d20", Secret: true}, w.pc)
		w.apply(&DiceRoll{Expr: "1d20", Secret: true}, w.gm)
		w.apply(&DiceRoll{Expr: "1d20"}, w.gm)
		if len(w.s.Rolls) != 2 {
			t.Fatalf("the shared log holds %d rolls, want the 2 that were not secret", len(w.s.Rolls))
		}
		for _, r := range w.s.Rolls {
			if r.Secret {
				t.Fatalf("a secret roll reached the shared log: %+v", r)
			}
		}
	})
	t.Run("a secret roll is refused on the same terms as any other", func(t *testing.T) {
		w := newWorld(t)
		w.refuse(&DiceRoll{Expr: "2d6 * 3", Secret: true}, w.pc, CodeInvalid)
		w.refuse(&DiceRoll{Expr: "1d20", Secret: true, Label: strings.Repeat("a", DiceLabelLimit+1)}, w.gm, CodeInvalid)
	})
}
