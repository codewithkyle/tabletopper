package controllers
import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"
	"github.com/oklog/ulid/v2"
)
var bonusPanels = map[string]string{
	"skills":        "Skills",
	"saving_throws": "Saving throws",
}
func (a *App) SaveCharacterIdentity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	if !parsePanelForm(w, r, "identity") {
		return
	}
	input, validationErrors := buildIdentityInput(r)
	if len(validationErrors) > 0 {
		renderPanelBlock(w, r, "identity", validationErrors)
		return
	}
	result, err := a.Queries.UpdateCharacterIdentity(ctx, queries.UpdateCharacterIdentityParams{
		Name:       input.Name,
		Race:       input.Race,
		Background: input.Background,
		Alignment:  input.Alignment,
		Classes:    input.Classes,
		Size:       input.Size,
		ID:         characterID,
		OwnerID:    sess.UserID,
	})
	a.finishCharacterPanel(w, r, "identity", "Identity", result, err, characterID, sess.UserID)
}
func (a *App) SaveCharacterAbilities(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	characterID, ok := panelCharacterID(w, r)
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
	result, err := a.Queries.UpdateCharacterAbilities(ctx, queries.UpdateCharacterAbilitiesParams{
		Str:     input.Str,
		Dex:     input.Dex,
		Con:     input.Con,
		Int:     input.Int,
		Wis:     input.Wis,
		Cha:     input.Cha,
		ID:      characterID,
		OwnerID: sess.UserID,
	})
	a.finishCharacterPanel(w, r, "abilities", "Abilities", result, err, characterID, sess.UserID)
}
func (a *App) SaveCharacterCoreStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	if !parsePanelForm(w, r, "core-stats") {
		return
	}
	input, validationErrors := buildCoreStatsInput(r)
	if len(validationErrors) > 0 {
		renderPanelBlock(w, r, "core-stats", validationErrors)
		return
	}
	result, err := a.Queries.UpdateCharacterCoreStats(ctx, queries.UpdateCharacterCoreStatsParams{
		XP:                  input.XP,
		Level:               input.Level,
		ProficiencyBonus:    input.ProficiencyBonus,
		Speed:               input.Speed,
		AC:                  input.AC,
		InitiativeBonus:     input.InitiativeBonus,
		SpellcastingAbility: input.SpellcastingAbility,
		SpellBonusMisc:      input.SpellBonusMisc,
		ID:                  characterID,
		OwnerID:             sess.UserID,
	})
	a.finishCharacterPanel(w, r, "core-stats", "Core stats", result, err, characterID, sess.UserID)
}
func (a *App) SaveCharacterVitals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	if !parsePanelForm(w, r, "vitals") {
		return
	}
	input, validationErrors := buildVitalsInput(r)
	if len(validationErrors) > 0 {
		renderPanelBlock(w, r, "vitals", validationErrors)
		return
	}
	result, err := a.Queries.UpdateCharacterVitals(ctx, queries.UpdateCharacterVitalsParams{
		MaxHP:              input.MaxHP,
		CurrentHP:          input.CurrentHP,
		TempHP:             input.TempHP,
		HitDice:            input.HitDice,
		HitDiceSpent:       input.HitDiceSpent,
		DeathSaveSuccesses: input.DeathSaveSuccesses,
		DeathSaveFailures:  input.DeathSaveFailures,
		HeroicInspiration:  input.HeroicInspiration,
		Exhaustion:         input.Exhaustion,
		ID:                 characterID,
		OwnerID:            sess.UserID,
	})
	a.finishCharacterPanel(w, r, "vitals", "Vitals", result, err, characterID, sess.UserID)
}
func (a *App) SaveCharacterProficiencies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	if !parsePanelForm(w, r, "proficiencies") {
		return
	}
	input := buildProficienciesInput(r)
	result, err := a.Queries.UpdateCharacterProficiencies(ctx, queries.UpdateCharacterProficienciesParams{
		Languages:     input.Languages,
		Proficiencies: input.Proficiencies,
		ID:            characterID,
		OwnerID:       sess.UserID,
	})
	a.finishCharacterPanel(w, r, "proficiencies", "Proficiencies", result, err, characterID, sess.UserID)
}
func (a *App) SaveCharacterPersonality(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	if !parsePanelForm(w, r, "personality") {
		return
	}
	input, validationErrors := buildPersonalityInput(r)
	if len(validationErrors) > 0 {
		renderPanelBlock(w, r, "personality", validationErrors)
		return
	}
	result, err := a.Queries.UpdateCharacterPersonality(ctx, queries.UpdateCharacterPersonalityParams{
		PersonalityTraits: input.PersonalityTraits,
		Ideals:            input.Ideals,
		Bonds:             input.Bonds,
		Flaws:             input.Flaws,
		ID:                characterID,
		OwnerID:           sess.UserID,
	})
	a.finishCharacterPanel(w, r, "personality", "Personality", result, err, characterID, sess.UserID)
}
func (a *App) SaveCharacterAppearance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	if !parsePanelForm(w, r, "appearance") {
		return
	}
	input, validationErrors := buildAppearanceInput(r)
	if len(validationErrors) > 0 {
		renderPanelBlock(w, r, "appearance", validationErrors)
		return
	}
	result, err := a.Queries.UpdateCharacterAppearance(ctx, queries.UpdateCharacterAppearanceParams{
		Age:     input.Age,
		Height:  input.Height,
		Weight:  input.Weight,
		Eyes:    input.Eyes,
		Skin:    input.Skin,
		Hair:    input.Hair,
		ID:      characterID,
		OwnerID: sess.UserID,
	})
	a.finishCharacterPanel(w, r, "appearance", "Appearance", result, err, characterID, sess.UserID)
}
func (a *App) SaveCharacterBonuses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	characterID, ok := panelCharacterID(w, r)
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
		slog.Error("Failed to encode bonuses", "kind", kind, "error", err)
		htmx.ServerError(w)
		return
	}
	var result sql.Result
	switch kind {
	case "skills":
		result, err = a.Queries.UpdateCharacterSkills(ctx, queries.UpdateCharacterSkillsParams{
			Skills:             misc,
			SkillProficiencies: states,
			ID:                 characterID,
			OwnerID:            sess.UserID,
		})
	case "saving_throws":
		result, err = a.Queries.UpdateCharacterSavingThrows(ctx, queries.UpdateCharacterSavingThrowsParams{
			SavingThrows:             misc,
			SavingThrowProficiencies: states,
			ID:                       characterID,
			OwnerID:                  sess.UserID,
		})
	}
	a.finishCharacterPanel(w, r, kind, label, result, err, characterID, sess.UserID)
}
func (a *App) SaveCharacterFeatures(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	if !parsePanelForm(w, r, pages.FeaturesPanel) {
		return
	}
	payload, err := marshalFeatureRowsPayload(r)
	if err != nil {
		slog.Error("Failed to encode feature rows", "error", err)
		htmx.ServerError(w)
		return
	}
	result, err := a.Queries.UpdateCharacterFeatures(ctx, queries.UpdateCharacterFeaturesParams{
		Features: payload,
		ID:       characterID,
		OwnerID:  sess.UserID,
	})
	a.finishCharacterPanel(w, r, pages.FeaturesPanel, "Features", result, err, characterID, sess.UserID)
}
type identityInput struct {
	Name       string
	Race       sql.NullString
	Background sql.NullString
	Alignment  sql.NullString
	Classes    sql.NullString
	Size       string
}
func buildIdentityInput(r *http.Request) (identityInput, []string) {
	validationErrors := make([]string, 0)
	name := strings.TrimSpace(r.PostFormValue("name"))
	if name == "" {
		validationErrors = append(validationErrors, "Name is required.")
	}
	size := strings.TrimSpace(r.PostFormValue("size"))
	if size == "" {
		validationErrors = append(validationErrors, "Size is required.")
	}
	return identityInput{
		Name:       name,
		Race:       nullableString(r.PostFormValue("race")),
		Background: nullableString(r.PostFormValue("background")),
		Alignment:  nullableString(r.PostFormValue("alignment")),
		Classes:    nullableString(r.PostFormValue("classes")),
		Size:       size,
	}, validationErrors
}
type abilitiesInput struct {
	Str uint8
	Dex uint8
	Con uint8
	Int uint8
	Wis uint8
	Cha uint8
}
func buildAbilitiesInput(r *http.Request) (abilitiesInput, []string) {
	validationErrors := make([]string, 0)
	abilities := map[string]uint8{}
	for _, ability := range []struct{ Field, Label string }{
		{"str", "Strength"},
		{"dex", "Dexterity"},
		{"con", "Constitution"},
		{"int", "Intelligence"},
		{"wis", "Wisdom"},
		{"cha", "Charisma"},
	} {
		value, err := parseUint8(r.PostFormValue(ability.Field), 10)
		if err != nil {
			validationErrors = append(validationErrors, ability.Label+" must be between 0 and 255.")
		}
		abilities[ability.Field] = value
	}
	return abilitiesInput{
		Str: abilities["str"],
		Dex: abilities["dex"],
		Con: abilities["con"],
		Int: abilities["int"],
		Wis: abilities["wis"],
		Cha: abilities["cha"],
	}, validationErrors
}
type coreStatsInput struct {
	XP                  uint32
	Level               uint8
	ProficiencyBonus    uint16
	Speed               string
	AC                  uint16
	InitiativeBonus     int16
	SpellcastingAbility queries.CharactersSpellcastingAbility
	SpellBonusMisc      int16
}
func buildCoreStatsInput(r *http.Request) (coreStatsInput, []string) {
	validationErrors := make([]string, 0)
	xp, err := parseUint32(r.PostFormValue("xp"), 0)
	if err != nil {
		validationErrors = append(validationErrors, "XP must be a valid non-negative number.")
	}
	ac, err := parseUint16(r.PostFormValue("ac"), 10)
	if err != nil {
		validationErrors = append(validationErrors, "Armor class must be between 0 and 65535.")
	}
	initiativeBonus, err := parseInt16(r.PostFormValue("initiative_bonus"), 0)
	if err != nil {
		validationErrors = append(validationErrors, "Initiative bonus must be between -32768 and 32767.")
	}
	spellBonusMisc, err := parseInt16(r.PostFormValue("spell_bonus_misc"), 0)
	if err != nil {
		validationErrors = append(validationErrors, "Spell bonus must be between -32768 and 32767.")
	}
	speed := strings.TrimSpace(r.PostFormValue("speed"))
	if speed == "" {
		speed = "30 ft."
	}
	level := levelFromXP(xp)
	return coreStatsInput{
		XP:               xp,
		Level:            level,
		ProficiencyBonus: proficiencyBonusForLevel(level),
		Speed:            speed,
		AC:               ac,
		InitiativeBonus:  initiativeBonus,
		SpellcastingAbility: queries.CharactersSpellcastingAbility(pages.NormalizeSpellcastingAbility(r.PostFormValue("spellcasting_ability"))),
		SpellBonusMisc:      spellBonusMisc,
	}, validationErrors
}
type vitalsInput struct {
	MaxHP              uint16
	CurrentHP          uint16
	TempHP             uint16
	HitDice            string
	HitDiceSpent       uint8
	DeathSaveSuccesses uint8
	DeathSaveFailures  uint8
	HeroicInspiration  bool
	Exhaustion         uint8
}
func buildVitalsInput(r *http.Request) (vitalsInput, []string) {
	validationErrors := make([]string, 0)
	maxHP, err := parseUint16(r.PostFormValue("max_hp"), 1)
	if err != nil {
		validationErrors = append(validationErrors, "Max hit points must be between 0 and 65535.")
	}
	currentHP, err := parseUint16(r.PostFormValue("current_hp"), 1)
	if err != nil {
		validationErrors = append(validationErrors, "Hit points must be between 0 and 65535.")
	}
	tempHP, err := parseUint16(r.PostFormValue("temp_hp"), 0)
	if err != nil {
		validationErrors = append(validationErrors, "Temp hit points must be between 0 and 65535.")
	}
	hitDice := strings.TrimSpace(r.PostFormValue("hit_dice"))
	if len([]rune(hitDice)) > characterWordLimit {
		validationErrors = append(validationErrors, "Hit dice must be 64 characters or fewer.")
	}
	spent, err := parseUint8(r.PostFormValue("hit_dice_spent"), 0)
	if err != nil || spent > pages.HitDiceSpentLimit {
		validationErrors = append(validationErrors, "Spent hit dice must be between 0 and 20.")
		spent = 0
	}
	exhaustion, err := parseUint8(r.PostFormValue("exhaustion"), 0)
	if err != nil || exhaustion > pages.ExhaustionLimit {
		validationErrors = append(validationErrors, "Exhaustion must be between 0 and 6.")
		exhaustion = 0
	}
	successes, ok := deathSaves(r, "death_save_successes")
	if !ok {
		validationErrors = append(validationErrors, "Death save successes must be between 0 and 3.")
	}
	failures, ok := deathSaves(r, "death_save_failures")
	if !ok {
		validationErrors = append(validationErrors, "Death save failures must be between 0 and 3.")
	}
	return vitalsInput{
		MaxHP:              maxHP,
		CurrentHP:          currentHP,
		TempHP:             tempHP,
		HitDice:            hitDice,
		HitDiceSpent:       spent,
		DeathSaveSuccesses: successes,
		DeathSaveFailures:  failures,
		HeroicInspiration:  r.PostFormValue("heroic_inspiration") != "",
		Exhaustion:         exhaustion,
	}, validationErrors
}
func deathSaves(r *http.Request, field string) (uint8, bool) {
	ticked := len(r.PostForm[field])
	if ticked > pages.DeathSaveLimit {
		return 0, false
	}
	return uint8(ticked), true
}
type proficienciesInput struct {
	Languages     string
	Proficiencies string
}
func buildProficienciesInput(r *http.Request) proficienciesInput {
	languages := strings.TrimSpace(r.PostFormValue("languages"))
	if languages == "" {
		languages = "Common"
	}
	return proficienciesInput{
		Languages:     languages,
		Proficiencies: strings.TrimSpace(r.PostFormValue("proficiencies")),
	}
}
const (
	characterProseLimit = 4096
	characterWordLimit  = 64
)
type personalityInput struct {
	PersonalityTraits string
	Ideals            string
	Bonds             string
	Flaws             string
}
func buildPersonalityInput(r *http.Request) (personalityInput, []string) {
	validationErrors := make([]string, 0)
	prose := map[string]string{}
	for _, box := range []struct{ Field, Label string }{
		{"personality_traits", "Personality Traits"},
		{"ideals", "Ideals"},
		{"bonds", "Bonds"},
		{"flaws", "Flaws"},
	} {
		value := strings.TrimSpace(r.PostFormValue(box.Field))
		if len(value) > characterProseLimit {
			validationErrors = append(validationErrors, "There is too much text in "+strings.ToLower(box.Label)+". Anything that long belongs in the journal.")
			continue
		}
		prose[box.Field] = value
	}
	return personalityInput{
		PersonalityTraits: prose["personality_traits"],
		Ideals:            prose["ideals"],
		Bonds:             prose["bonds"],
		Flaws:             prose["flaws"],
	}, validationErrors
}
type appearanceInput struct {
	Age    string
	Height string
	Weight string
	Eyes   string
	Skin   string
	Hair   string
}
func buildAppearanceInput(r *http.Request) (appearanceInput, []string) {
	validationErrors := make([]string, 0)
	details := map[string]string{}
	for _, field := range []struct{ Field, Label string }{
		{"age", "Age"},
		{"height", "Height"},
		{"weight", "Weight"},
		{"eyes", "Eyes"},
		{"skin", "Skin"},
		{"hair", "Hair"},
	} {
		value := strings.TrimSpace(r.PostFormValue(field.Field))
		if len([]rune(value)) > characterWordLimit {
			validationErrors = append(validationErrors, field.Label+" must be 64 characters or fewer.")
			continue
		}
		details[field.Field] = value
	}
	return appearanceInput{
		Age:    details["age"],
		Height: details["height"],
		Weight: details["weight"],
		Eyes:   details["eyes"],
		Skin:   details["skin"],
		Hair:   details["hair"],
	}, validationErrors
}
func panelCharacterID(w http.ResponseWriter, r *http.Request) (ulid.ULID, bool) {
	characterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "character")
		return ulid.ULID{}, false
	}
	return characterID, true
}
func parsePanelForm(w http.ResponseWriter, r *http.Request, panel string) bool {
	if err := r.ParseForm(); err != nil {
		renderPanelBlock(w, r, panel, []string{"The submitted form data could not be read."})
		return false
	}
	return true
}
func renderPanelBlock(w http.ResponseWriter, r *http.Request, panel string, messages []string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if len(messages) > 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}
	render(w, r, pages.PanelFormErrors(panel, messages))
}
func unknownBonusKind(w http.ResponseWriter, kind string) {
	slog.Warn("unknown character bonus kind requested", "kind", kind)
	htmx.Error(w, "Not Found", "That part of the character sheet does not exist. Refresh the page and try again.", http.StatusNotFound)
}
func savedRow(w http.ResponseWriter, panel, gone string, result sql.Result, err error) bool {
	if err != nil {
		slog.Error("Failed to save", "panel", panel, "row", gone, "error", err)
		htmx.ServerError(w)
		return false
	}
	if matched, err := result.RowsAffected(); err == nil && matched == 0 {
		htmx.NotFound(w, gone)
		return false
	}
	return true
}
func finishRow(w http.ResponseWriter, r *http.Request, panel, label, gone string, result sql.Result, err error) bool {
	if !savedRow(w, panel, gone, result, err) {
		return false
	}
	htmx.Toast(w, label+" saved.")
	renderPanelBlock(w, r, panel, nil)
	return true
}
func marshalBonusPayloads(r *http.Request, grid string) (json.RawMessage, json.RawMessage, error) {
	entries := bonusEntriesFor(grid)
	misc := map[string]int{}
	states := map[string]string{}
	for _, entry := range entries {
		field := grid + "-" + entry.Key
		misc[entry.Key] = parseBonus(r.PostFormValue(field + "-misc"))
		states[entry.Key] = pages.NormalizeProficiency(r.PostFormValue(field + "-proficiency"))
	}
	encodedMisc, err := json.Marshal(misc)
	if err != nil {
		return nil, nil, err
	}
	encodedStates, err := json.Marshal(states)
	if err != nil {
		return nil, nil, err
	}
	return encodedMisc, encodedStates, nil
}
func bonusEntriesFor(grid string) []pages.BonusEntry {
	if grid == "skills" {
		return pages.SkillEntries()
	}
	return pages.SavingThrowEntries()
}
func (a *App) finishCharacterPanel(w http.ResponseWriter, r *http.Request, panel string, label string, result sql.Result, err error, characterID, ownerID ulid.ULID) {
	if !finishRow(w, r, panel, label, "character", result, err) {
		return
	}
	character, err := a.Queries.GetCharacter(r.Context(), queries.GetCharacterParams{ID: characterID, OwnerID: ownerID})
	if err != nil {
		slog.Error("Failed to read back derived values", "panel", panel, "error", err)
		return
	}
	derived := characterDerived(character)
	render(w, r, pages.DerivedValues(derived))
	render(w, r, pages.CharacterBarValues(characterHeaderFrom(character, derived)))
}
