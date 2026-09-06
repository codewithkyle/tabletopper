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

// DeleteCharacter removes the character, its journal, its avatar object and the
// avatar's asset row. Object first, rows after: the rows are the record that an
// object may exist, so they go only once R2 has confirmed it is gone. A
// failure to drop the asset row after the character is gone is logged and
// not reported, because the user's request has been honoured.
//
// THE JOURNAL GOES FIRST, AND ITS IMAGES ARE HANDED TO THE SWEEPER. There are
// no foreign keys in this schema, so nothing cascades and every table has to be
// named here. Detaching the images rather than deleting them is what keeps this
// a handful of statements for a character with a long journal: an entry with
// forty pictures costs the same as an empty one, and internal/sweep deletes the
// objects a day later.
//
// Inventory, spells and spell_slots rows are still left behind. That is a known
// gap rather than a decision, and the two statements below are the pattern to
// follow when it is closed.
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

	// BEFORE DeleteCharacterJournals, because it finds the images through the
	// journals rows that statement removes -- and both before the character
	// row goes, which is the ordering that matters if one of them fails.
	// Nothing cascades in this schema, so a journal left behind by a character
	// that is already gone is unreachable: no page can open it, and no later
	// delete will ever retry it. Going the other way round risks the smaller
	// failure instead -- a character still listed whose journal is empty --
	// which the reader can clear by deleting it again.
	//
	// So either error stops here rather than carrying on.
	err = a.Queries.DetachCharacterJournalImages(ctx, queries.DetachCharacterJournalImagesParams{
		OwnerID:     sess.UserID,
		CharacterID: characterID,
	})
	if err != nil {
		slog.Error("Failed to detach character journal images", "error", err)
		htmx.ServerError(w)
		return
	}

	err = a.Queries.DeleteCharacterJournals(ctx, queries.DeleteCharacterJournalsParams{
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to delete character journals", "error", err)
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

// CharacterPage is the Character tab, and the only page that renders the
// characters row itself -- the other two tabs are views of the inventory and
// spells tables and take their own page data.
//
// It is also the only editor page that reads other tables, and it reads four of
// them. Equipment is the inventory rows ticked as equipped and Prepared Spells
// is the spell rows ticked as prepared -- both views, so a character's gear and
// their spells are written down once, on the tabs that own them, and read here
// rather than typed in twice. Only the ticked rows are fetched, by queries that
// filter in SQL: the page shows three of forty and has no use for the rest.
//
// Spell Slots is not a view. It is ten small forms writing the spell_slots
// table, and it is here rather than on the spells tab because resetting `used`
// after a long rest touches nine levels at once -- the one thing a page per
// level cannot do.
//
// Five queries, then, for a page that used to be one row. Each is an indexed
// lookup returning at most a handful of rows, and the alternative to the last
// two is a FULL OUTER JOIN that MySQL does not have.
func (a *App) CharacterPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	character, characterID, ok := a.loadCharacter(w, r)
	if !ok {
		return
	}

	equipped, err := a.Queries.ListEquippedInventory(ctx, queries.ListEquippedInventoryParams{
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to load equipped inventory", "error", err)
		redirectToError(w, r)
		return
	}

	prepared, err := a.Queries.ListPreparedSpells(ctx, queries.ListPreparedSpellsParams{
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to load prepared spells", "error", err)
		redirectToError(w, r)
		return
	}

	levels, ok := a.loadSpellLevels(w, r, characterID, sess.UserID)
	if !ok {
		return
	}

	data := characterToEditPageData(characterID.String(), character)
	data.Equipped = inventoryPageItems(equipped)
	data.Prepared = preparedSpellGroups(prepared)
	data.SpellSlots = levels

	render(w, r, pages.EditCharacter(data))
}

// loadCharacter is the ownership gate every editor page goes through, and the
// only one: a page that asked the question a second way would answer a miss
// differently sooner or later. Every failure is a redirect rather than an alert,
// because this answers a page request and nothing is open yet to show an alert
// in. A character that is not this user's and one that never existed are the
// same miss, because the query is scoped to the owner.
//
// The parsed id comes back with the row. Two of the three pages go on to query
// the inventory table with it, and would otherwise have to parse the path a
// second time for a value this function already had.
func (a *App) loadCharacter(w http.ResponseWriter, r *http.Request) (queries.Character, ulid.ULID, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	id := r.PathValue("id")
	if id == "" {
		redirect(w, r, "/characters")
		return queries.Character{}, ulid.ULID{}, false
	}
	uid, err := ulid.Parse(id)
	if err != nil {
		redirect(w, r, "/characters")
		return queries.Character{}, ulid.ULID{}, false
	}

	character, err := a.Queries.GetCharacter(ctx, queries.GetCharacterParams{
		ID:      uid,
		OwnerID: sess.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		redirect(w, r, "/characters")
		return queries.Character{}, ulid.ULID{}, false
	}
	if err != nil {
		slog.Error("Failed to load character", "error", err)
		redirectToError(w, r)
		return queries.Character{}, ulid.ULID{}, false
	}

	return character, uid, true
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

// FeatureRowFragment serves one blank repeater row to the add button on the
// Features panel. It is the whole server side of the add-row mechanic: no
// database, no session data in the response, just the same templ component the
// initial page render uses, so a row is defined in exactly one place.
//
// IT TAKES NOTHING, which is the point of the route being named after what it
// serves. It used to read a ?field= that decided the name attributes on the row
// it returned, and so had to check that value against an allowlist before
// rendering -- an unvalidated one would have put arbitrary field names into the
// next post. There is one repeater now, the row's field names are constants, and
// a parameter that cannot vary cannot be wrong.
//
// Behind middleware.Fragment, which is RequireSession plus the no-store and
// noindex headers every /fragment/ route owes its caller. The markup is not
// secret -- it holds nothing but empty fields -- but an unauthenticated
// endpoint here would be surface for no reason.
func (a *App) FeatureRowFragment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.FeatureRowFragment())
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
		Features:        parseFeatures(character.Features),
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

// parseFeatures unmarshals a stored `[{"name": ..., "value": ...}]` column into
// the slice the templates range over. It replaced normalizeInfoRowsJSON, which
// re-marshalled the same rows back into a string for a data-rows attribute so
// monster-info-table.js could parse them a second time in the browser.
//
// A malformed column yields an empty slice and a warning, which is what the old
// function did and what the component did on top of it -- the repeater renders
// with no rows and the add button still works, rather than the page failing.
func parseFeatures(raw json.RawMessage) []pages.Feature {
	rows := []pages.Feature{}
	if len(raw) == 0 {
		return rows
	}

	if err := json.Unmarshal(raw, &rows); err != nil {
		slog.Warn("invalid info rows payload; defaulting", "error", err)
		return []pages.Feature{}
	}

	if rows == nil {
		rows = []pages.Feature{}
	}

	return rows
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

type featurePayload struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func marshalFeatureRowsPayload(r *http.Request) (json.RawMessage, error) {
	names := r.PostForm[pages.FeaturesPanel+"-name"]
	values := r.PostForm[pages.FeaturesPanel+"-value"]
	rows := make([]featurePayload, 0)

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

		rows = append(rows, featurePayload{Name: name, Value: value})
	}

	payload, err := json.Marshal(rows)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(payload), nil
}
