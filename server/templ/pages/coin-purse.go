package pages

import (
	"strconv"
	"strings"

	"tabletopper/internal/dnd"
)

const CoinLimit = dnd.CoinLimit

type CoinPurse struct {
	CharacterID string
	Coins       []CoinCount
	Total       string
	Change      string
	Problem     string
}
type CoinCount struct {
	Field string
	Abbr  string
	Name  string
	Count string
}

func NewCoinPurse(characterID string, purse dnd.Purse) CoinPurse {
	return CoinPurse{
		CharacterID: characterID,
		Coins:       purseCoins(purse, false),
		Total:       CoinTotalLabel(purse),
	}
}
func HeldCoins(purse dnd.Purse) []CoinCount {
	return purseCoins(purse, true)
}
func purseCoins(purse dnd.Purse, held bool) []CoinCount {
	counts := make([]CoinCount, 0, len(purse))
	for _, coin := range dnd.Coins() {
		if held && purse[coin] == 0 {
			continue
		}
		counts = append(counts, CoinCount{
			Field: coin.Field(),
			Abbr:  coin.Abbr(),
			Name:  coin.Name(),
			Count: strconv.FormatInt(purse[coin], 10),
		})
	}
	return counts
}
func CoinTotalLabel(purse dnd.Purse) string {
	total := purse.Total()
	gold := strconv.FormatInt(total/dnd.Gold.Value(), 10)
	if cents := total % dnd.Gold.Value(); cents != 0 {
		gold += "." + strings.TrimRight(strconv.FormatInt(100+cents, 10)[1:], "0")
	}
	return gold + " gp"
}
func CoinPanel(characterID string) string {
	return "coins-" + characterID
}
func coinsPath(characterID string) string {
	return "/characters/" + characterID + "/coins"
}
func coinChangeFormID(characterID string) string {
	return "coin-change-" + characterID
}
func coinChangeInputID(characterID string) string {
	return "coin-change-input-" + characterID
}
func coinChangeNoteID(characterID string) string {
	return "coin-change-note-" + characterID
}
func coinCountsID(characterID string) string {
	return "coin-counts-" + characterID
}
func coinTotalID(characterID string) string {
	return "coin-total-" + characterID
}
func coinInputID(characterID string, field string) string {
	return "coin-" + field + "-" + characterID
}
