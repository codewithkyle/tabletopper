package dnd

import (
	"errors"
	"strings"
	"unicode"
)

type Coin int

const (
	Copper Coin = iota
	Silver
	Electrum
	Gold
	Platinum
)

const CoinLimit = 9_999_999

var (
	ErrCoinFormat    = errors.New("coin change is not amounts and coins")
	ErrCoinLimit     = errors.New("coin count is over the limit")
	ErrNotEnoughCoin = errors.New("not enough coin")
)

var coinValues = [...]int64{1, 10, 50, 100, 1000}
var coinAbbrs = [...]string{"cp", "sp", "ep", "gp", "pp"}
var coinNames = [...]string{"Copper", "Silver", "Electrum", "Gold", "Platinum"}

func Coins() []Coin {
	return []Coin{Copper, Silver, Electrum, Gold, Platinum}
}
func (c Coin) Value() int64  { return coinValues[c] }
func (c Coin) Abbr() string  { return coinAbbrs[c] }
func (c Coin) Name() string  { return coinNames[c] }
func (c Coin) Field() string { return strings.ToLower(coinNames[c]) }

type Purse [len(coinValues)]int64

func (p Purse) Total() int64 {
	var total int64
	for _, coin := range Coins() {
		total += p[coin] * coin.Value()
	}
	return total
}
func (p Purse) Empty() bool {
	return p == Purse{}
}

type CoinChange struct {
	Coin   Coin
	Amount int64
}

func ParseCoinChanges(raw string) ([]CoinChange, error) {
	text := []rune(strings.ToLower(raw))
	var changes []CoinChange
	sign := int64(1)
	at := 0
	skip := func(gap func(rune) bool) {
		for at < len(text) && gap(text[at]) {
			at++
		}
	}
	for {
		skip(func(r rune) bool { return unicode.IsSpace(r) || r == ',' })
		if at == len(text) {
			break
		}
		switch text[at] {
		case '+':
			sign = 1
			at++
		case '-', '−':
			sign = -1
			at++
		}
		skip(unicode.IsSpace)
		start := at
		var amount int64
		for at < len(text) && text[at] >= '0' && text[at] <= '9' {
			amount = amount*10 + int64(text[at]-'0')
			if amount > CoinLimit {
				return nil, ErrCoinLimit
			}
			at++
		}
		if at == start {
			return nil, ErrCoinFormat
		}
		skip(unicode.IsSpace)
		start = at
		skip(unicode.IsLetter)
		coin, ok := coinNamed(string(text[start:at]))
		if !ok {
			return nil, ErrCoinFormat
		}
		changes = append(changes, CoinChange{Coin: coin, Amount: sign * amount})
	}
	if len(changes) == 0 {
		return nil, ErrCoinFormat
	}
	return changes, nil
}
func coinNamed(word string) (Coin, bool) {
	for _, coin := range Coins() {
		if word == coin.Abbr() || word == coin.Field() {
			return coin, true
		}
	}
	return 0, false
}
func (p Purse) Apply(changes []CoinChange) (Purse, error) {
	next := p
	for _, change := range changes {
		if change.Amount > 0 {
			next[change.Coin] += change.Amount
		}
	}
	for _, change := range changes {
		if change.Amount < 0 {
			spent, err := next.spend(change.Coin, -change.Amount)
			if err != nil {
				return p, err
			}
			next = spent
		}
	}
	for _, count := range next {
		if count > CoinLimit {
			return p, ErrCoinLimit
		}
	}
	return next, nil
}
func (p Purse) spend(coin Coin, amount int64) (Purse, error) {
	if p.Total() < amount*coin.Value() {
		return p, ErrNotEnoughCoin
	}
	paid := min(p[coin], amount)
	p[coin] -= paid
	owed := (amount - paid) * coin.Value()
	if owed == 0 {
		return p, nil
	}
	if p.below(coin) >= owed {
		return p.payDown(coin, owed), nil
	}
	for larger := coin + 1; larger <= Platinum && owed > 0; larger++ {
		broken := min(p[larger], (owed+larger.Value()-1)/larger.Value())
		p[larger] -= broken
		value := broken * larger.Value()
		if value >= owed {
			p = p.giveChange(coin, larger, value-owed)
			owed = 0
		} else {
			owed -= value
		}
	}
	return p.payDown(coin, owed), nil
}
func (p Purse) below(coin Coin) int64 {
	var total int64
	for smaller := Copper; smaller < coin; smaller++ {
		total += p[smaller] * smaller.Value()
	}
	return total
}
func (p Purse) payDown(coin Coin, owed int64) Purse {
	for smaller := coin - 1; smaller >= Copper && owed > 0; smaller-- {
		taken := min(p[smaller], owed/smaller.Value())
		p[smaller] -= taken
		owed -= taken * smaller.Value()
	}
	return p
}
func (p Purse) giveChange(coin, broken Coin, change int64) Purse {
	for smaller := broken - 1; smaller >= coin && change > 0; smaller-- {
		if smaller == Electrum && coin != Electrum {
			continue
		}
		p[smaller] += change / smaller.Value()
		change %= smaller.Value()
	}
	return p
}
