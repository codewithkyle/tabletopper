package dnd

import (
	"errors"
	"math/rand/v2"
	"reflect"
	"testing"
)

func purse(cp, sp, ep, gp, pp int64) Purse {
	return Purse{cp, sp, ep, gp, pp}
}
func TestParseCoinChangesReadsWhatPlayersType(t *testing.T) {
	for _, c := range []struct {
		raw  string
		want []CoinChange
	}{
		{"-25sp", []CoinChange{{Silver, -25}}},
		{"+25sp", []CoinChange{{Silver, 25}}},
		{"25sp", []CoinChange{{Silver, 25}}},
		{"  - 25 SP ", []CoinChange{{Silver, -25}}},
		{"−3gp", []CoinChange{{Gold, -3}}},
		{"+12gp 30sp 4pp", []CoinChange{{Gold, 12}, {Silver, 30}, {Platinum, 4}}},
		{"-1gp, 5sp", []CoinChange{{Gold, -1}, {Silver, -5}}},
		{"+3gp -2sp 1cp", []CoinChange{{Gold, 3}, {Silver, -2}, {Copper, -1}}},
		{"5gp3sp", []CoinChange{{Gold, 5}, {Silver, 3}}},
		{"10 gold -1 electrum", []CoinChange{{Gold, 10}, {Electrum, -1}}},
		{"+0gp", []CoinChange{{Gold, 0}}},
	} {
		got, err := ParseCoinChanges(c.raw)
		if err != nil {
			t.Errorf("ParseCoinChanges(%q) failed: %v", c.raw, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParseCoinChanges(%q) = %v, want %v", c.raw, got, c.want)
		}
	}
}
func TestParseCoinChangesRefusesWhatItCannotRead(t *testing.T) {
	for _, c := range []struct {
		raw  string
		want error
	}{
		{"", ErrCoinFormat},
		{"   ", ErrCoinFormat},
		{"-25", ErrCoinFormat},
		{"gp", ErrCoinFormat},
		{"25 doubloons", ErrCoinFormat},
		{"1,000gp", ErrCoinFormat},
		{"1.5gp", ErrCoinFormat},
		{"+5gp -", ErrCoinFormat},
		{"10000000gp", ErrCoinLimit},
		{"99999999999999999999999gp", ErrCoinLimit},
	} {
		if _, err := ParseCoinChanges(c.raw); !errors.Is(err, c.want) {
			t.Errorf("ParseCoinChanges(%q) error = %v, want %v", c.raw, err, c.want)
		}
	}
}
func TestSpendingMakesChangeFromTheSmallestCoinThatCovers(t *testing.T) {
	for _, c := range []struct {
		name  string
		start Purse
		spend string
		want  Purse
	}{
		{"pays in kind", purse(0, 30, 0, 0, 0), "-25sp", purse(0, 5, 0, 0, 0)},
		{"breaks gold for silver", purse(0, 0, 0, 100, 0), "-25sp", purse(0, 5, 0, 97, 0)},
		{"uses the silver it has first", purse(0, 10, 0, 100, 0), "-25sp", purse(0, 5, 0, 98, 0)},
		{"spends small coins that cover it", purse(250, 0, 0, 100, 0), "-25sp", purse(0, 0, 0, 100, 0)},
		{"keeps small coins that cannot", purse(200, 0, 0, 100, 0), "-25sp", purse(200, 5, 0, 97, 0)},
		{"breaks platinum into gold and silver", purse(0, 0, 0, 0, 1), "-5sp", purse(0, 5, 0, 9, 0)},
		{"never hands out electrum as change", purse(0, 0, 0, 1, 0), "-1cp", purse(9, 9, 0, 0, 0)},
		{"gives electrum back when paying in it", purse(0, 0, 0, 0, 1), "-1ep", purse(0, 0, 1, 9, 0)},
		{"spends electrum before gold", purse(0, 0, 1, 10, 0), "-3sp", purse(0, 2, 0, 10, 0)},
		{"empties larger coins before reaching for smaller", purse(200, 0, 2, 0, 0), "-30sp", purse(0, 0, 0, 0, 0)},
		{"loot pays for what follows it", purse(0, 0, 0, 0, 0), "-5gp +10gp", purse(0, 0, 0, 5, 0)},
		{"spends each term", purse(0, 0, 0, 2, 0), "-1gp 5sp", purse(0, 5, 0, 0, 0)},
	} {
		changes, err := ParseCoinChanges(c.spend)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		got, err := c.start.Apply(changes)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: %v %s = %v, want %v", c.name, c.start, c.spend, got, c.want)
		}
	}
}
func TestSpendingMoreThanThePurseHoldsChangesNothing(t *testing.T) {
	start := purse(5, 5, 0, 1, 0)
	changes, _ := ParseCoinChanges("+1gp -3gp")
	got, err := start.Apply(changes)
	if !errors.Is(err, ErrNotEnoughCoin) {
		t.Fatalf("error = %v, want %v", err, ErrNotEnoughCoin)
	}
	if got != start {
		t.Errorf("purse = %v, want it untouched at %v", got, start)
	}
}
func TestAddingPastTheLimitIsRefused(t *testing.T) {
	start := purse(0, 0, 0, CoinLimit, 0)
	changes, _ := ParseCoinChanges("+1gp")
	if _, err := start.Apply(changes); !errors.Is(err, ErrCoinLimit) {
		t.Errorf("error = %v, want %v", err, ErrCoinLimit)
	}
}
func TestSpendingConservesValueAndNeverGoesNegative(t *testing.T) {
	random := rand.New(rand.NewPCG(1, 2))
	for range 20000 {
		var start Purse
		for coin := range start {
			start[coin] = random.Int64N(40)
		}
		coin := Coin(random.IntN(len(start)))
		amount := random.Int64N(60) + 1
		got, err := start.Apply([]CoinChange{{coin, -amount}})
		cost := amount * coin.Value()
		if start.Total() < cost {
			if !errors.Is(err, ErrNotEnoughCoin) {
				t.Fatalf("%v -%d%s: error = %v, want %v", start, amount, coin.Abbr(), err, ErrNotEnoughCoin)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%v -%d%s: %v", start, amount, coin.Abbr(), err)
		}
		if got.Total() != start.Total()-cost {
			t.Fatalf("%v -%d%s = %v, worth %d cp, want %d", start, amount, coin.Abbr(), got, got.Total(), start.Total()-cost)
		}
		for c, count := range got {
			if count < 0 {
				t.Fatalf("%v -%d%s = %v, %s went negative", start, amount, coin.Abbr(), got, Coin(c).Abbr())
			}
		}
	}
}
