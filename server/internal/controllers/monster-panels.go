package controllers
import (
	"database/sql"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"
	"github.com/oklog/ulid/v2"
)
func (a *App) SaveMonsterIdentity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	monsterID, ok := panelMonsterID(w, r)
	if !ok {
		return
	}
	if !parsePanelForm(w, r, "identity") {
		return
	}
	input, validationErrors := buildMonsterIdentityInput(r)
	if len(validationErrors) > 0 {
		renderPanelBlock(w, r, "identity", validationErrors)
		return
	}
	result, err := a.Queries.UpdateMonsterIdentity(ctx, queries.UpdateMonsterIdentityParams{
		Name:      input.Name,
		Size:      input.Size,
		Type:      input.Type,
		Tags:      input.Tags,
		Alignment: input.Alignment,
		ID:        monsterID,
		OwnerID:   sess.UserID,
	})
	a.finishMonsterPanel(w, r, "identity", "Identity", result, err, monsterID, sess.UserID)
}
func (a *App) SaveMonsterAbilities(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	monsterID, ok := panelMonsterID(w, r)
	if !ok {
		return
	}
	if !parsePanelForm(w, r, "abilities") {
		return
	}
	input, validationErrors := buildAbilitiesInput(r)
	if len(validationErrors) > 0 {
		renderPanelBlock(w, r, "abilities", validationErrors)
		return
	}
	result, err := a.Queries.UpdateMonsterAbilities(ctx, queries.UpdateMonsterAbilitiesParams{
		Str:     input.Str,
		Dex:     input.Dex,
		Con:     input.Con,
		Int:     input.Int,
		Wis:     input.Wis,
		Cha:     input.Cha,
		ID:      monsterID,
		OwnerID: sess.UserID,
	})
	a.finishMonsterPanel(w, r, "abilities", "Abilities", result, err, monsterID, sess.UserID)
}
func (a *App) SaveMonsterCombat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	monsterID, ok := panelMonsterID(w, r)
	if !ok {
		return
	}
	if !parsePanelForm(w, r, "combat") {
		return
	}
	input, validationErrors := buildMonsterCombatInput(r)
	if len(validationErrors) > 0 {
		renderPanelBlock(w, r, "combat", validationErrors)
		return
	}
	result, err := a.Queries.UpdateMonsterCombat(ctx, queries.UpdateMonsterCombatParams{
		AC:                        input.AC,
		HP:                        input.HP,
		HitDice:                   input.HitDice,
		Speed:                     input.Speed,
		InitiativeBonus:           input.InitiativeBonus,
		CR:                        input.CR,
		LegendaryActionUses:       input.LegendaryActionUses,
		LegendaryActionUsesInLair: input.LegendaryActionUsesInLair,
		ID:                        monsterID,
		OwnerID:                   sess.UserID,
	})
	a.finishMonsterPanel(w, r, "combat", "Combat", result, err, monsterID, sess.UserID)
}
func (a *App) SaveMonsterDefenses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	monsterID, ok := panelMonsterID(w, r)
	if !ok {
		return
	}
	if !parsePanelForm(w, r, "defenses") {
		return
	}
	input, validationErrors := buildMonsterDefensesInput(r)
	if len(validationErrors) > 0 {
		renderPanelBlock(w, r, "defenses", validationErrors)
		return
	}
	result, err := a.Queries.UpdateMonsterDefenses(ctx, queries.UpdateMonsterDefensesParams{
		Vulnerabilities: input.Vulnerabilities,
		Resistances:     input.Resistances,
		Immunities:      input.Immunities,
		Gear:            input.Gear,
		Senses:          input.Senses,
		Languages:       input.Languages,
		ID:              monsterID,
		OwnerID:         sess.UserID,
	})
	a.finishMonsterPanel(w, r, "defenses", "Defenses", result, err, monsterID, sess.UserID)
}
func (a *App) SaveMonsterBonuses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	monsterID, ok := panelMonsterID(w, r)
	if !ok {
		return
	}
	kind := r.PathValue("kind")
	label, ok := bonusPanels[kind]
	if !ok {
		unknownBonusKind(w, kind)
		return
	}
	if !parsePanelForm(w, r, kind) {
		return
	}
	misc, states, err := marshalBonusPayloads(r, kind)
	if err != nil {
		slog.Error("Failed to encode monster bonuses", "kind", kind, "error", err)
		htmx.ServerError(w)
		return
	}
	var result sql.Result
	switch kind {
	case "skills":
		result, err = a.Queries.UpdateMonsterSkills(ctx, queries.UpdateMonsterSkillsParams{
			Skills:             misc,
			SkillProficiencies: states,
			ID:                 monsterID,
			OwnerID:            sess.UserID,
		})
	case "saving_throws":
		result, err = a.Queries.UpdateMonsterSavingThrows(ctx, queries.UpdateMonsterSavingThrowsParams{
			SavingThrows:             misc,
			SavingThrowProficiencies: states,
			ID:                       monsterID,
			OwnerID:                  sess.UserID,
		})
	}
	a.finishMonsterPanel(w, r, kind, label, result, err, monsterID, sess.UserID)
}
func (a *App) SaveMonsterDescription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	monsterID, ok := panelMonsterID(w, r)
	if !ok {
		return
	}
	if !parsePanelForm(w, r, "description") {
		return
	}
	input, validationErrors := buildMonsterDescriptionInput(r)
	if len(validationErrors) > 0 {
		renderPanelBlock(w, r, "description", validationErrors)
		return
	}
	result, err := a.Queries.UpdateMonsterDescription(ctx, queries.UpdateMonsterDescriptionParams{
		Habitat:     input.Habitat,
		Treasure:    input.Treasure,
		Description: input.Description,
		ID:          monsterID,
		OwnerID:     sess.UserID,
	})
	a.finishMonsterPanel(w, r, "description", "Description", result, err, monsterID, sess.UserID)
}
type monsterIdentityInput struct {
	Name      string
	Size      string
	Type      string
	Tags      string
	Alignment string
}
func buildMonsterIdentityInput(r *http.Request) (monsterIdentityInput, []string) {
	values, validationErrors := cappedMonsterFields(r, []cappedMonsterField{
		{"tags", "Tags", pages.MonsterTagsLimit},
	})
	name := strings.TrimSpace(r.PostFormValue("name"))
	switch {
	case name == "":
		validationErrors = append(validationErrors, "Name is required.")
	case len([]rune(name)) > pages.MonsterNameLimit:
		validationErrors = append(validationErrors, "Name must be 128 characters or fewer.")
	}
	return monsterIdentityInput{
		Name:      name,
		Size:      pages.NormalizeSize(r.PostFormValue("size")),
		Type:      pages.NormalizeCreatureType(r.PostFormValue("type")),
		Tags:      values["tags"],
		Alignment: pages.NormalizeAlignment(r.PostFormValue("alignment")),
	}, validationErrors
}
type monsterCombatInput struct {
	AC                        uint8
	HP                        uint16
	HitDice                   string
	Speed                     string
	InitiativeBonus           int16
	CR                        string
	LegendaryActionUses       uint8
	LegendaryActionUsesInLair uint8
}
func buildMonsterCombatInput(r *http.Request) (monsterCombatInput, []string) {
	values, validationErrors := cappedMonsterFields(r, []cappedMonsterField{
		{"hit_dice", "Hit dice", pages.MonsterHitDiceLimit},
		{"speed", "Speed", pages.MonsterSpeedLimit},
	})
	ac, err := parseUint8(r.PostFormValue("ac"), 10)
	if err != nil {
		validationErrors = append(validationErrors, "Armor class must be between 0 and "+strconv.Itoa(pages.MonsterACLimit)+".")
	}
	hp, err := parseUint16(r.PostFormValue("hp"), 1)
	if err != nil || hp > pages.MonsterHPLimit {
		validationErrors = append(validationErrors, "Hit points must be between 0 and "+strconv.Itoa(pages.MonsterHPLimit)+".")
		hp = 1
	}
	initiativeBonus, err := parseInt16(r.PostFormValue("initiative_bonus"), 0)
	if err != nil {
		validationErrors = append(validationErrors, "Initiative bonus must be between -32768 and 32767.")
	}
	uses, err := parseUint8(r.PostFormValue("legendary_action_uses"), 0)
	if err != nil {
		validationErrors = append(validationErrors, "Legendary action uses must be between 0 and "+strconv.Itoa(pages.MonsterLegendaryUsesLimit)+".")
	}
	usesInLair, err := parseUint8(r.PostFormValue("legendary_action_uses_in_lair"), 0)
	if err != nil {
		validationErrors = append(validationErrors, "Legendary uses in lair must be between 0 and "+strconv.Itoa(pages.MonsterLegendaryUsesLimit)+".")
	}
	return monsterCombatInput{
		AC:      ac,
		HP:      hp,
		HitDice: values["hit_dice"],
		Speed:                     values["speed"],
		InitiativeBonus:           initiativeBonus,
		CR:                        pages.NormalizeChallengeRating(r.PostFormValue("cr")),
		LegendaryActionUses:       uses,
		LegendaryActionUsesInLair: usesInLair,
	}, validationErrors
}
type monsterDefensesInput struct {
	Vulnerabilities string
	Resistances     string
	Immunities      string
	Gear            string
	Senses          string
	Languages       string
}
func buildMonsterDefensesInput(r *http.Request) (monsterDefensesInput, []string) {
	values, validationErrors := cappedMonsterFields(r, []cappedMonsterField{
		{"vulnerabilities", "Vulnerabilities", pages.MonsterDefenseLimit},
		{"resistances", "Resistances", pages.MonsterDefenseLimit},
		{"immunities", "Immunities", pages.MonsterDefenseLimit},
		{"gear", "Gear", pages.MonsterDefenseLimit},
		{"senses", "Senses", pages.MonsterSensesLimit},
		{"languages", "Languages", pages.MonsterLanguagesLimit},
	})
	return monsterDefensesInput{
		Vulnerabilities: values["vulnerabilities"],
		Resistances:     values["resistances"],
		Immunities:      values["immunities"],
		Gear:            values["gear"],
		Senses:          values["senses"],
		Languages:       values["languages"],
	}, validationErrors
}
type monsterDescriptionInput struct {
	Habitat     string
	Treasure    string
	Description string
}
func buildMonsterDescriptionInput(r *http.Request) (monsterDescriptionInput, []string) {
	values, validationErrors := cappedMonsterFields(r, []cappedMonsterField{
		{"habitat", "Habitat", pages.MonsterHabitatLimit},
		{"treasure", "Treasure", pages.MonsterTreasureLimit},
	})
	description := strings.TrimSpace(r.PostFormValue("description"))
	if len(description) > pages.MonsterProseLimit {
		validationErrors = append(validationErrors, "There is too much text in the notes. Anything that long belongs in a document of its own.")
		description = ""
	}
	return monsterDescriptionInput{
		Habitat:     values["habitat"],
		Treasure:    values["treasure"],
		Description: description,
	}, validationErrors
}
type cappedMonsterField struct {
	Field string
	Label string
	Limit int
}
func cappedMonsterFields(r *http.Request, fields []cappedMonsterField) (map[string]string, []string) {
	values := map[string]string{}
	validationErrors := make([]string, 0)
	for _, field := range fields {
		value := strings.TrimSpace(r.PostFormValue(field.Field))
		if len([]rune(value)) > field.Limit {
			validationErrors = append(validationErrors, field.Label+" must be "+strconv.Itoa(field.Limit)+" characters or fewer.")
			continue
		}
		values[field.Field] = value
	}
	return values, validationErrors
}
func panelMonsterID(w http.ResponseWriter, r *http.Request) (ulid.ULID, bool) {
	monsterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "monster")
		return ulid.ULID{}, false
	}
	return monsterID, true
}
func (a *App) finishMonsterPanel(w http.ResponseWriter, r *http.Request, panel string, label string, result sql.Result, err error, monsterID, ownerID ulid.ULID) {
	if err != nil {
		slog.Error("Failed to save monster panel", "panel", panel, "error", err)
		htmx.ServerError(w)
		return
	}
	if matched, err := result.RowsAffected(); err == nil && matched == 0 {
		htmx.NotFound(w, "monster")
		return
	}
	htmx.Toast(w, label+" saved.")
	renderPanelBlock(w, r, panel, nil)
	a.redrawMonster(w, r, panel, monsterID, ownerID)
}
func (a *App) redrawMonster(w http.ResponseWriter, r *http.Request, panel string, monsterID, ownerID ulid.ULID) {
	ctx := r.Context()
	monster, err := a.Queries.GetMonster(ctx, queries.GetMonsterParams{ID: monsterID, OwnerID: ownerID})
	if err != nil {
		slog.Error("Failed to read the monster back", "panel", panel, "error", err)
		return
	}
	actions, err := a.Queries.ListMonsterActions(ctx, queries.ListMonsterActionsParams{MonsterID: monsterID, OwnerID: ownerID})
	if err != nil {
		slog.Error("Failed to read the monster's actions back", "panel", panel, "error", err)
		return
	}
	derived := monsterDerived(monster, actions)
	render(w, r, pages.MonsterDerivedValues(derived))
	render(w, r, pages.MonsterBarValues(monsterHeader(monster, derived)))
	render(w, r, pages.MonsterStatBlock(monsterStatBlock(monster, actions, derived), true))
}
