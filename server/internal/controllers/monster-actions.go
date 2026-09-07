package controllers

// The seven sections of a stat block, where the ROW is the unit of work rather
// than the panel. It is the attacks table's shape -- add answers with the row it
// made, each row autosaves to its own URL, delete answers 200 -- with two
// differences worth naming.
//
// THE KIND RIDES IN THE PATH, because a row cannot change section: it identifies
// the row as much as its id does, and it is what tells an add which of the seven
// containers to append to. It is matched against the allowlist before any
// statement runs, so the ENUM behind the column never has to refuse anything.
//
// EVERY ONE OF THE THREE REDRAWS THE STAT BLOCK. A save is obvious -- the block
// prints what is being typed. The add matters as much: a blank Actions row is
// what turns a CR 0 monster from 0 XP into 10, and the first lair action is what
// puts the in-lair figure on the CR line.

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

// AddMonsterAction creates an empty row in one section and answers with it. The
// row is read back rather than assembled from what the insert "should" have
// written, so the schema stays the only place a new row's starting state is
// declared.
//
// Answering a POST with markup is the mutation case the fragment rules name --
// the alternative is a POST that returns nothing followed by a GET to fetch what
// it just made.
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
	// The statement selects from monsters, so a monster that is not this user's
	// matches nothing and inserts nothing. Zero rows is that, and it is the only
	// thing it can be: the id is freshly minted, so a duplicate key is not on
	// the table.
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

// SaveMonsterAction is the row's autosave: two fields, two columns, and the kind
// deliberately not among them -- moving a Bite from Actions to Reactions is
// deleting a row and adding another, not an edit.
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

// DeleteMonsterAction drops one row. It MUST answer 200: base.templ's noSwap
// config lists 204, and a status in that list sets the swap to "none", which
// overrides the hx-swap="delete" on the button and leaves the row sitting on
// screen after the database has dropped it.
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

	// The swap style is "delete", so the body is thrown away -- but htmx lifts
	// out-of-band elements out of a response before it swaps anything, so the
	// block still redraws without the row that has just gone.
	a.redrawMonster(w, r, section.Kind, monsterID, sess.UserID)
}

// finishMonsterActionRow is finishMonsterPanel with one word changed, and the
// word is the reason it is not that function. Zero matched rows on a panel means
// the monster is not this user's; here it means this row is gone -- deleted in
// another tab, most likely -- and telling somebody their monster no longer
// exists because a row does would send them to look for the wrong problem.
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

// monsterActionKind is the allowlist gate, and it has unknownBonusKind's shape:
// only a hand-built request reaches it, because every kind the editor sends is
// one of seven constants in a template, so the message says the page is wrong
// rather than that the monster is gone.
//
// IT RUNS BEFORE ANY STATEMENT. The column is an ENUM, which would refuse an
// unknown kind on its own -- as a 500. This is what keeps it from having to.
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

// A row spends its first seconds nameless -- it is created blank and typed into
// -- and a debounce landing in there should not toast " saved.". The section's
// own noun stands in, which is why it is a field on the section rather than a
// word in the add button.
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

// Two fields, and they are capped for the reason every other box on this editor
// is: MySQL runs in strict mode, so an overlong value comes back from the driver
// as an error rather than as a truncation. The name is measured in characters,
// which is what its VARCHAR counts; the description in bytes, which is what its
// TEXT counts.
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
