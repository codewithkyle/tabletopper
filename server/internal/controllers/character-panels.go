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

// bonusPanels is the allowlist for the one handler left that takes its panel
// name from the URL. A segment that is not a key here never reaches a query or a
// field-name prefix. The value is the word the toast says, which is why this is
// a map and not a set: `saving_throws` is the wire format and "Saving throws" is
// the sentence.
//
// The repeaters had a second one of these until inventory replaced two of the
// three. One key is not an allowlist, so Features now has its own route and its
// own handler, and the word it says is written in that handler.
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

// SaveCharacterVitals owns the half of the sheet that changes during a fight:
// the hit points, what restores them, and what happens when they run out. It is
// not part of core-stats for two reasons -- that panel recomputes level and
// proficiency from xp on every save, which has no business happening because
// somebody ticked a death save, and these nine are the columns most likely to be
// written from a phone in the middle of a turn.
//
// TWO OF ITS CONTROLS ARE CHECKBOXES, which post nothing at all when they are
// not ticked, so this handler reads a value out of an absence -- the shape the
// rest of the panel handlers exist to avoid. It is safe here for the same
// reason it is safe in buildInventoryInput: the panel renders all nine controls
// together and posts as one form, so a missing field really is an unticked box
// rather than a partial request. A test pins that the panel keeps rendering
// them.
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

// The two panels nobody rolls. They are separate for the reason the queries
// behind them are: four boxes of prose and six words are not one panel, and the
// split is what lets the page put them side by side.
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

// SaveCharacterFeatures is the last repeater. The whole panel posts, not the row
// that changed: the wire format carries no index and this zips parallel slices
// in document order, so the set of rows in the request is the set of rows that
// will exist. That is also what makes deletion work -- repeater.js removes the
// row from the DOM and the next post simply does not contain it.
//
// It used to be SaveCharacterRows, taking the repeater's name from the path and
// looking it up in an allowlist, because Equipment and Resources came through
// here too. Inventory replaced both, and a path parameter with one legal value
// is a lookup that can only fail by accident.
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
		// The select's allowlist is also the column's, which is an ENUM. Sending
		// it a value it does not hold would be a 500 rather than a rejection, so
		// this normalises rather than validates -- there is nothing for a player
		// to correct, because the select could not have produced it.
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

// Every bound here is refused rather than clamped, and refused before the write.
// The three of them are also CHECK constraints on the table, so a value that got
// past this function would not be stored wrong -- it would be a 500. Rejecting
// here is what keeps that constraint unreachable, which is the only way a CHECK
// is worth having.
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

// deathSaves counts one row of bubbles. THE COUNT IS THE VALUE: three checkboxes
// share a name, an unticked one posts nothing, and the number on the wire is how
// many came back -- so there is no total to keep in sync with the boxes and no
// way for the two to disagree. A row longer than the rules allow can only come
// from a hand-built request, and it is refused rather than truncated so that a
// sheet never quietly saves something other than what was sent.
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

// The caps the details and vitals panels enforce, each in the unit its columns
// count. characterWordLimit is the width of every VARCHAR(64) on those panels --
// the six appearance fields and the hit dice pool.
// MySQL runs in strict mode, so an overlong value comes back from the driver as
// an error, and without these it would reach the player as a 500 on a box the
// sheet invited them to fill in.
//
// TEXT COUNTS BYTES AND VARCHAR COUNTS CHARACTERS, which is why one is measured
// with len and the other with a rune count. Neither is the column's own ceiling:
// TEXT holds 64 KB and 4 KB is already several pages of a bond, and the prose
// boxes carry maxlength="1024" -- 1024 UTF-16 units cannot encode to more than
// 3072 bytes, so that cap is reachable only by a request nobody's browser made.
// The word inputs carry their column's 64 as a maxlength, so that one is the
// same number in both places.
//
// The vitals bounds are not here. They are rules rather than column widths, the
// markup renders them into max attributes, and pages.DeathSaveLimit and its two
// neighbours are where they live so that both readings come from one number.
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

// The four read identically -- free prose, capped, never required -- so this is
// the loop buildAbilitiesInput is rather than four copies of the same lines.
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

// Six words, and the same loop. None is required and none is parsed: a height
// is "5 ft. 10 in." to one player and "178 cm" to the next, and an age that
// reads "300, and looks 25" is a fact about the character rather than a number
// the sheet failed to get.
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

// unknownBonusKind answers a {kind} segment that is not in bonusPanels. It took
// the name of the allowlist as an argument while the repeaters had one too; that
// went with them.
//
// Only a hand-built request reaches it -- every segment the editor sends is a
// constant in a template -- so the message says the page is wrong rather than
// the character is gone.
func unknownBonusKind(w http.ResponseWriter, kind string) {
	slog.Warn("unknown character bonus kind requested", "kind", kind)
	htmx.Error(w, "Not Found", "That part of the character sheet does not exist. Refresh the page and try again.", http.StatusNotFound)
}

// finishPanel is the tail every panel handler shares. Found-rows semantics (see
// database.Open) make zero matched rows mean the id is not this user's, not
// that the save changed nothing, so it is a 404 rather than a silent success.
//
// A save that lands answers with the toast and a cleared error block. label is
// the panel's name in prose, for the toast, and panel is its field prefix, for
// the block's id. Both are needed because the two disagree wherever the wire
// format is not a sentence: the saving throws panel is `saving_throws` in every
// id and field name and "Saving throws" in the message the user reads.
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

// marshalBonusPayloads builds the two blobs one bonus grid posts: the misc
// bonuses, `{"stealth": 2}`, and the proficiency states, `{"stealth":
// "expertise"}`. grid is the field-name prefix its inputs carry and is always
// one of the keys in bonusPanels.
//
// IT WALKS THE GRID'S OWN LIST RATHER THAN THE POSTED FORM, which is the
// difference between this and the marshalBonusesPayload it replaced. That one
// scanned the request for anything starting `skills-` and took whatever followed
// as a key, so a hand-built post could put `skills-flying` in the column and the
// sheet would carry it forever, unreachable from any control. Reading the
// eighteen keys the grid defines means a request can only answer the questions
// that were asked.
//
// A misc bonus that will not parse becomes 0 rather than an error: the inputs
// are type=number with a step, so a browser cannot submit anything else, and
// failing a whole panel save over a field the player cannot produce would cost
// more than it protects. A proficiency state gets the same treatment from
// NormalizeProficiency.
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

// bonusEntriesFor is the other half of bonusPanels: that map says which grid
// names are legal and what each is called in a sentence, and this says which
// rows each one has. The caller has already checked the name against the map, so
// there is no third answer to give.
func bonusEntriesFor(grid string) []pages.BonusEntry {
	if grid == "skills" {
		return pages.SkillEntries()
	}

	return pages.SavingThrowEntries()
}

// finishCharacterPanel is the tail every panel on the character editor shares.
// It answers the way finishPanel does and then appends two blocks of
// hx-swap-oob elements: the derived values -- ability modifiers, both grids'
// totals, passive perception and the two spell numbers -- and the bar across
// the top of the page, which carries the name, the subtitle and six readings.
// So a save updates the page that is already open rather than the next load of
// it.
//
// THE CHARACTER IS READ BACK rather than recomputed from what was just posted.
// That is what makes one refresh serve every panel: a skills save changes the
// skill totals, an abilities save changes every total on the sheet and two
// readings on the bar, and no handler knows enough on its own to say which.
//
// EVERY PANEL COMES THROUGH HERE, including the four that cannot change a
// derived value or a reading. That is the point. The alternative is a list of
// which panel refreshes what, kept by hand, and a panel added later that writes
// max_hp or a name would be correct in the database and stale on the screen
// until somebody noticed and remembered to add it. The cost of not keeping that
// list is one indexed lookup and a few dozen span swaps on a debounce that
// fires at most once a second.
//
// A read that fails is logged and dropped. The save landed, the toast is already
// queued, and a stale readout that a reload fixes is a smaller thing to hand
// somebody than an error over a write that worked.
func (a *App) finishCharacterPanel(w http.ResponseWriter, r *http.Request, panel string, label string, result sql.Result, err error, characterID, ownerID ulid.ULID) {
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

	character, err := a.Queries.GetCharacter(r.Context(), queries.GetCharacterParams{ID: characterID, OwnerID: ownerID})
	if err != nil {
		slog.Error("Failed to read back derived values", "panel", panel, "error", err)
		return
	}

	derived := characterDerived(character)
	render(w, r, pages.DerivedValues(derived))
	render(w, r, pages.CharacterBarValues(characterHeaderFrom(character, derived)))
}
