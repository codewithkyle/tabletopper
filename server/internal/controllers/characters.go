package controllers

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	db "main/internal/database"
	"main/internal/helpers"
	"main/internal/queries"
	"main/internal/session"
	"main/templ/pages"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/oklog/ulid/v2"
)

func CharacterPage(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	db, err := db.Connect()
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}
	session, err := session.GetUserSessionFromCookie(r, db, ctx)
	if err != nil {
		http.Redirect(w, r, "/sign-in", http.StatusTemporaryRedirect)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Redirect(w, r, "/characters", http.StatusTemporaryRedirect)
		return
	}
	uid, err := ulid.Parse(id)
	if err != nil {
		http.Redirect(w, r, "/characters", http.StatusSeeOther)
		return
	}

	q := queries.New(db)
	character, err := q.GetCharacterByIDAndOwner(ctx, queries.GetCharacterByIDAndOwnerParams{
		ID:      uid[:],
		OwnerID: session.UserId[:],
	})
	if err != nil {
		if err == sql.ErrNoRows {
			http.Redirect(w, r, "/characters", http.StatusSeeOther)
			return
		}

		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}

	data := characterToEditPageData(id, character)
	pages.EditCharacter(session, data).Render(r.Context(), w)
}

func EditCharacterForm(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	db, err := db.Connect()
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}
	session, err := session.GetUserSessionFromCookie(r, db, ctx)
	if err != nil {
		http.Redirect(w, r, "/sign-in", http.StatusTemporaryRedirect)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Redirect(w, r, "/characters", http.StatusTemporaryRedirect)
		return
	}
	uid, err := ulid.Parse(id)
	if err != nil {
		http.Redirect(w, r, "/characters", http.StatusSeeOther)
		return
	}

	err = r.ParseForm()
	if err != nil {
		if isHTMXRequest(r) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			pages.NewCharacterFormErrors([]string{"The submitted form data could not be read."}).Render(r.Context(), w)
			return
		}

		http.Redirect(w, r, "/characters/"+id, http.StatusSeeOther)
		return
	}

	formInput, validationErrors, err := buildCharacterFormInput(r)
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}

	if len(validationErrors) > 0 {
		if isHTMXRequest(r) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			pages.NewCharacterFormErrors(validationErrors).Render(r.Context(), w)
			return
		}

		http.Redirect(w, r, "/characters/"+id, http.StatusSeeOther)
		return
	}

	q := queries.New(db)
	result, err := q.UpdateCharacterByIDAndOwner(ctx, queries.UpdateCharacterByIDAndOwnerParams{
		Name:             formInput.Name,
		Level:            formInput.Level,
		Xp:               formInput.XP,
		Race:             formInput.Race,
		Background:       formInput.Background,
		Alignment:        formInput.Alignment,
		Classes:          formInput.Classes,
		Size:             formInput.Size,
		Ac:               formInput.AC,
		MaxHp:            formInput.MaxHP,
		CurrentHp:        formInput.CurrentHP,
		ProficiencyBonus: formInput.ProficiencyBonus,
		TempHp:           formInput.TempHP,
		Speed:            formInput.Speed,
		InitiativeBonus:  formInput.InitiativeBonus,
		SpellSaveDc:      formInput.SpellSaveDC,
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
		ID:               uid[:],
		OwnerID:          session.UserId[:],
	})
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}

	if rowsAffected == 0 {
		http.Redirect(w, r, "/characters", http.StatusSeeOther)
		return
	}

	if isHTMXRequest(r) {
		w.Header().Set("HX-Redirect", "/characters")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/characters", http.StatusSeeOther)
}

func CharactersPage(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	db, err := db.Connect()
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}
	session, err := session.GetUserSessionFromCookie(r, db, ctx)
	if err != nil {
		http.Redirect(w, r, "/sign-in", http.StatusTemporaryRedirect)
		return
	}

	q := queries.New(db)
	results, err := q.GetCharacters(ctx, session.UserId[:])
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}

	pages.Characters(session, results).Render(r.Context(), w)
}

func NewCharacterPage(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	db, err := db.Connect()
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}
	session, err := session.GetUserSessionFromCookie(r, db, ctx)
	if err != nil {
		http.Redirect(w, r, "/sign-in", http.StatusTemporaryRedirect)
		return
	}

	pages.NewCharacter(session).Render(r.Context(), w)
}

func NewCharacterForm(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	db, err := db.Connect()
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}
	session, err := session.GetUserSessionFromCookie(r, db, ctx)
	if err != nil {
		http.Redirect(w, r, "/sign-in", http.StatusTemporaryRedirect)
		return
	}

	err = r.ParseForm()
	if err != nil {
		if isHTMXRequest(r) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			pages.NewCharacterFormErrors([]string{"The submitted form data could not be read."}).Render(r.Context(), w)
			return
		}

		http.Redirect(w, r, "/characters/new", http.StatusSeeOther)
		return
	}

	formInput, validationErrors, err := buildCharacterFormInput(r)
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}

	if len(validationErrors) > 0 {
		if isHTMXRequest(r) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			pages.NewCharacterFormErrors(validationErrors).Render(r.Context(), w)
			return
		}

		http.Redirect(w, r, "/characters/new", http.StatusSeeOther)
		return
	}

	q := queries.New(db)
	id := ulid.Make()
	err = q.CreateCharacter(ctx, queries.CreateCharacterParams{
		ID:               id[:],
		OwnerID:          session.UserId[:],
		AssetID:          sql.NullString{},
		Name:             formInput.Name,
		Level:            formInput.Level,
		Xp:               formInput.XP,
		Race:             formInput.Race,
		Background:       formInput.Background,
		Alignment:        formInput.Alignment,
		Classes:          formInput.Classes,
		Size:             formInput.Size,
		Ac:               formInput.AC,
		MaxHp:            formInput.MaxHP,
		CurrentHp:        formInput.CurrentHP,
		ProficiencyBonus: formInput.ProficiencyBonus,
		TempHp:           formInput.TempHP,
		Speed:            formInput.Speed,
		InitiativeBonus:  formInput.InitiativeBonus,
		SpellSaveDc:      formInput.SpellSaveDC,
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
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}

	if isHTMXRequest(r) {
		w.Header().Set("HX-Redirect", "/characters")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/characters", http.StatusSeeOther)
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

	level := helpers.CalculateCharacterLevelFromXP(xp)
	proficiencyBonus := helpers.CalculateCharacterProficiencyBonus(level)

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
		FormAction:       "/characters/" + id,
		Name:             character.Name,
		Race:             nullStringValue(character.Race),
		Background:       nullStringValue(character.Background),
		Classes:          nullStringValue(character.Classes),
		Size:             fallbackString(strings.TrimSpace(character.Size), "medium"),
		Alignment:        fallbackString(nullStringValue(character.Alignment), "unaliged"),
		Xp:               strconv.FormatUint(uint64(character.Xp), 10),
		Languages:        fallbackString(strings.TrimSpace(character.Languages), "Common"),
		Proficiencies:    strings.TrimSpace(character.Proficiencies),
		Str:              strconv.FormatUint(uint64(character.Str), 10),
		Dex:              strconv.FormatUint(uint64(character.Dex), 10),
		Con:              strconv.FormatUint(uint64(character.Con), 10),
		Int:              strconv.FormatUint(uint64(character.Int), 10),
		Wis:              strconv.FormatUint(uint64(character.Wis), 10),
		Cha:              strconv.FormatUint(uint64(character.Cha), 10),
		Ac:               strconv.FormatUint(uint64(character.Ac), 10),
		Speed:            fallbackString(strings.TrimSpace(character.Speed), "30 ft."),
		InitiativeBonus:  strconv.FormatInt(int64(character.InitiativeBonus), 10),
		MaxHP:            strconv.FormatUint(uint64(character.MaxHp), 10),
		CurrentHP:        strconv.FormatUint(uint64(character.CurrentHp), 10),
		TempHP:           strconv.FormatUint(uint64(character.TempHp), 10),
		SpellSaveDC:      strconv.FormatUint(uint64(character.SpellSaveDc), 10),
		SpellAtkBonus:    strconv.FormatInt(int64(character.SpellAtkBonus), 10),
		SkillsJSON:       normalizeJSONObjectJSON(character.Skills),
		SavingThrowsJSON: normalizeJSONObjectJSON(character.SavingThrows),
		FeaturesJSON:     normalizeInfoRowsJSON(character.Features),
		WeaponsJSON:      normalizeInfoRowsJSON(character.Weapons),
		ResourcesJSON:    normalizeInfoRowsJSON(character.Resources),
		SpellSlotsJSON:   normalizeSpellSlotsJSON(character.SpellSlots),
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

func normalizeJSONObjectJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		slog.Warn("invalid JSON object payload; defaulting", "error", err)
		return "{}"
	}

	if payload == nil {
		payload = map[string]any{}
	}

	normalized, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("failed to normalize JSON object payload; defaulting", "error", err)
		return "{}"
	}

	return string(normalized)
}

func normalizeInfoRowsJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "[]"
	}

	var rows []infoRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		slog.Warn("invalid info rows payload; defaulting", "error", err)
		return "[]"
	}

	if rows == nil {
		rows = make([]infoRow, 0)
	}

	normalized, err := json.Marshal(rows)
	if err != nil {
		slog.Warn("failed to normalize info rows payload; defaulting", "error", err)
		return "[]"
	}

	return string(normalized)
}

func normalizeSpellSlotsJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}

	var payload map[string]spellLevelPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		slog.Warn("invalid spell slots payload; defaulting", "error", err)
		return "{}"
	}

	if payload == nil {
		payload = map[string]spellLevelPayload{}
	}

	normalized, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("failed to normalize spell slots payload; defaulting", "error", err)
		return "{}"
	}

	return string(normalized)
}

func isHTMXRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("HX-Request"), "true")
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

var spellSlotFieldPattern = regexp.MustCompile(`^spells-level-(\d+)-(slots|used)$`)
var spellEntryFieldPattern = regexp.MustCompile(`^spells-level-(\d+)-spell-(\d+)-(name|components|school|castingTime|range|duration|text)$`)

func marshalSpellSlotsPayload(r *http.Request) (json.RawMessage, error) {
	levels := map[string]*spellLevelPayload{}
	spellMap := map[int]map[int]*spellPayload{}

	for i := 0; i <= 9; i++ {
		key := strconv.Itoa(i)
		levels[key] = &spellLevelPayload{
			Level:  i,
			Slots:  0,
			Used:   0,
			Spells: []spellPayload{},
		}
		spellMap[i] = map[int]*spellPayload{}
	}

	for key, formValues := range r.PostForm {
		if len(formValues) == 0 {
			continue
		}

		value := strings.TrimSpace(formValues[0])

		if slotMatch := spellSlotFieldPattern.FindStringSubmatch(key); len(slotMatch) == 3 {
			level, err := strconv.Atoi(slotMatch[1])
			if err != nil || level < 0 || level > 9 {
				continue
			}

			parsed, err := strconv.Atoi(value)
			if err != nil {
				parsed = 0
			}

			if parsed < 0 {
				parsed = 0
			}

			entry := levels[strconv.Itoa(level)]
			if slotMatch[2] == "slots" {
				entry.Slots = parsed
			} else {
				entry.Used = parsed
			}

			continue
		}

		if spellMatch := spellEntryFieldPattern.FindStringSubmatch(key); len(spellMatch) == 4 {
			level, err := strconv.Atoi(spellMatch[1])
			if err != nil || level < 0 || level > 9 {
				continue
			}

			index, err := strconv.Atoi(spellMatch[2])
			if err != nil || index < 0 {
				continue
			}

			field := spellMatch[3]
			if spellMap[level][index] == nil {
				spellMap[level][index] = &spellPayload{}
			}

			spell := spellMap[level][index]
			switch field {
			case "name":
				spell.Name = value
			case "components":
				spell.Components = value
			case "school":
				spell.School = value
			case "castingTime":
				spell.CastingTime = value
			case "range":
				spell.Range = value
			case "duration":
				spell.Duration = value
			case "text":
				spell.Text = value
			}
		}
	}

	for level := 0; level <= 9; level++ {
		entry := levels[strconv.Itoa(level)]
		if entry.Used > entry.Slots {
			entry.Used = entry.Slots
		}

		indexes := make([]int, 0, len(spellMap[level]))
		for index := range spellMap[level] {
			indexes = append(indexes, index)
		}

		sort.Ints(indexes)
		for _, index := range indexes {
			spell := spellMap[level][index]
			if spell == nil {
				continue
			}

			if spell.Name == "" && spell.Components == "" && spell.School == "" && spell.CastingTime == "" && spell.Range == "" && spell.Duration == "" && spell.Text == "" {
				continue
			}

			entry.Spells = append(entry.Spells, *spell)
		}
	}

	payload, err := json.Marshal(levels)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(payload), nil
}
