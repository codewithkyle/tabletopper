package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"tabletopper/internal/hub"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

func (a *App) RoomDiceFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, _, err := a.roomMember(ctx, sess, r.URL.Query().Get("room"))
	if err != nil || a.Hub == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	view, ok := a.Hub.Rolls(ctx, row.ID, sess.UserID)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomDice(diceData(row.ID, view)))
}
func (a *App) RollDice(w http.ResponseWriter, r *http.Request) {
	who, roomID, ok := a.pawnActor(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	adv, ok := advantage(r.FormValue("adv"))
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	err := a.Hub.Dispatch(r.Context(), roomID, who, &room.DiceRoll{
		Expr:   r.FormValue("expr"),
		Label:  r.FormValue("label"),
		Adv:    adv,
		Secret: r.FormValue("secret") != "",
	})
	if err != nil {
		a.rejectCommand(w, "roll dice", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func advantage(raw string) (int, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return room.AdvNone, true
	}
	adv, err := strconv.Atoi(trimmed)
	if err != nil || adv < room.AdvLow || adv > room.AdvHigh {
		return 0, false
	}
	return adv, true
}
func diceData(roomID ulid.ULID, view *hub.RollsView) pages.RoomDiceData {
	data := pages.RoomDiceData{RoomID: roomID.String()}
	for i := len(view.Rolls) - 1; i >= 0; i-- {
		data.Rolls = append(data.Rolls, diceRoll(view.Rolls[i]))
	}
	return data
}
func diceRoll(r room.Roll) pages.RoomDiceRoll {
	out := pages.RoomDiceRoll{
		ID:     r.ID.String(),
		Who:    pages.RollerName(r.GM, r.Name),
		Label:  r.Label,
		Expr:   r.Expr,
		Mod:    r.Mod,
		Total:  r.Total,
		Crit:   r.Crit(),
		Fumble: r.Fumble(),
		Secret: r.Secret,
		Adv:    r.Adv,
	}
	for _, d := range r.Dice {
		out.Dice = append(out.Dice, pages.RoomDie{Value: d.Value, Kept: d.Kept, Sign: d.Sign})
	}
	return out
}
