package controllers
import (
	"database/sql"
	"log/slog"
	"net/http"
	"strings"
	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"
	"github.com/oklog/ulid/v2"
)
func (a *App) AddMonsterAction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	monsterID, ok := panelMonsterID(w, r)
	if !ok {
		return
	}
	section, ok := monsterActionKind(w, r)
	if !ok {
		return
	}
	actionID := ulid.Make()
	result, err := a.Queries.InsertMonsterAction(ctx, queries.InsertMonsterActionParams{
		ID:        actionID,
		Kind:      queries.MonsterActionsKind(section.Kind),
		MonsterID: monsterID,
		OwnerID:   sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to add monster action", "kind", section.Kind, "error", err)
		htmx.ServerError(w)
		return
	}
	if inserted, err := result.RowsAffected(); err == nil && inserted == 0 {
		htmx.NotFound(w, "monster")
		return
	}
	action, err := a.Queries.GetMonsterAction(ctx, queries.GetMonsterActionParams{
		ID:        actionID,
		MonsterID: monsterID,
		OwnerID:   sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to read back new monster action", "error", err)
		htmx.ServerError(w)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.MonsterActionRow(monsterID.String(), monsterActionPageRow(action)))
	a.redrawMonster(w, r, section.Kind, monsterID, sess.UserID)
}
func (a *App) SaveMonsterAction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	monsterID, ok := panelMonsterID(w, r)
	if !ok {
		return
	}
	section, ok := monsterActionKind(w, r)
	if !ok {
		return
	}
	actionID, ok := monsterActionRowID(w, r)
	if !ok {
		return
	}
	panel := pages.MonsterActionRowPanel(actionID.String())
	if !parsePanelForm(w, r, panel) {
		return
	}
	input, problems := buildMonsterActionInput(r)
	if len(problems) > 0 {
		renderPanelBlock(w, r, panel, problems)
		return
	}
	result, err := a.Queries.UpdateMonsterAction(ctx, queries.UpdateMonsterActionParams{
		Name:        input.Name,
		Description: input.Description,
		ID:          actionID,
		MonsterID:   monsterID,
		OwnerID:     sess.UserID,
	})
	a.finishMonsterActionRow(w, r, panel, monsterActionToastLabel(section, input.Name), result, err, monsterID, sess.UserID)
}
func (a *App) DeleteMonsterAction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	monsterID, ok := panelMonsterID(w, r)
	if !ok {
		return
	}
	section, ok := monsterActionKind(w, r)
	if !ok {
		return
	}
	actionID, ok := monsterActionRowID(w, r)
	if !ok {
		return
	}
	result, err := a.Queries.DeleteMonsterAction(ctx, queries.DeleteMonsterActionParams{
		ID:        actionID,
		MonsterID: monsterID,
		OwnerID:   sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to delete monster action", "error", err)
		htmx.ServerError(w)
		return
	}
	if deleted, err := result.RowsAffected(); err == nil && deleted == 0 {
		htmx.NotFound(w, "action")
		return
	}
	htmx.Toast(w, section.Singular+" deleted.")
	a.redrawMonster(w, r, section.Kind, monsterID, sess.UserID)
}
func (a *App) finishMonsterActionRow(w http.ResponseWriter, r *http.Request, panel string, label string, result sql.Result, err error, monsterID, ownerID ulid.ULID) {
	if err != nil {
		slog.Error("Failed to save monster action", "error", err)
		htmx.ServerError(w)
		return
	}
	if matched, err := result.RowsAffected(); err == nil && matched == 0 {
		htmx.NotFound(w, "action")
		return
	}
	htmx.Toast(w, label+" saved.")
	renderPanelBlock(w, r, panel, nil)
	a.redrawMonster(w, r, panel, monsterID, ownerID)
}
func monsterActionKind(w http.ResponseWriter, r *http.Request) (pages.MonsterActionSection, bool) {
	kind := r.PathValue("kind")
	section, ok := pages.MonsterActionSectionFor(kind)
	if !ok {
		slog.Warn("unknown monster action kind requested", "kind", kind)
		htmx.Error(w, "Not Found", "That part of the stat block does not exist. Refresh the page and try again.", http.StatusNotFound)
		return pages.MonsterActionSection{}, false
	}
	return section, true
}
func monsterActionRowID(w http.ResponseWriter, r *http.Request) (ulid.ULID, bool) {
	actionID, err := ulid.Parse(r.PathValue("actionId"))
	if err != nil {
		htmx.NotFound(w, "action")
		return ulid.ULID{}, false
	}
	return actionID, true
}
func monsterActionToastLabel(section pages.MonsterActionSection, name string) string {
	if name == "" {
		return section.Singular
	}
	return name
}
type monsterActionInput struct {
	Name        string
	Description string
}
func buildMonsterActionInput(r *http.Request) (monsterActionInput, []string) {
	var problems []string
	name := strings.TrimSpace(r.PostFormValue("name"))
	if len([]rune(name)) > pages.MonsterActionNameLimit {
		problems = append(problems, "Name must be 128 characters or fewer.")
	}
	description := strings.TrimSpace(r.PostFormValue("description"))
	if len(description) > pages.MonsterProseLimit {
		problems = append(problems, "That description is too long to save.")
	}
	return monsterActionInput{Name: name, Description: description}, problems
}
func monsterActionPageRow(row queries.MonsterAction) pages.MonsterAction {
	return pages.MonsterAction{
		ID:          row.ID.String(),
		Kind:        string(row.Kind),
		Name:        row.Name,
		Description: row.Description,
	}
}
