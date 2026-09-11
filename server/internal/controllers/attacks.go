package controllers

import (
	"log/slog"
	"net/http"
	"strings"

	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)
















const (
	attackNameLimit   = 128
	attackBonusLimit  = 32
	attackDamageLimit = 64
	attackNotesLimit  = 65535
)








func (a *App) AddAttack(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}

	attackID := ulid.Make()
	
	
	
	
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
	
	
	finishRow(w, r, panel, attackToastLabel(input.Name), "attack", result, err)
}





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
