package controllers

// The character editor saves one panel at a time. Every handler below owns a
// disjoint set of columns and writes only those, through a query that names
// only those.
//
// THAT NARROWNESS IS LOAD-BEARING. The parse helpers in characters.go return
// their fallback on an empty string rather than an error, so reading a field
// the request never carried does not fail -- it yields a default. A handler
// that read the whole sheet would answer the Identity panel by writing 10 over
// every ability score, 1 over max_hp and current_hp, and empty JSON over all
// six blobs, and follow it with a toast saying it saved.
//
// NO READER IN THIS PACKAGE IS WIDE ENOUGH TO DO THAT, and that is now a
// property of the code rather than a rule to remember. There was one while
// creation was a page carrying the whole sheet behind a Save button. Creation is
// a name in a dialog now, and the statement it runs takes three values, so the
// only writes that reach a character are the ten below -- each naming its own
// columns -- and an insert that cannot carry a column at all.
//
// TestCreateCannotCarrySheetData is what holds that shut: a fourth value would
// have to appear in the statement and in the generated params, and it asserts
// against both.
//
// The panel builders are the other half. One per panel, reading only what its
// panel renders, so each rule has a single definition.

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

// bonusPanels and infoRowPanels are the allowlists for the two handlers that
// take the panel name from the URL. A segment that is not a key here never
// reaches a query or a field-name prefix. The value is the word the toast says.
var bonusPanels = map[string]string{
	"skills":        "Skills",
	"saving_throws": "Saving throws",
}

var infoRowPanels = map[string]string{
	"features":  "Features",
	"weapons":   "Equipment",
	"resources": "Resources",
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
	finishPanel(w, r, "identity", "Identity", result, err)
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
	finishPanel(w, r, "abilities", "Abilities", result, err)
}

// SaveCharacterCoreStats writes three columns the panel has no field for. level
// and proficiency_bonus follow from xp, which this panel owns, so
// buildCoreStatsInput derives both and the query sets all three at once -- the
// row never holds a level that disagrees with its experience.
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
		XP:               input.XP,
		Level:            input.Level,
		ProficiencyBonus: input.ProficiencyBonus,
		Speed:            input.Speed,
		AC:               input.AC,
		InitiativeBonus:  input.InitiativeBonus,
		MaxHP:            input.MaxHP,
		CurrentHP:        input.CurrentHP,
		TempHP:           input.TempHP,
		SpellSaveDC:      input.SpellSaveDC,
		SpellAtkBonus:    input.SpellAtkBonus,
		ID:               characterID,
		OwnerID:          sess.UserID,
	})
	finishPanel(w, r, "core-stats", "Core stats", result, err)
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
	finishPanel(w, r, "proficiencies", "Proficiencies", result, err)
}

// SaveCharacterBonuses serves both bonus grids. They differ only in the field
// prefix they emit and the column they land in, so one handler and one
// allowlist covers them; the switch is over the query, which stays static
// because sqlc has no other kind.
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
		unknownPanel(w, "bonus panel", kind)
		return
	}
	if !parsePanelForm(w, r, kind) {
		return
	}

	payload, err := marshalBonusesPayload(r, kind)
	if err != nil {
		slog.Error("Failed to encode bonuses", "kind", kind, "error", err)
		htmx.ServerError(w)
		return
	}

	var result sql.Result
	switch kind {
	case "skills":
		result, err = a.Queries.UpdateCharacterSkills(ctx, queries.UpdateCharacterSkillsParams{
			Skills:  payload,
			ID:      characterID,
			OwnerID: sess.UserID,
		})
	case "saving_throws":
		result, err = a.Queries.UpdateCharacterSavingThrows(ctx, queries.UpdateCharacterSavingThrowsParams{
			SavingThrows: payload,
			ID:           characterID,
			OwnerID:      sess.UserID,
		})
	}
	finishPanel(w, r, kind, label, result, err)
}

// SaveCharacterRows serves the three repeaters. The whole repeater posts, not
// the row that changed: the wire format carries no index and the controller
// zips parallel slices in document order, so the set of rows in the request is
// the set of rows that will exist. That is also what makes deletion work --
// repeater.js removes the row from the DOM and the next post simply does not
// contain it.
func (a *App) SaveCharacterRows(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}

	field := r.PathValue("field")
	label, ok := infoRowPanels[field]
	if !ok {
		unknownPanel(w, "repeater", field)
		return
	}
	if !parsePanelForm(w, r, field) {
		return
	}

	payload, err := marshalInfoRowsPayload(r, field)
	if err != nil {
		slog.Error("Failed to encode info rows", "field", field, "error", err)
		htmx.ServerError(w)
		return
	}

	var result sql.Result
	switch field {
	case "features":
		result, err = a.Queries.UpdateCharacterFeatures(ctx, queries.UpdateCharacterFeaturesParams{
			Features: payload,
			ID:       characterID,
			OwnerID:  sess.UserID,
		})
	case "weapons":
		result, err = a.Queries.UpdateCharacterWeapons(ctx, queries.UpdateCharacterWeaponsParams{
			Weapons: payload,
			ID:      characterID,
			OwnerID: sess.UserID,
		})
	case "resources":
		result, err = a.Queries.UpdateCharacterResources(ctx, queries.UpdateCharacterResourcesParams{
			Resources: payload,
			ID:        characterID,
			OwnerID:   sess.UserID,
		})
	}
	finishPanel(w, r, field, label, result, err)
}

// SaveCharacterSpells writes all ten levels on every save, because the column
// is one JSON object holding all ten and marshalSpellSlotsPayload rebuilds it
// from the post. The whole spellcasting section is therefore one panel and one
// form, not ten.
func (a *App) SaveCharacterSpells(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	if !parsePanelForm(w, r, "spells") {
		return
	}

	payload, err := marshalSpellSlotsPayload(r)
	if err != nil {
		slog.Error("Failed to encode spell slots", "error", err)
		htmx.ServerError(w)
		return
	}

	result, err := a.Queries.UpdateCharacterSpellSlots(ctx, queries.UpdateCharacterSpellSlotsParams{
		SpellSlots: payload,
		ID:         characterID,
		OwnerID:    sess.UserID,
	})
	finishPanel(w, r, "spells", "Spells", result, err)
}

// THE PANEL BUILDERS.
//
// One per panel, each reading only the fields its panel renders and returning
// only the errors those fields can raise. Nothing composes them into a reader of
// the whole sheet; that was the create page's, and a character is created from a
// name now.

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

	// The six read identically, so they are a loop over the field name and the
	// word the error uses rather than six copies of the same four lines.
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
	XP               uint32
	Level            uint8
	ProficiencyBonus uint16
	Speed            string
	AC               uint16
	InitiativeBonus  int16
	MaxHP            uint16
	CurrentHP        uint16
	TempHP           uint16
	SpellSaveDC      uint16
	SpellAtkBonus    int16
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

	initiativeBonus, err := parseInt16(r.PostFormValue("initiative_bonus"), 0)
	if err != nil {
		validationErrors = append(validationErrors, "Initiative bonus must be between -32768 and 32767.")
	}

	spellSaveDC, err := parseUint16(r.PostFormValue("spell_save_dc"), 10)
	if err != nil {
		validationErrors = append(validationErrors, "Spell save DC must be between 0 and 65535.")
	}

	spellAtkBonus, err := parseInt16(r.PostFormValue("spell_atk_bonus"), 0)
	if err != nil {
		validationErrors = append(validationErrors, "Spell attack bonus must be between -32768 and 32767.")
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
		MaxHP:            maxHP,
		CurrentHP:        currentHP,
		TempHP:           tempHP,
		SpellSaveDC:      spellSaveDC,
		SpellAtkBonus:    spellAtkBonus,
	}, validationErrors
}

type proficienciesInput struct {
	Languages     string
	Proficiencies string
}

// No errors to return: both fields are free text and neither is required.
// Languages carries a default because the column is NOT NULL and a sheet with
// no languages at all is a data-entry slip rather than a choice.
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

// THE SHARED TAIL.

// panelCharacterID reads the {id} segment. An unparseable one is answered as a
// missing character rather than a bad request: the only way to reach here with
// one is a stale page or a hand-edited URL, and both mean the same thing to the
// user.
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

// renderPanelBlock writes the panel's error block, which is what BOTH replies
// carry: the messages on a rejected save, and an empty block on an accepted
// one. The empty one is not busywork -- the panel targets this element, so
// rendering it clean is what clears a message the previous save put there.
// Answering a success with 204 instead would leave the complaint sitting over a
// panel that has since saved.
//
// The status follows from the messages. 422 is the one 4xx base.templ's noSwap
// config lets through, via the hx-status:422 on each panel.
func renderPanelBlock(w http.ResponseWriter, r *http.Request, panel string, messages []string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if len(messages) > 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}

	render(w, r, pages.PanelFormErrors(panel, messages))
}

// unknownPanel answers a URL segment that is not in one of the two allowlists.
// Only a hand-built request reaches it -- every segment the editor sends is a
// constant in a template -- so the message says the page is wrong rather than
// the character is gone.
func unknownPanel(w http.ResponseWriter, kind string, value string) {
	slog.Warn("unknown character panel requested", "kind", kind, "value", value)
	htmx.Error(w, "Not Found", "That part of the character sheet does not exist. Refresh the page and try again.", http.StatusNotFound)
}

// finishPanel is the tail every panel handler shares. Found-rows semantics (see
// database.Open) make zero matched rows mean the id is not this user's, not
// that the save changed nothing, so it is a 404 rather than a silent success.
//
// A save that lands answers with the toast and a cleared error block. label is
// the panel's name in prose, for the toast, and panel is its field prefix, for
// the block's id; they differ where the wire format and the sheet disagree, as
// with weapons and Equipment.
func finishPanel(w http.ResponseWriter, r *http.Request, panel string, label string, result sql.Result, err error) {
	if err != nil {
		slog.Error("Failed to save character panel", "panel", panel, "error", err)
		htmx.ServerError(w)
		return
	}

	if matched, err := result.RowsAffected(); err == nil && matched == 0 {
		htmx.NotFound(w, "character")
		return
	}

	htmx.Toast(w, label+" saved.")
	renderPanelBlock(w, r, panel, nil)
}

// marshalBonusesPayload builds the `{"str": 2, ...}` object one bonus grid
// posts. field is the name prefix its inputs carry and is always one of the
// keys in bonusPanels.
//
// A value that will not parse becomes 0 rather than an error: the inputs are
// type=number with a step, so a browser cannot submit anything else, and
// failing a whole panel save over a field the user cannot produce would cost
// more than it protects.
func marshalBonusesPayload(r *http.Request, field string) (json.RawMessage, error) {
	prefix := field + "-"
	values := map[string]int{}

	for key, formValues := range r.PostForm {
		if !strings.HasPrefix(key, prefix) {
			continue
		}

		bonusKey := strings.TrimPrefix(key, prefix)
		if bonusKey == "" || len(formValues) == 0 {
			continue
		}

		values[bonusKey] = parseBonus(formValues[0])
	}

	payload, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(payload), nil
}
