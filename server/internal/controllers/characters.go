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

// The editor's two pages. Both need the same character loaded and mapped the
// same way, and differ only in which panels they render, so the work is in
// loadCharacterForEdit and these are the two ways out of it.
//
// characterToEditPageData builds the whole struct for both. A spells page that
// reads only SpellLevels is not worth a second mapper -- the row is already in
// memory and the unused fields cost a few string conversions.
func (a *App) CharacterPage(w http.ResponseWriter, r *http.Request) {
	data, ok := a.loadCharacterForEdit(w, r)
	if !ok {
		return
	}

	render(w, r, pages.EditCharacter(data))
}

func (a *App) CharacterSpellsPage(w http.ResponseWriter, r *http.Request) {
	data, ok := a.loadCharacterForEdit(w, r)
	if !ok {
		return
	}

	render(w, r, pages.EditCharacterSpells(data))
}

// loadCharacterForEdit answers a page request, so every failure is a redirect
// rather than an alert: nothing is open yet to show one in. A character that is
// not this user's and one that never existed are the same miss, because the
// query is scoped to the owner.
func (a *App) loadCharacterForEdit(w http.ResponseWriter, r *http.Request) (pages.EditCharacterPageData, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	id := r.PathValue("id")
	if id == "" {
		redirect(w, r, "/characters")
		return pages.EditCharacterPageData{}, false
	}
	uid, err := ulid.Parse(id)
	if err != nil {
		redirect(w, r, "/characters")
		return pages.EditCharacterPageData{}, false
	}

	character, err := a.Queries.GetCharacter(ctx, queries.GetCharacterParams{
		ID:      uid,
		OwnerID: sess.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		redirect(w, r, "/characters")
		return pages.EditCharacterPageData{}, false
	}
	if err != nil {
		slog.Error("Failed to load character", "error", err)
		redirectToError(w, r)
		return pages.EditCharacterPageData{}, false
	}

	return characterToEditPageData(id, character), true
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

// The column is varchar(128), and MySQL counts characters there rather than
// bytes. So does this: len() on the string would reject a name of 90 accented
// letters that the database would have taken without complaint.
const characterNameLimit = 128

// NewCharacterForm creates a character from a name and sends the browser to the
// editor. This is the whole of creation now -- every other column is answered by
// the statement or by the schema, and the sheet is filled in afterwards a panel
// at a time by a page that saves as you go.
//
// It reads one field, and reads it here rather than through buildIdentityInput.
// That builder also requires `size`, which the dialog does not carry and should
// not grow a control for: a second question in a one-question dialog invites a
// third.
//
// The reply on success is a redirect with no body, so nothing lands back in the
// dialog -- the navigation takes it away. The toast still arrives, on the page
// after this one: toast.js parks a message in sessionStorage when the same
// response also carries HX-Redirect, because a message shown a moment before a
// navigation is never read.
// NewCharacterFragment serves the content of the new-character dialog: a
// heading, one field and a button. Like the other two fragments it reaches no
// database and carries nothing from the session, because the form it returns is
// the same for every user -- but it stays behind auth.Fragment all the same,
// since an unauthenticated route here would be surface for no reason.
func (a *App) NewCharacterFragment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.NewCharacterFragment())
}

func (a *App) NewCharacterForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	if err := r.ParseForm(); err != nil {
		rejectNewCharacter(w, r, "The submitted form data could not be read.")
		return
	}

	name := strings.TrimSpace(r.PostFormValue("name"))
	switch {
	case name == "":
		rejectNewCharacter(w, r, "Name is required.")
		return
	case len([]rune(name)) > characterNameLimit:
		rejectNewCharacter(w, r, "Name must be 128 characters or fewer.")
		return
	}

	id := ulid.Make()
	err := a.Queries.CreateCharacterFromName(ctx, queries.CreateCharacterFromNameParams{
		ID:      id,
		OwnerID: sess.UserID,
		Name:    name,
	})
	if err != nil {
		slog.Error("Failed to create character", "error", err)
		htmx.ServerError(w)
		return
	}

	htmx.Toast(w, name+" has been created.")
	htmx.Redirect(w, "/characters/"+id.String()+"/edit")
}

// rejectNewCharacter answers with the dialog's error block under a 422, which is
// the one code the form has an hx-status route for -- every other 4xx is in the
// noSwap list and would leave the dialog showing nothing new. The form is left
// alone, so the name the user typed is still in the field when the message
// appears above it.
func rejectNewCharacter(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnprocessableEntity)
	render(w, r, pages.PanelFormErrors(pages.NewCharacterPanel, []string{message}))
}

func characterToEditPageData(id string, character queries.Character) pages.EditCharacterPageData {
	return pages.EditCharacterPageData{
		CharacterID:     id,
		Name:            character.Name,
		Race:            nullStringValue(character.Race),
		Background:      nullStringValue(character.Background),
		Classes:         nullStringValue(character.Classes),
		Size:            fallbackString(strings.TrimSpace(character.Size), pages.DefaultSize),
		Alignment:       fallbackString(nullStringValue(character.Alignment), pages.DefaultAlignment),
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
// and it is what the form does with it on the next save anyway (parseBonus
// fails on "2.5" and falls back to 0).
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

// parseBonus reads one cell of a bonus grid. Unparseable is 0 rather than an
// error: the inputs are type=number with a step, so a browser cannot submit
// anything else, and the value is one of twenty-four on the panel.
func parseBonus(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}

	return parsed
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
