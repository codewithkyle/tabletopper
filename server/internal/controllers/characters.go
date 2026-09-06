package controllers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

// DeleteCharacter empties every table that holds a row for this character,
// deletes the objects those rows point at, and only then removes the character
// itself.
//
// NOTHING CASCADES IN THIS SCHEMA -- there are no foreign keys -- so every one
// of those tables is named here, and a row this handler forgets is unreachable
// the moment the character is gone: no page can open it, no later delete will
// find it, and an asset row it forgot is a key sitting in the bucket that
// nothing will ever ask for again. The order below is the whole of the safety.
//
// OBJECTS BEFORE THE ROWS THAT DESCRIBE THEM. The row is the record that an
// object may exist, so R2 goes first and the rows after; the other way round
// leaves a bucket filling with keys nothing remembers. That is also why the
// journal image paths are read before anything is deleted at all -- the join
// that finds them runs through the journals table, and once those rows are gone
// there is nothing left that knows which objects were this character's.
//
// ONE CLASS OF PICTURE IS NOT REACHED FROM HERE, and it belongs to the
// sweeper. Deleting a single entry detaches its images and removes the entry
// row, so their journal_id names a journal that is gone and the join above
// cannot see them. They are already marked detached, which is precisely what
// internal/sweep looks for, and it takes the object and the row within the day.
//
// THE CHARACTER ROW GOES LAST, and that is what makes every failure above it
// recoverable. While it is there the roster still lists the character and
// deleting it again re-runs the whole purge from the top: every statement is a
// delete scoped by the character and the owner, so repeating one finds nothing
// and succeeds, and a key already gone from R2 deletes again without complaint.
// So each step below reports its failure and stops, and the reader gets a
// character they can delete a second time rather than a report of a delete that
// half happened.
//
// Past that row there is nothing left to find, which is why the avatar's asset
// row -- the one step that runs after it -- is logged rather than reported: a
// retry could not reach it, and the user's request has been honoured.
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

	// Read first, destroy after. This is the only step that has to precede the
	// deletes rather than merely preferring to.
	imageKeys, err := a.Queries.ListCharacterJournalImages(ctx, queries.ListCharacterJournalImagesParams{
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to list character journal images", "error", err)
		htmx.ServerError(w)
		return
	}

	// One call for the journal, whatever its size: DeleteMany batches, and an
	// empty list is a no-op, so a character who never pasted a picture pays
	// nothing for this.
	if err := a.Storage.DeleteMany(ctx, imageKeys); err != nil {
		slog.Error("Failed to delete journal image objects", "error", err, "count", len(imageKeys))
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

	if err := a.deleteCharacterRows(ctx, characterID, sess.UserID); err != nil {
		slog.Error("Failed to delete character rows", "error", err)
		htmx.ServerError(w)
		return
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

// deleteCharacterRows empties every table that carries this character's rows,
// and it is separate from the handler because the list is the point: one
// statement per table, in one place, so a table added to the schema has an
// obvious hole to fill. TestDeletingACharacterEmptiesEveryTableThatHoldsItsRows
// reads db/schema.sql and fails when one is missing.
//
// THE FIRST TWO ARE ORDERED AND THE REST ARE NOT. The image delete finds its
// rows through the journals table, so emptying that table first would leave
// every picture behind with nothing pointing at it. The other four are
// independent and run in the order they were written.
//
// The shares are among the independent ones only because the column is
// denormalised. Every link this character handed out names it directly, so they
// go in one statement whether or not the journals they point at are still
// there -- which is the reason shares.character_id exists at all; reaching them
// through journals would have meant a subquery against a table this request
// empties two statements earlier.
//
// The first failure stops the purge and is returned wrapped, so the log names
// the table rather than only the driver error -- which of these failed is the
// difference between a leftover the reader can clear by deleting again and one
// they cannot.
func (a *App) deleteCharacterRows(ctx context.Context, characterID, ownerID ulid.ULID) error {
	if err := a.Queries.DeleteCharacterJournalImages(ctx, queries.DeleteCharacterJournalImagesParams{
		OwnerID:     ownerID,
		CharacterID: characterID,
	}); err != nil {
		return fmt.Errorf("journal images: %w", err)
	}

	if err := a.Queries.DeleteCharacterJournals(ctx, queries.DeleteCharacterJournalsParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
	}); err != nil {
		return fmt.Errorf("journals: %w", err)
	}

	if err := a.Queries.DeleteCharacterInventory(ctx, queries.DeleteCharacterInventoryParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
	}); err != nil {
		return fmt.Errorf("inventory: %w", err)
	}

	if err := a.Queries.DeleteCharacterSpells(ctx, queries.DeleteCharacterSpellsParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
	}); err != nil {
		return fmt.Errorf("spells: %w", err)
	}

	if err := a.Queries.DeleteCharacterSpellSlots(ctx, queries.DeleteCharacterSpellSlotsParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
	}); err != nil {
		return fmt.Errorf("spell slots: %w", err)
	}

	if err := a.Queries.DeleteCharacterShares(ctx, queries.DeleteCharacterSharesParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
	}); err != nil {
		return fmt.Errorf("shares: %w", err)
	}

	return nil
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
