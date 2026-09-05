package controllers

import (
	"database/sql"
	"encoding/json"
	"errors"
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

// DeleteCharacter removes the character, its avatar object and the avatar's
// asset row. Object first, rows after: the rows are the record that an
// object may exist, so they go only once R2 has confirmed it is gone. A
// failure to drop the asset row after the character is gone is logged and
// not reported, because the user's request has been honoured.
func (a *App) DeleteCharacter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "character")
		return
	}

	character, err := a.Queries.GetCharacterAsset(ctx, queries.GetCharacterAssetParams{
		ID:      characterID,
		OwnerID: sess.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		htmx.NotFound(w, "character")
		return
	}
	if err != nil {
		slog.Error("Failed to query character asset", "error", err)
		htmx.ServerError(w)
		return
	}

	if character.FilePath.Valid {
		if err := a.Storage.Delete(ctx, character.FilePath.String); err != nil {
			slog.Error("Failed to delete avatar object", "error", err)
			htmx.ServerError(w)
			return
		}
	}

	err = a.Queries.DeleteCharacter(ctx, queries.DeleteCharacterParams{
		ID:      characterID,
		OwnerID: sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to delete character", "error", err)
		htmx.ServerError(w)
		return
	}

	if character.AssetID != nil {
		err := a.Queries.DeleteAsset(ctx, queries.DeleteAssetParams{
			ID:      *character.AssetID,
			OwnerID: sess.UserID,
		})
		if err != nil {
			slog.Error("Failed to delete avatar asset row; leaving it behind", "error", err, "assetID", character.AssetID.String())
		}
	}

	htmx.Toast(w, character.Name+" has been deleted.")
}

func (a *App) CharacterPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	id := r.PathValue("id")
	if id == "" {
		redirect(w, r, "/characters")
		return
	}
	uid, err := ulid.Parse(id)
	if err != nil {
		redirect(w, r, "/characters")
		return
	}

	character, err := a.Queries.GetCharacter(ctx, queries.GetCharacterParams{
		ID:      uid,
		OwnerID: sess.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		redirect(w, r, "/characters")
		return
	}
	if err != nil {
		slog.Error("Failed to load character", "error", err)
		redirectToError(w, r)
		return
	}

	data := characterToEditPageData(id, character)
	render(w, r, pages.EditCharacter(data))
}

func (a *App) EditCharacterForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "character")
		return
	}

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		render(w, r, pages.NewCharacterFormErrors([]string{"The submitted form data could not be read."}))
		return
	}

	formInput, validationErrors, err := buildCharacterFormInput(r)
	if err != nil {
		slog.Error("Failed to read character form", "error", err)
		htmx.ServerError(w)
		return
	}

	if len(validationErrors) > 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		render(w, r, pages.NewCharacterFormErrors(validationErrors))
		return
	}

	result, err := a.Queries.UpdateCharacter(ctx, queries.UpdateCharacterParams{
		Name:             formInput.Name,
		Level:            formInput.Level,
		XP:               formInput.XP,
		Race:             formInput.Race,
		Background:       formInput.Background,
		Alignment:        formInput.Alignment,
		Classes:          formInput.Classes,
		Size:             formInput.Size,
		AC:               formInput.AC,
		MaxHP:            formInput.MaxHP,
		CurrentHP:        formInput.CurrentHP,
		ProficiencyBonus: formInput.ProficiencyBonus,
		TempHP:           formInput.TempHP,
		Speed:            formInput.Speed,
		InitiativeBonus:  formInput.InitiativeBonus,
		SpellSaveDC:      formInput.SpellSaveDC,
		SpellAtkBonus:    formInput.SpellAtkBonus,
		Str:              formInput.Str,
		Dex:              formInput.Dex,
		Con:              formInput.Con,
		Int:              formInput.Int,
		Wis:              formInput.Wis,
		Cha:              formInput.Cha,
		Languages:        formInput.Languages,
		Proficiencies:    formInput.Proficiencies,
		Skills:           formInput.Skills,
		SavingThrows:     formInput.SavingThrows,
		Features:         formInput.Features,
		Weapons:          formInput.Weapons,
		SpellSlots:       formInput.SpellSlots,
		Resources:        formInput.Resources,
		ID:               characterID,
		OwnerID:          sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to update character", "error", err)
		htmx.ServerError(w)
		return
	}
	// Found-rows semantics (see database.Open), so zero means the id is not
	// this user's rather than that nothing changed.
	if matched, err := result.RowsAffected(); err == nil && matched == 0 {
		htmx.NotFound(w, "character")
		return
	}

	slog.Info("Character updated", "characterID", characterID.String())
	htmx.Toast(w, formInput.Name+" has been updated.")
	htmx.Redirect(w, "/characters")
}

func (a *App) CharactersPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	results, err := a.Queries.GetCharacters(ctx, sess.UserID)
	if err != nil {
		slog.Error("Failed to load characters", "error", err)
		redirectToError(w, r)
		return
	}

	render(w, r, pages.Characters(results))
}

func (a *App) NewCharacterPage(w http.ResponseWriter, r *http.Request) {
	render(w, r, pages.NewCharacter())
}

// InfoRowFragment serves one blank repeater row to the add buttons on the
// character forms. It is the whole server side of the add-row mechanic: no
// database, no session data in the response, just the same templ component the
// initial page render uses, so a row is defined in exactly one place.
//
// The field prefix decides the name attributes the row emits, so it is checked
// against the three known repeaters rather than trusted. An unknown prefix is a
// 404 and not a row carrying arbitrary field names.
//
// Behind middleware.Fragment, which is RequireSession plus the no-store and
// noindex headers every /fragment/ route owes its caller. The markup is not
// secret -- it holds nothing but empty fields -- but an unauthenticated
// endpoint here would be surface for no reason.
func (a *App) InfoRowFragment(w http.ResponseWriter, r *http.Request) {
	field := r.URL.Query().Get("field")
	if !pages.IsInfoRowField(field) {
		slog.Warn("unknown info row field requested", "field", field)
		// Status only. http.NotFound would write Go's plain-text 404 page into
		// a response the caller is going to swap, and a fragment route should
		// never hand back something page-shaped. noSwap covers 4xx, so htmx
		// leaves the target alone either way -- this is about the contract.
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.InfoRowFragment(field))
}

// SpellCardFragment serves one blank spell card to a level's add button, the
// Phase 8 counterpart of InfoRowFragment above.
//
// `level` is the only thing the client sends, and it is the whole reason the
// spell wire format moved off per-spell indices: an index would have to be a
// counter the client keeps, where a level is a constant baked into the button.
// It is bounded to the ten levels that exist rather than trusted -- it lands in
// every field name on the card, so an unchecked value would put arbitrary keys
// into the next post.
func (a *App) SpellCardFragment(w http.ResponseWriter, r *http.Request) {
	level, err := strconv.Atoi(r.URL.Query().Get("level"))
	if err != nil || level < 0 || level > 9 {
		slog.Warn("invalid spell level requested", "level", r.URL.Query().Get("level"))
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.SpellCardFragment(level))
}

func (a *App) NewCharacterForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		render(w, r, pages.NewCharacterFormErrors([]string{"The submitted form data could not be read."}))
		return
	}

	formInput, validationErrors, err := buildCharacterFormInput(r)
	if err != nil {
		slog.Error("Failed to read character form", "error", err)
		htmx.ServerError(w)
		return
	}

	if len(validationErrors) > 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		render(w, r, pages.NewCharacterFormErrors(validationErrors))
		return
	}

	id := ulid.Make()
	err = a.Queries.CreateCharacter(ctx, queries.CreateCharacterParams{
		ID:               id,
		OwnerID:          sess.UserID,
		Name:             formInput.Name,
		Level:            formInput.Level,
		XP:               formInput.XP,
		Race:             formInput.Race,
		Background:       formInput.Background,
		Alignment:        formInput.Alignment,
		Classes:          formInput.Classes,
		Size:             formInput.Size,
		AC:               formInput.AC,
		MaxHP:            formInput.MaxHP,
		CurrentHP:        formInput.CurrentHP,
		ProficiencyBonus: formInput.ProficiencyBonus,
		TempHP:           formInput.TempHP,
		Speed:            formInput.Speed,
		InitiativeBonus:  formInput.InitiativeBonus,
		SpellSaveDC:      formInput.SpellSaveDC,
		SpellAtkBonus:    formInput.SpellAtkBonus,
		Str:              formInput.Str,
		Dex:              formInput.Dex,
		Con:              formInput.Con,
		Int:              formInput.Int,
		Wis:              formInput.Wis,
		Cha:              formInput.Cha,
		Languages:        formInput.Languages,
		Proficiencies:    formInput.Proficiencies,
		Skills:           formInput.Skills,
		SavingThrows:     formInput.SavingThrows,
		Features:         formInput.Features,
		Weapons:          formInput.Weapons,
		SpellSlots:       formInput.SpellSlots,
		Resources:        formInput.Resources,
		Notes:            "",
	})
	if err != nil {
		slog.Error("Failed to create character", "error", err)
		htmx.ServerError(w)
		return
	}

	htmx.Toast(w, formInput.Name+" has been created.")
	htmx.Redirect(w, "/characters")
}

type characterFormInput struct {
	Name             string
	Level            uint8
	XP               uint32
	Race             sql.NullString
	Background       sql.NullString
	Alignment        sql.NullString
	Classes          sql.NullString
	Size             string
	AC               uint16
	MaxHP            uint16
	CurrentHP        uint16
	ProficiencyBonus uint16
	TempHP           uint16
	Speed            string
	InitiativeBonus  int16
	SpellSaveDC      uint16
	SpellAtkBonus    int16
	Str              uint8
	Dex              uint8
	Con              uint8
	Int              uint8
	Wis              uint8
	Cha              uint8
	Languages        string
	Proficiencies    string
	Skills           json.RawMessage
	SavingThrows     json.RawMessage
	Features         json.RawMessage
	Weapons          json.RawMessage
	SpellSlots       json.RawMessage
	Resources        json.RawMessage
}

func buildCharacterFormInput(r *http.Request) (characterFormInput, []string, error) {
	validationErrors := make([]string, 0)

	name := strings.TrimSpace(r.PostFormValue("name"))
	if name == "" {
		validationErrors = append(validationErrors, "Name is required.")
	}

	size := strings.TrimSpace(r.PostFormValue("size"))
	if size == "" {
		validationErrors = append(validationErrors, "Size is required.")
	}

	xp, xpErr := parseUint32(r.PostFormValue("xp"), 0)
	if xpErr != nil {
		validationErrors = append(validationErrors, "XP must be a valid non-negative number.")
	}

	strVal, strErr := parseUint8(r.PostFormValue("str"), 10)
	if strErr != nil {
		validationErrors = append(validationErrors, "Strength must be between 0 and 255.")
	}

	dexVal, dexErr := parseUint8(r.PostFormValue("dex"), 10)
	if dexErr != nil {
		validationErrors = append(validationErrors, "Dexterity must be between 0 and 255.")
	}

	conVal, conErr := parseUint8(r.PostFormValue("con"), 10)
	if conErr != nil {
		validationErrors = append(validationErrors, "Constitution must be between 0 and 255.")
	}

	intVal, intErr := parseUint8(r.PostFormValue("int"), 10)
	if intErr != nil {
		validationErrors = append(validationErrors, "Intelligence must be between 0 and 255.")
	}

	wisVal, wisErr := parseUint8(r.PostFormValue("wis"), 10)
	if wisErr != nil {
		validationErrors = append(validationErrors, "Wisdom must be between 0 and 255.")
	}

	chaVal, chaErr := parseUint8(r.PostFormValue("cha"), 10)
	if chaErr != nil {
		validationErrors = append(validationErrors, "Charisma must be between 0 and 255.")
	}

	ac, acErr := parseUint16(r.PostFormValue("ac"), 10)
	if acErr != nil {
		validationErrors = append(validationErrors, "Armor class must be between 0 and 65535.")
	}

	maxHP, maxHPErr := parseUint16(r.PostFormValue("max_hp"), 1)
	if maxHPErr != nil {
		validationErrors = append(validationErrors, "Max hit points must be between 0 and 65535.")
	}

	currentHP, currentHPErr := parseUint16(r.PostFormValue("current_hp"), 1)
	if currentHPErr != nil {
		validationErrors = append(validationErrors, "Hit points must be between 0 and 65535.")
	}

	tempHP, tempHPErr := parseUint16(r.PostFormValue("temp_hp"), 0)
	if tempHPErr != nil {
		validationErrors = append(validationErrors, "Temp hit points must be between 0 and 65535.")
	}

	initiativeBonus, initiativeErr := parseInt16(r.PostFormValue("initiative_bonus"), 0)
	if initiativeErr != nil {
		validationErrors = append(validationErrors, "Initiative bonus must be between -32768 and 32767.")
	}

	spellSaveDC, spellSaveErr := parseUint16(r.PostFormValue("spell_save_dc"), 10)
	if spellSaveErr != nil {
		validationErrors = append(validationErrors, "Spell save DC must be between 0 and 65535.")
	}

	spellAtkBonus, spellAtkErr := parseInt16(r.PostFormValue("spell_atk_bonus"), 0)
	if spellAtkErr != nil {
		validationErrors = append(validationErrors, "Spell attack bonus must be between -32768 and 32767.")
	}

	rawSkills, err := marshalSkillsPayload(r)
	if err != nil {
		return characterFormInput{}, nil, err
	}

	rawSavingThrows, err := marshalSavingThrowsPayload(r)
	if err != nil {
		return characterFormInput{}, nil, err
	}

	rawFeatures, err := marshalInfoRowsPayload(r, "features")
	if err != nil {
		return characterFormInput{}, nil, err
	}

	rawWeapons, err := marshalInfoRowsPayload(r, "weapons")
	if err != nil {
		return characterFormInput{}, nil, err
	}

	rawResources, err := marshalInfoRowsPayload(r, "resources")
	if err != nil {
		return characterFormInput{}, nil, err
	}

	rawSpellSlots, err := marshalSpellSlotsPayload(r)
	if err != nil {
		return characterFormInput{}, nil, err
	}

	level := levelFromXP(xp)
	proficiencyBonus := proficiencyBonusForLevel(level)

	speed := strings.TrimSpace(r.PostFormValue("speed"))
	if speed == "" {
		speed = "30 ft."
	}

	languages := strings.TrimSpace(r.PostFormValue("languages"))
	if languages == "" {
		languages = "Common"
	}

	proficiencies := strings.TrimSpace(r.PostFormValue("proficiencies"))

	return characterFormInput{
		Name:             name,
		Level:            level,
		XP:               xp,
		Race:             nullableString(r.PostFormValue("race")),
		Background:       nullableString(r.PostFormValue("background")),
		Alignment:        nullableString(r.PostFormValue("alignment")),
		Classes:          nullableString(r.PostFormValue("classes")),
		Size:             size,
		AC:               ac,
		MaxHP:            maxHP,
		CurrentHP:        currentHP,
		ProficiencyBonus: proficiencyBonus,
		TempHP:           tempHP,
		Speed:            speed,
		InitiativeBonus:  initiativeBonus,
		SpellSaveDC:      spellSaveDC,
		SpellAtkBonus:    spellAtkBonus,
		Str:              strVal,
		Dex:              dexVal,
		Con:              conVal,
		Int:              intVal,
		Wis:              wisVal,
		Cha:              chaVal,
		Languages:        languages,
		Proficiencies:    proficiencies,
		Skills:           rawSkills,
		SavingThrows:     rawSavingThrows,
		Features:         rawFeatures,
		Weapons:          rawWeapons,
		SpellSlots:       rawSpellSlots,
		Resources:        rawResources,
	}, validationErrors, nil
}

func characterToEditPageData(id string, character queries.Character) pages.EditCharacterPageData {
	return pages.EditCharacterPageData{
		FormAction:      "/characters/" + id,
		Name:            character.Name,
		Race:            nullStringValue(character.Race),
		Background:      nullStringValue(character.Background),
		Classes:         nullStringValue(character.Classes),
		Size:            fallbackString(strings.TrimSpace(character.Size), "medium"),
		Alignment:       fallbackString(nullStringValue(character.Alignment), "unaliged"),
		XP:              strconv.FormatUint(uint64(character.XP), 10),
		Languages:       fallbackString(strings.TrimSpace(character.Languages), "Common"),
		Proficiencies:   strings.TrimSpace(character.Proficiencies),
		Str:             strconv.FormatUint(uint64(character.Str), 10),
		Dex:             strconv.FormatUint(uint64(character.Dex), 10),
		Con:             strconv.FormatUint(uint64(character.Con), 10),
		Int:             strconv.FormatUint(uint64(character.Int), 10),
		Wis:             strconv.FormatUint(uint64(character.Wis), 10),
		Cha:             strconv.FormatUint(uint64(character.Cha), 10),
		AC:              strconv.FormatUint(uint64(character.AC), 10),
		Speed:           fallbackString(strings.TrimSpace(character.Speed), "30 ft."),
		InitiativeBonus: strconv.FormatInt(int64(character.InitiativeBonus), 10),
		MaxHP:           strconv.FormatUint(uint64(character.MaxHP), 10),
		CurrentHP:       strconv.FormatUint(uint64(character.CurrentHP), 10),
		TempHP:          strconv.FormatUint(uint64(character.TempHP), 10),
		SpellSaveDC:     strconv.FormatUint(uint64(character.SpellSaveDC), 10),
		SpellAtkBonus:   strconv.FormatInt(int64(character.SpellAtkBonus), 10),
		Skills:          parseStatBonuses(character.Skills),
		SavingThrows:    parseStatBonuses(character.SavingThrows),
		Features:        parseInfoRows(character.Features),
		Weapons:         parseInfoRows(character.Weapons),
		Resources:       parseInfoRows(character.Resources),
		SpellLevels:     parseSpellLevels(character.SpellSlots),
	}
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}

	return strings.TrimSpace(value.String)
}

func fallbackString(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}

	return trimmed
}

// parseStatBonuses unmarshals one of the `{"str": 2, "dex": 0, ...}` blobs into a
// map the templates can range over, rather than handing the JSON to the browser
// for a component to parse. It serves both bonus grids -- saving throws and
// skills -- and replaced normalizeJSONObjectJSON, which did the handing-over and
// went with the second of them.
//
// Decoded as float64, not int: type=number with step="1" rejects a decimal on
// validity but still reports it as the value, so a fractional bonus could reach
// the column and would fail a map[string]int unmarshal outright -- defaulting
// every one of the six to 0. Truncating the one bad entry is the smaller loss,
// and it is what the form does with it on the next save anyway (Atoi fails on
// "2.5" and marshalSavingThrowsPayload falls back to 0).
func parseStatBonuses(raw json.RawMessage) map[string]int {
	bonuses := map[string]int{}
	if len(raw) == 0 {
		return bonuses
	}

	var payload map[string]float64
	if err := json.Unmarshal(raw, &payload); err != nil {
		slog.Warn("invalid stat bonus payload; defaulting", "error", err)
		return bonuses
	}

	for key, value := range payload {
		bonuses[key] = int(value)
	}

	return bonuses
}

// parseInfoRows unmarshals a stored `[{"name": ..., "value": ...}]` column into
// the slice the templates range over. It replaced normalizeInfoRowsJSON, which
// re-marshalled the same rows back into a string for a data-rows attribute so
// monster-info-table.js could parse them a second time in the browser.
//
// A malformed column yields an empty slice and a warning, which is what the old
// function did and what the component did on top of it -- the repeater renders
// with no rows and the add button still works, rather than the page failing.
func parseInfoRows(raw json.RawMessage) []pages.InfoRow {
	rows := []pages.InfoRow{}
	if len(raw) == 0 {
		return rows
	}

	if err := json.Unmarshal(raw, &rows); err != nil {
		slog.Warn("invalid info rows payload; defaulting", "error", err)
		return []pages.InfoRow{}
	}

	if rows == nil {
		rows = []pages.InfoRow{}
	}

	return rows
}

// parseSpellLevels unmarshals the stored `{"0": {...}, "1": {...}}` column into
// the ten ordered blocks the templates range over. It replaced
// normalizeSpellSlotsJSON, which re-marshalled the same map into a string for a
// data-levels attribute so spell-slots-table.js could parse it again and do this
// same filling-in client-side.
//
// The column is keyed by level as a string and may be missing levels, so the
// ten are built here rather than trusted from storage. Clamping mirrors the
// write path: 0-99 for each counter, used never above slots. An unrecognised
// school falls back to the default, which is what the component did on load --
// it is the only place a legacy row can carry a value the picker cannot show.
func parseSpellLevels(raw json.RawMessage) []pages.SpellLevel {
	stored := map[string]pages.SpellLevel{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &stored); err != nil {
			slog.Warn("invalid spell slots payload; defaulting", "error", err)
			stored = map[string]pages.SpellLevel{}
		}
	}

	levels := make([]pages.SpellLevel, 0, 10)
	for level := 0; level <= 9; level++ {
		entry := stored[strconv.Itoa(level)]
		entry.Level = level
		entry.Slots = clampSpellCount(entry.Slots)
		entry.Used = clampSpellCount(entry.Used)
		if entry.Used > entry.Slots {
			entry.Used = entry.Slots
		}

		if entry.Spells == nil {
			entry.Spells = []pages.Spell{}
		}
		for i := range entry.Spells {
			entry.Spells[i].School = pages.NormalizeSpellSchool(entry.Spells[i].School)
		}

		levels = append(levels, entry)
	}

	return levels
}

// clampSpellCount holds a slot counter to the 0-99 spell-slots-table.js enforced
// as you typed. The fields carry min="0" max="99", so reaching this with an
// out-of-range value means the post did not come from the form.
func clampSpellCount(value int) int {
	if value < 0 {
		return 0
	}
	if value > 99 {
		return 99
	}

	return value
}

func parseSpellCount(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}

	return clampSpellCount(parsed)
}

func nullableString(value string) sql.NullString {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return sql.NullString{}
	}

	return sql.NullString{String: trimmed, Valid: true}
}

func parseUint32(value string, fallback uint32) (uint32, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseUint(trimmed, 10, 32)
	if err != nil {
		return fallback, err
	}

	return uint32(parsed), nil
}

func parseUint16(value string, fallback uint16) (uint16, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseUint(trimmed, 10, 16)
	if err != nil {
		return fallback, err
	}

	return uint16(parsed), nil
}

func parseUint8(value string, fallback uint8) (uint8, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseUint(trimmed, 10, 8)
	if err != nil {
		return fallback, err
	}

	return uint8(parsed), nil
}

func parseInt16(value string, fallback int16) (int16, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseInt(trimmed, 10, 16)
	if err != nil {
		return fallback, err
	}

	return int16(parsed), nil
}

func marshalSkillsPayload(r *http.Request) (json.RawMessage, error) {
	values := map[string]int{}
	for key, formValues := range r.PostForm {
		if !strings.HasPrefix(key, "skills-") {
			continue
		}

		skillKey := strings.TrimPrefix(key, "skills-")
		if skillKey == "" || len(formValues) == 0 {
			continue
		}

		parsed, err := strconv.Atoi(strings.TrimSpace(formValues[0]))
		if err != nil {
			parsed = 0
		}

		values[skillKey] = parsed
	}

	payload, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(payload), nil
}

func marshalSavingThrowsPayload(r *http.Request) (json.RawMessage, error) {
	values := map[string]int{}
	for key, formValues := range r.PostForm {
		if !strings.HasPrefix(key, "saving_throws-") {
			continue
		}

		saveKey := strings.TrimPrefix(key, "saving_throws-")
		if saveKey == "" || len(formValues) == 0 {
			continue
		}

		parsed, err := strconv.Atoi(strings.TrimSpace(formValues[0]))
		if err != nil {
			parsed = 0
		}

		values[saveKey] = parsed
	}

	payload, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(payload), nil
}

type infoRow struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func marshalInfoRowsPayload(r *http.Request, field string) (json.RawMessage, error) {
	names := r.PostForm[field+"-name"]
	values := r.PostForm[field+"-value"]
	rows := make([]infoRow, 0)

	count := len(names)
	if len(values) < count {
		count = len(values)
	}

	for i := 0; i < count; i++ {
		name := strings.TrimSpace(names[i])
		value := strings.TrimSpace(values[i])
		if name == "" && value == "" {
			continue
		}

		rows = append(rows, infoRow{Name: name, Value: value})
	}

	payload, err := json.Marshal(rows)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(payload), nil
}

type spellPayload struct {
	Name        string `json:"name"`
	Components  string `json:"components"`
	School      string `json:"school"`
	CastingTime string `json:"castingTime"`
	Range       string `json:"range"`
	Duration    string `json:"duration"`
	Text        string `json:"text"`
}

type spellLevelPayload struct {
	Level  int            `json:"level"`
	Slots  int            `json:"slots"`
	Used   int            `json:"used"`
	Spells []spellPayload `json:"spells"`
}

// marshalSpellSlotsPayload reads the spellcasting section back off the form.
//
// The names are order-based as of Phase 8: every card in a level posts the same
// seven keys, so PostForm["spells-level-3-name"] is that level's spell names in
// document order and the seven slices zip. Deleting a card removes its entry
// from all seven at once and adding one appends to all seven, so nothing has to
// renumber and no index has to be tracked anywhere.
//
// That replaced two regexes, a map[int]map[int]*spellPayload and a sort.Ints:
// the indexed format allowed gaps, which meant collecting spells into a sparse
// map and sorting the keys to recover document order. Reading the slices gives
// that order directly.
//
// The `-slots` and `-used` keys are per-level singletons and are unchanged. They
// cannot collide with the seven card keys -- none of those is named slots or
// used -- which is why the level prefix is safe to share.
func marshalSpellSlotsPayload(r *http.Request) (json.RawMessage, error) {
	levels := map[string]spellLevelPayload{}

	for level := 0; level <= 9; level++ {
		key := strconv.Itoa(level)
		prefix := "spells-level-" + key

		slots := parseSpellCount(r.PostFormValue(prefix + "-slots"))
		used := parseSpellCount(r.PostFormValue(prefix + "-used"))
		if used > slots {
			used = slots
		}

		levels[key] = spellLevelPayload{
			Level:  level,
			Slots:  slots,
			Used:   used,
			Spells: zipSpellPayloads(r, prefix),
		}
	}

	payload, err := json.Marshal(levels)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(payload), nil
}

// zipSpellPayloads pairs one level's seven slices by position, stopping at the
// shortest so a truncated post cannot index out of range.
//
// THE EMPTY-CARD GUARD DELIBERATELY IGNORES SCHOOL. The picker has no empty
// option, so every card posts one -- which meant the old guard's
// `spell.School == ""` was never true and a blank card was always stored. It was
// unreachable for a second reason too: the name carried `required`, so a blank
// card blocked the save outright rather than being dropped. Both are gone as of
// Phase 8, and a card now counts as empty when everything the user could
// actually have typed is empty. The school is a default, not an answer.
func zipSpellPayloads(r *http.Request, prefix string) []spellPayload {
	names := r.PostForm[prefix+"-name"]
	components := r.PostForm[prefix+"-components"]
	schools := r.PostForm[prefix+"-school"]
	castingTimes := r.PostForm[prefix+"-castingTime"]
	ranges := r.PostForm[prefix+"-range"]
	durations := r.PostForm[prefix+"-duration"]
	texts := r.PostForm[prefix+"-text"]

	count := len(names)
	for _, other := range [][]string{components, schools, castingTimes, ranges, durations, texts} {
		if len(other) < count {
			count = len(other)
		}
	}

	spells := make([]spellPayload, 0, count)
	for i := 0; i < count; i++ {
		spell := spellPayload{
			Name:        strings.TrimSpace(names[i]),
			Components:  strings.TrimSpace(components[i]),
			School:      pages.NormalizeSpellSchool(strings.TrimSpace(schools[i])),
			CastingTime: strings.TrimSpace(castingTimes[i]),
			Range:       strings.TrimSpace(ranges[i]),
			Duration:    strings.TrimSpace(durations[i]),
			Text:        strings.TrimSpace(texts[i]),
		}

		if spell.Name == "" && spell.Components == "" && spell.CastingTime == "" &&
			spell.Range == "" && spell.Duration == "" && spell.Text == "" {
			continue
		}

		spells = append(spells, spell)
	}

	return spells
}
