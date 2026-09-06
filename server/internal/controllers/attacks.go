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

// The Attacks panel, and the second place on the sheet where the row is the unit
// of work rather than the panel. It follows inventory's shape exactly -- add
// answers with the row it made, each row autosaves to its own URL, delete
// answers 200 -- and differs from it in one way worth naming: these rows live on
// the Character tab rather than a tab of their own. Inventory and spells earned
// their own pages by being long. A character has three or four attacks and reads
// them every round, so a tab switch mid-turn would be the cost of the
// consistency.
//
// The column widths are enforced here because MySQL runs in strict mode: an
// overlong value comes back from the driver as an error, so without these a
// pasted paragraph in a 32-character field would reach the player as a 500 on a
// field they were entitled to overfill. The four varchars are measured in
// characters, which is what VARCHAR counts; notes is measured in bytes, which is
// what TEXT counts.
const (
	attackNameLimit   = 128
	attackBonusLimit  = 32
	attackDamageLimit = 64
	attackNotesLimit  = 65535
)

// AddAttack creates an empty row and answers with it. The row is read back
// rather than assembled from what the insert "should" have written, so the
// schema stays the only place a new attack's starting state is declared.
//
// Answering a POST with markup is the mutation case the fragment rules name --
// the alternative is a POST that returns nothing followed by a GET to fetch what
// it just made.
func (a *App) AddAttack(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}

	attackID := ulid.Make()
	// The statement selects from characters, so a character that is not this
	// user's matches nothing and inserts nothing. Zero rows is that, and it is
	// the only thing it can be: the id is freshly minted, so a duplicate key is
	// not on the table.
	result, err := a.Queries.InsertAttack(ctx, queries.InsertAttackParams{
		ID:          attackID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to add attack", "error", err)
		htmx.ServerError(w)
		return
	}
	if inserted, err := result.RowsAffected(); err == nil && inserted == 0 {
		htmx.NotFound(w, "character")
		return
	}

	attack, err := a.Queries.GetAttack(ctx, queries.GetAttackParams{
		ID:          attackID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to read back new attack", "error", err)
		htmx.ServerError(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.AttackRow(characterID.String(), attackPageRow(attack)))
}

// SaveAttack is the row's autosave. It writes six columns and reads six fields,
// and the form that posts them renders all six together.
func (a *App) SaveAttack(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	attackID, ok := attackRowID(w, r)
	if !ok {
		return
	}

	panel := pages.AttackRowPanel(attackID.String())
	if !parsePanelForm(w, r, panel) {
		return
	}

	input, problems := buildAttackInput(r)
	if len(problems) > 0 {
		renderPanelBlock(w, r, panel, problems)
		return
	}

	result, err := a.Queries.UpdateAttack(ctx, queries.UpdateAttackParams{
		Name:        input.Name,
		AttackBonus: input.Bonus,
		Damage:      input.Damage,
		DamageType:  input.DamageType,
		Mastery:     input.Mastery,
		Notes:       input.Notes,
		ID:          attackID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	finishAttackRow(w, r, panel, input.Name, result, err)
}

// DeleteAttack drops one row. The reply carries no body, and it MUST be a 200:
// base.templ's noSwap config lists 204, and a status in that list sets the swap
// to "none", which overrides the hx-swap="delete" on the button and leaves the
// row sitting on screen after the database has dropped it.
func (a *App) DeleteAttack(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	attackID, ok := attackRowID(w, r)
	if !ok {
		return
	}

	result, err := a.Queries.DeleteAttack(ctx, queries.DeleteAttackParams{
		ID:          attackID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to delete attack", "error", err)
		htmx.ServerError(w)
		return
	}
	if deleted, err := result.RowsAffected(); err == nil && deleted == 0 {
		htmx.NotFound(w, "attack")
		return
	}

	htmx.Toast(w, "Attack deleted.")
}

// finishAttackRow is finishInventoryRow with one word changed, and the word is
// the reason it is not that function. Zero matched rows on a panel means the
// character is not this user's; here it means this attack is gone -- deleted in
// another tab, most likely -- and telling someone their character no longer
// exists because a row does would send them to look for the wrong problem.
func finishAttackRow(w http.ResponseWriter, r *http.Request, panel string, name string, result sql.Result, err error) {
	if err != nil {
		slog.Error("Failed to save attack", "error", err)
		htmx.ServerError(w)
		return
	}

	if matched, err := result.RowsAffected(); err == nil && matched == 0 {
		htmx.NotFound(w, "attack")
		return
	}

	htmx.Toast(w, attackToastLabel(name)+" saved.")
	renderPanelBlock(w, r, panel, nil)
}

// A row spends its first seconds nameless, and a debounce landing in there
// should not toast " saved.".
func attackToastLabel(name string) string {
	if name == "" {
		return "Attack"
	}

	return name
}

type attackInput struct {
	Name       string
	Bonus      string
	Damage     string
	DamageType string
	Mastery    string
	Notes      string
}

// The two selects are normalised rather than validated. A value that is not on
// the list becomes the empty member, which is what an unset one already is --
// see NormalizeDamageType for why that is the right answer here and a fallback
// to a real member is the right one for a spell's school. Only a hand-built
// request can produce one, because the select offers nothing else.
func buildAttackInput(r *http.Request) (attackInput, []string) {
	var problems []string

	name := strings.TrimSpace(r.PostFormValue("name"))
	if len([]rune(name)) > attackNameLimit {
		problems = append(problems, "Attack name must be 128 characters or fewer.")
	}

	bonus := strings.TrimSpace(r.PostFormValue("attack_bonus"))
	if len([]rune(bonus)) > attackBonusLimit {
		problems = append(problems, "Attack bonus must be 32 characters or fewer.")
	}

	damage := strings.TrimSpace(r.PostFormValue("damage"))
	if len([]rune(damage)) > attackDamageLimit {
		problems = append(problems, "Damage must be 64 characters or fewer.")
	}

	notes := strings.TrimSpace(r.PostFormValue("notes"))
	if len(notes) > attackNotesLimit {
		problems = append(problems, "Those notes are too long to save.")
	}

	return attackInput{
		Name:       name,
		Bonus:      bonus,
		Damage:     damage,
		DamageType: pages.NormalizeDamageType(r.PostFormValue("damage_type")),
		Mastery:    pages.NormalizeMastery(r.PostFormValue("mastery")),
		Notes:      notes,
	}, problems
}

func attackRowID(w http.ResponseWriter, r *http.Request) (ulid.ULID, bool) {
	attackID, err := ulid.Parse(r.PathValue("attackId"))
	if err != nil {
		htmx.NotFound(w, "attack")
		return ulid.ULID{}, false
	}

	return attackID, true
}

func attackPageRows(rows []queries.Attack) []pages.Attack {
	attacks := make([]pages.Attack, 0, len(rows))
	for _, row := range rows {
		attacks = append(attacks, attackPageRow(row))
	}

	return attacks
}

func attackPageRow(row queries.Attack) pages.Attack {
	return pages.Attack{
		ID:         row.ID.String(),
		Name:       row.Name,
		Bonus:      row.AttackBonus,
		Damage:     row.Damage,
		DamageType: row.DamageType,
		Mastery:    row.Mastery,
		Notes:      row.Notes,
	}
}
