package controllers

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"tabletopper/internal/dnd"
	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

func (a *App) SaveCharacterCoins(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	panel := pages.CoinPanel(characterID.String())
	if !parsePanelForm(w, r, panel) {
		return
	}
	var purse dnd.Purse
	for _, coin := range dnd.Coins() {
		purse[coin] = int64(clamped(r.PostFormValue(coin.Field()), uint32(0), 0, dnd.CoinLimit))
	}
	result, err := a.Queries.UpdateCharacterCoins(ctx, coinParams(purse, characterID, sess.UserID))
	if !finishRow(w, r, panel, "Coins", "character", result, err) {
		return
	}
	render(w, r, pages.CoinTotal(pages.NewCoinPurse(characterID.String(), purse), true))
}
func (a *App) ChangeCharacterCoins(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	refused := pages.CoinPurse{CharacterID: characterID.String()}
	if err := r.ParseForm(); err != nil {
		refused.Problem = "The submitted form data could not be read."
		renderStatus(w, r, http.StatusUnprocessableEntity, pages.CoinChangeForm(refused))
		return
	}
	refused.Change = strings.TrimSpace(r.PostFormValue("change"))
	changes, err := dnd.ParseCoinChanges(refused.Change)
	if err != nil {
		refused.Problem = coinProblem(err, dnd.Purse{})
		renderStatus(w, r, http.StatusUnprocessableEntity, pages.CoinChangeForm(refused))
		return
	}
	var held, next dnd.Purse
	err = a.tx(ctx, func(q *queries.Queries) error {
		row, err := q.LockCharacterCoins(ctx, queries.LockCharacterCoinsParams{ID: characterID, OwnerID: sess.UserID})
		if err != nil {
			return err
		}
		held = coinPurse(row.Copper, row.Silver, row.Electrum, row.Gold, row.Platinum)
		next, err = held.Apply(changes)
		if err != nil {
			return err
		}
		_, err = q.UpdateCharacterCoins(ctx, coinParams(next, characterID, sess.UserID))
		return err
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		htmx.NotFound(w, "character")
		return
	case errors.Is(err, dnd.ErrNotEnoughCoin), errors.Is(err, dnd.ErrCoinLimit):
		refused.Problem = coinProblem(err, held)
		renderStatus(w, r, http.StatusUnprocessableEntity, pages.CoinChangeForm(refused))
		return
	case err != nil:
		slog.Error("Failed to change coins", "error", err)
		htmx.ServerError(w)
		return
	}
	htmx.Toast(w, "Coins saved.")
	purse := pages.NewCoinPurse(characterID.String(), next)
	renderFragment(w, r, pages.CoinChangeForm(purse))
	render(w, r, pages.CoinCounts(purse, true))
	render(w, r, pages.CoinTotal(purse, true))
}
func coinProblem(err error, held dnd.Purse) string {
	switch {
	case errors.Is(err, dnd.ErrNotEnoughCoin):
		return "There is not enough coin for that. The purse holds " + pages.CoinTotalLabel(held) + " in all."
	case errors.Is(err, dnd.ErrCoinLimit):
		return "A purse holds at most " + strconv.Itoa(dnd.CoinLimit) + " of each coin."
	}
	return "Write each amount with its coin, like +10gp or -25sp."
}
func coinPurse(copper, silver, electrum, gold, platinum uint32) dnd.Purse {
	var purse dnd.Purse
	purse[dnd.Copper] = int64(copper)
	purse[dnd.Silver] = int64(silver)
	purse[dnd.Electrum] = int64(electrum)
	purse[dnd.Gold] = int64(gold)
	purse[dnd.Platinum] = int64(platinum)
	return purse
}
func characterPurse(character queries.Character) dnd.Purse {
	return coinPurse(character.Copper, character.Silver, character.Electrum, character.Gold, character.Platinum)
}
func coinParams(purse dnd.Purse, characterID, ownerID ulid.ULID) queries.UpdateCharacterCoinsParams {
	return queries.UpdateCharacterCoinsParams{
		Copper:   uint32(purse[dnd.Copper]),
		Silver:   uint32(purse[dnd.Silver]),
		Electrum: uint32(purse[dnd.Electrum]),
		Gold:     uint32(purse[dnd.Gold]),
		Platinum: uint32(purse[dnd.Platinum]),
		ID:       characterID,
		OwnerID:  ownerID,
	}
}
