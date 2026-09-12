package room

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/oklog/ulid/v2"
)

type Die struct {
	Sides int  `json:"sides"`
	Value int  `json:"value"`
	Sign  int  `json:"sign"`
	Kept  bool `json:"kept"`
}
type Result struct {
	Dice  []Die `json:"dice"`
	Mod   int   `json:"mod"`
	Total int   `json:"total"`
}

const (
	AdvNone = 0
	AdvHigh = 1
	AdvLow  = -1
)

const diceDigitsMax = 5

func (r Result) Crit() bool   { return r.natural(20) }
func (r Result) Fumble() bool { return r.natural(1) }
func (r Result) natural(value int) bool {
	for _, d := range r.Dice {
		if d.Sides == 20 && d.Kept && d.Sign > 0 && d.Value == value {
			return true
		}
	}
	return false
}

type diceTerm struct {
	sign  int
	dice  bool
	count int
	sides int
	keep  int
	high  bool
	flat  int
}

func (e Env) roll(sides int) int {
	if e.Dice != nil {
		return e.Dice(sides)
	}
	return rand.IntN(sides) + 1
}
func rollDice(expr string, adv int, env Env) (Result, error) {
	terms, err := parseDice(expr)
	if err != nil {
		return Result{}, err
	}
	if err := applyAdvantage(terms, adv); err != nil {
		return Result{}, err
	}
	return evaluate(terms, env), nil
}
func badDice() error {
	return invalid("Bad dice", "Write a roll like 1d20 + 5, 2d6 or 4d6kh3.")
}
func parseDice(expr string) ([]diceTerm, error) {
	if utf8.RuneCountInString(expr) > DiceExprLimit {
		return nil, invalid("Dice too long", fmt.Sprintf("A dice roll can be at most %d characters.", DiceExprLimit))
	}
	s := strings.ToLower(strings.Join(strings.Fields(expr), ""))
	if s == "" {
		return nil, invalid("No dice", "Type something to roll, like 1d20 + 5.")
	}
	if strings.ContainsAny(s, "*/") {
		return nil, invalid("Bad dice", "The dice tray does not multiply or divide.")
	}
	sign, i := 1, 0
	if s[0] == '+' || s[0] == '-' {
		if s[0] == '-' {
			sign = -1
		}
		i++
	}
	var terms []diceTerm
	for {
		t, next, err := parseTerm(s, i, sign)
		if err != nil {
			return nil, err
		}
		terms = append(terms, t)
		if len(terms) > DiceTermsMax {
			return nil, invalid("Too many parts", fmt.Sprintf("A dice roll holds at most %d parts.", DiceTermsMax))
		}
		if next == len(s) {
			return terms, nil
		}
		switch s[next] {
		case '+':
			sign = 1
		case '-':
			sign = -1
		default:
			return nil, badDice()
		}
		i = next + 1
		if i == len(s) {
			return nil, badDice()
		}
	}
}
func parseTerm(s string, i, sign int) (diceTerm, int, error) {
	t := diceTerm{sign: sign, count: 1}
	count, next, counted := parseDiceNumber(s, i)
	if counted {
		t.count, i = count, next
	}
	if i == len(s) || s[i] != 'd' {
		if !counted {
			return t, 0, badDice()
		}
		if count > DiceModLimit {
			return t, 0, invalid("Bad dice", fmt.Sprintf("A modifier can be at most %d.", DiceModLimit))
		}
		t.flat = count
		return t, i, nil
	}
	t.dice = true
	i++
	sides, next, ok := parseDiceNumber(s, i)
	if !ok {
		return t, 0, invalid("Bad dice", "A die needs a number of sides, like 1d20.")
	}
	t.sides, i = sides, next
	if i+1 < len(s) && s[i] == 'k' && (s[i+1] == 'h' || s[i+1] == 'l') {
		t.high = s[i+1] == 'h'
		t.keep = 1
		i += 2
		if keep, next, ok := parseDiceNumber(s, i); ok {
			if keep == 0 {
				return t, 0, invalid("Bad dice", "Keeping no dice leaves nothing to add up.")
			}
			t.keep, i = keep, next
		}
	}
	if err := checkDiceTerm(t); err != nil {
		return t, 0, err
	}
	return t, i, nil
}
func parseDiceNumber(s string, i int) (int, int, bool) {
	j := i
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	if j == i || j-i > diceDigitsMax {
		return 0, i, false
	}
	n, err := strconv.Atoi(s[i:j])
	if err != nil {
		return 0, i, false
	}
	return n, j, true
}
func checkDiceTerm(t diceTerm) error {
	if t.count < 1 || t.count > DiceCountMax {
		return invalid("Bad dice", fmt.Sprintf("Roll between 1 and %d dice at a time.", DiceCountMax))
	}
	if t.sides < DieSidesMin || t.sides > DieSidesMax {
		return invalid("Bad dice", fmt.Sprintf("A die has between %d and %d sides.", DieSidesMin, DieSidesMax))
	}
	if t.keep > t.count {
		return invalid("Bad dice", "You cannot keep more dice than you roll.")
	}
	return nil
}
func applyAdvantage(terms []diceTerm, adv int) error {
	switch adv {
	case AdvNone:
		return nil
	case AdvHigh, AdvLow:
	default:
		return invalid("Bad dice", "A roll is made straight, with advantage or with disadvantage.")
	}
	for i := range terms {
		t := &terms[i]
		if !t.dice || t.sides != 20 || t.count != 1 || t.keep != 0 {
			continue
		}
		t.count, t.keep, t.high = 2, 1, adv == AdvHigh
		return nil
	}
	return nil
}
func evaluate(terms []diceTerm, env Env) Result {
	var out Result
	for _, t := range terms {
		if !t.dice {
			out.Mod += t.sign * t.flat
			out.Total += t.sign * t.flat
			continue
		}
		start := len(out.Dice)
		for range t.count {
			out.Dice = append(out.Dice, Die{
				Sides: t.sides,
				Value: env.roll(t.sides),
				Sign:  t.sign,
				Kept:  true,
			})
		}
		rolled := out.Dice[start:]
		keepDice(rolled, t)
		for _, d := range rolled {
			if d.Kept {
				out.Total += t.sign * d.Value
			}
		}
	}
	return out
}
func keepDice(dice []Die, t diceTerm) {
	if t.keep == 0 || t.keep >= len(dice) {
		return
	}
	order := make([]int, len(dice))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		if t.high {
			return dice[b].Value - dice[a].Value
		}
		return dice[a].Value - dice[b].Value
	})
	for _, i := range order[t.keep:] {
		dice[i].Kept = false
	}
}

type Roll struct {
	ID     ulid.ULID `json:"id"`
	By     ulid.ULID `json:"by"`
	Name   string    `json:"name"`
	GM     bool      `json:"gm"`
	Label  string    `json:"label"`
	Expr   string    `json:"expr"`
	Adv    int       `json:"adv"`
	Secret bool      `json:"secret"`
	Result
}
type Rolled struct {
	Header
	Roll Roll `json:"roll"`
}

func (*Rolled) eventType() string { return "rolled" }
func (*Rolled) transient()        {}

type RollsUpserted struct {
	Kind
	Rolls []Roll `json:"rolls"`
}

func (*RollsUpserted) changeType() string { return "rolls.upserted" }

type RollsRemoved struct {
	Kind
	IDs []ulid.ULID `json:"ids"`
}

func (*RollsRemoved) changeType() string { return "rolls.removed" }

type DiceRoll struct {
	Expr   string `json:"expr"`
	Label  string `json:"label"`
	Adv    int    `json:"adv"`
	Secret bool   `json:"secret"`
}

func (c *DiceRoll) Authorize(s *State, a Actor) error { return nil }
func (c *DiceRoll) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	roll, err := s.newRoll(c, a, env)
	if err != nil {
		return nil, err
	}
	if c.Secret {
		return []Signal{signal(ToSender, &Rolled{Roll: roll})}, nil
	}
	s.Rolls = append(s.Rolls, roll)
	if len(s.Rolls) > RollsMax {
		s.Rolls = slices.Delete(s.Rolls, 0, len(s.Rolls)-RollsMax)
	}
	return nil, nil
}
func (s *State) newRoll(c *DiceRoll, a Actor, env Env) (Roll, error) {
	label := strings.TrimSpace(c.Label)
	if err := checkDiceLabel(label); err != nil {
		return Roll{}, err
	}
	result, err := rollDice(c.Expr, c.Adv, env)
	if err != nil {
		return Roll{}, err
	}
	name, gm := s.roller(a)
	return Roll{
		ID:     env.id(),
		By:     a.ID,
		Name:   name,
		GM:     gm,
		Label:  label,
		Expr:   tidyExpr(c.Expr),
		Adv:    c.Adv,
		Secret: c.Secret,
		Result: result,
	}, nil
}
func (s *State) roller(a Actor) (string, bool) {
	p := s.Player(a.ID)
	if p == nil {
		return "", a.GM()
	}
	if p.CharacterName != "" {
		return p.CharacterName, p.Role == RoleGM
	}
	return p.Name, p.Role == RoleGM
}
func tidyExpr(expr string) string {
	return strings.ToLower(strings.Join(strings.Fields(expr), " "))
}
func rollWithBonus(bonus int, env Env) Result {
	terms := []diceTerm{{sign: 1, dice: true, count: 1, sides: 20}}
	if bonus != 0 {
		sign := 1
		if bonus < 0 {
			sign, bonus = -1, -bonus
		}
		terms = append(terms, diceTerm{sign: sign, flat: bonus})
	}
	return evaluate(terms, env)
}
func cloneRoll(r Roll) Roll {
	r.Dice = cloneSlice(r.Dice)
	return r
}
func MergeRolls(shared, mine []Roll) []Roll {
	out := make([]Roll, 0, len(shared)+len(mine))
	for _, r := range shared {
		out = append(out, cloneRoll(r))
	}
	for _, r := range mine {
		out = append(out, cloneRoll(r))
	}
	slices.SortFunc(out, func(a, b Roll) int { return a.ID.Compare(b.ID) })
	return out
}
