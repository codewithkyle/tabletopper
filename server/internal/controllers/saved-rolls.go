package controllers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

const savedRollsMax = 24

func (a *App) RoomSaveRollFragment(w http.ResponseWriter, r *http.Request) {
	row, _, ok := a.member(w, r)
	if !ok {
		return
	}
	raw := r.URL.Query().Get("expr")
	if utf8.RuneCountInString(raw) > room.DiceExprLimit {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	data := pages.RoomSaveRollData{RoomID: row.ID.String(), Expr: strings.TrimSpace(raw)}
	_, expr, err := room.CheckDice("", raw)
	if err != nil {
		problem, ok := diceProblem(err)
		if !ok {
			slog.Error("Failed to check a roll before saving it", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		data.Problems = []string{problem}
	} else {
		data.Expr = expr
	}
	renderFragment(w, r, pages.RoomSaveRoll(data))
}
func (a *App) SaveRoll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	if _, _, ok := a.member(w, r); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	label, expr, err := room.CheckDice(r.FormValue("label"), r.FormValue("expr"))
	if err != nil {
		problem, ok := diceProblem(err)
		if !ok {
			slog.Error("Failed to check a roll before saving it", "error", err)
			htmx.ServerError(w)
			return
		}
		renderPanelBlock(w, r, pages.SaveRollPanel, []string{problem})
		return
	}
	saved, err := a.Queries.ListSavedRolls(ctx, sess.UserID)
	if err != nil {
		slog.Error("Failed to list saved rolls", "error", err)
		htmx.ServerError(w)
		return
	}
	for _, s := range saved {
		if strings.EqualFold(s.Label, label) && compactDice(s.Expr) == compactDice(expr) {
			renderPanelBlock(w, r, pages.SaveRollPanel, []string{"That roll is already saved."})
			return
		}
	}
	if len(saved) >= savedRollsMax {
		renderPanelBlock(w, r, pages.SaveRollPanel, []string{
			fmt.Sprintf("You can keep %d saved rolls. Remove one to make room for another.", savedRollsMax),
		})
		return
	}
	err = a.Queries.InsertSavedRoll(ctx, queries.InsertSavedRollParams{
		ID:      ulid.Make(),
		OwnerID: sess.UserID,
		Label:   label,
		Expr:    expr,
	})
	if err != nil {
		slog.Error("Failed to save a roll", "error", err)
		htmx.ServerError(w)
		return
	}
	htmx.CloseModal(w)
	htmx.Toast(w, pages.RoomSavedRoll{Label: label, Expr: expr}.Name()+" is saved.")
	htmx.SavedRolls(w)
	w.WriteHeader(http.StatusNoContent)
}
func (a *App) DeleteSavedRoll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	id, ok := pathID(w, r, "id", "saved roll")
	if !ok {
		return
	}
	_, err := a.Queries.DeleteSavedRoll(ctx, queries.DeleteSavedRollParams{ID: id, OwnerID: sess.UserID})
	if err != nil {
		slog.Error("Failed to delete a saved roll", "error", err)
		htmx.ServerError(w)
		return
	}
	htmx.Toast(w, "Saved roll removed.")
}
func diceProblem(err error) (string, bool) {
	var refusal *room.Error
	if !errors.As(err, &refusal) {
		return "", false
	}
	return refusal.Message, true
}
func savedRoll(s queries.ListSavedRollsRow) pages.RoomSavedRoll {
	return pages.RoomSavedRoll{ID: s.ID.String(), Label: s.Label, Expr: s.Expr}
}
func compactDice(expr string) string {
	return strings.Join(strings.Fields(expr), "")
}
