package controllers

import (
	"database/sql"
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

// The spells tab. Spellcasting used to be one panel writing one JSON column that
// held all ten levels at once; it is two tables and a page per level now, and
// these are the handlers for both.
//
// Like inventory, the row is the unit of work rather than the panel, and for the
// same reason: a spell referenced from a second view needs an identity that
// survives an edit, which a whole-list rewrite could not give it.
//
// The column widths are enforced here because MySQL runs in strict mode: an
// overlong value comes back from the driver as an error, so without these a
// pasted stat block in a 64-character field would reach the user as a 500 on a
// field they were entitled to overfill. The first five are measured in
// characters, which is what varchar counts; the spell text is measured in bytes,
// which is what TEXT counts.
const (
	spellNameLimit        = 128
	spellComponentsLimit  = 128
	spellCastingTimeLimit = 64
	spellRangeLimit       = 64
	spellDurationLimit    = 64
	spellDescriptionLimit = 65535

	// The counters are TINYINT UNSIGNED, so the column stops at 255. 99 is what
	// the inputs declare with max="99" and what the old JSON path clamped to.
	spellSlotLimit = 99
)

// CharacterSpellsRedirect is all that is left of /edit/spells. The spells tab
// opens on cantrips and there is no index above the levels, so this exists only
// so a bookmark or a stale tab href lands on a page instead of the catch-all
// 404.
//
// The id is parsed and printed back rather than passed through, so the Location
// it builds can only be a canonical ULID. It reads no database: a character that
// is not this user's is caught by the page it redirects to, and doing it twice
// would mean two queries to answer a request that renders nothing.
func (a *App) CharacterSpellsRedirect(w http.ResponseWriter, r *http.Request) {
	characterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		redirect(w, r, "/characters")
		return
	}

	redirect(w, r, "/characters/"+characterID.String()+"/edit/spells/0")
}

// CharacterSpellLevelPage is one level's spells.
//
// A level that is not one of the ten sends the browser to cantrips rather than
// to /characters. The character is real -- loadCharacter has already said so --
// and only the last segment is wrong, so the first page of the section is the
// answer, not the character list.
func (a *App) CharacterSpellLevelPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	character, characterID, ok := a.loadCharacter(w, r)
	if !ok {
		return
	}

	level, valid := parseSpellLevel(r.PathValue("level"))
	if !valid {
		redirect(w, r, "/characters/"+characterID.String()+"/edit/spells/0")
		return
	}

	// Cantrips have no slots, so their page renders no counters and asks for
	// none. That is one query saved on the page the Spells tab opens.
	var counters pages.SpellLevel
	if level > 0 {
		var ok bool
		if counters, ok = a.loadSpellSlots(w, r, characterID, sess.UserID, level); !ok {
			return
		}
	}

	rows, err := a.Queries.ListSpellsAtLevel(ctx, queries.ListSpellsAtLevelParams{
		CharacterID: characterID,
		OwnerID:     sess.UserID,
		Level:       level,
	})
	if err != nil {
		slog.Error("Failed to load spells", "error", err, "level", level)
		redirectToError(w, r)
		return
	}

	render(w, r, pages.EditCharacterSpellLevel(pages.SpellLevelPageData{
		CharacterID: characterID.String(),
		Header:      characterHeader(character),
		Level:       int(level),
		Current:     counters,
		Spells:      spellPageRows(rows),
	}))
}

// loadSpellSlots reads one level's counters.
//
// Nothing seeds spell_slots, so a level nobody has given a count has no row, and
// ErrNoRows is the zeroes it would have held rather than a failure. That is the
// property the table was designed around; it used to be spelled as a list of
// however many rows existed, back when one page rendered all ten levels.
func (a *App) loadSpellSlots(w http.ResponseWriter, r *http.Request, characterID, ownerID ulid.ULID, level uint8) (pages.SpellLevel, bool) {
	counters := pages.SpellLevel{Level: int(level), Slots: "0", Used: "0"}

	row, err := a.Queries.GetSpellSlots(r.Context(), queries.GetSpellSlotsParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
		Level:       level,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return counters, true
	}
	if err != nil {
		slog.Error("Failed to load spell slots", "error", err, "level", level)
		redirectToError(w, r)
		return pages.SpellLevel{}, false
	}

	counters.Slots = strconv.FormatUint(uint64(row.Slots), 10)
	counters.Used = strconv.FormatUint(uint64(row.Used), 10)

	return counters, true
}

// loadSpellLevels reads all ten levels for the Spell Slots panel on the
// Character tab: every level's counters, and how many spells each one holds.
//
// Two queries rather than a join. A character with no slot rows and no spells is
// the normal state of a fighter, and there is no join that produces ten rows of
// zeroes from two empty results -- MySQL has no FULL OUTER JOIN, and the levels
// missing from each side are not the same levels. The loop below says it in Go
// for the cost of one extra round trip.
func (a *App) loadSpellLevels(w http.ResponseWriter, r *http.Request, characterID, ownerID ulid.ULID) ([]pages.SpellLevel, bool) {
	ctx := r.Context()

	slots, err := a.Queries.ListSpellSlots(ctx, queries.ListSpellSlotsParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
	})
	if err != nil {
		slog.Error("Failed to load spell slots", "error", err)
		redirectToError(w, r)
		return nil, false
	}

	counts, err := a.Queries.CountSpellsByLevel(ctx, queries.CountSpellsByLevelParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
	})
	if err != nil {
		slog.Error("Failed to count spells by level", "error", err)
		redirectToError(w, r)
		return nil, false
	}

	return mergeSpellLevels(slots, counts), true
}

// mergeSpellLevels builds all ten levels from however few rows the two queries
// returned. Neither table seeds anything, so a level nobody has touched appears
// in neither result and reads here as the zeroes it would have held.
func mergeSpellLevels(slots []queries.SpellSlot, counts []queries.CountSpellsByLevelRow) []pages.SpellLevel {
	counters := make(map[uint8]queries.SpellSlot, len(slots))
	for _, row := range slots {
		counters[row.Level] = row
	}

	totals := make(map[uint8]int64, len(counts))
	for _, row := range counts {
		totals[row.Level] = row.Total
	}

	levels := make([]pages.SpellLevel, 0, pages.MaxSpellLevel+1)
	for level := 0; level <= pages.MaxSpellLevel; level++ {
		counter := counters[uint8(level)]
		levels = append(levels, pages.SpellLevel{
			Level: level,
			Slots: strconv.FormatUint(uint64(counter.Slots), 10),
			Used:  strconv.FormatUint(uint64(counter.Used), 10),
			Count: int(totals[uint8(level)]),
		})
	}

	return levels
}

// AddSpell creates an empty spell at a level and answers with it. The level is
// the only thing that comes from the client, and it comes from the URL of the
// page the button is on rather than from a field.
//
// The row is read back rather than assembled from what the insert "should" have
// written, so the schema stays the only place a new spell's starting school is
// declared. It costs a second round trip on a button press, which is not a
// keystroke.
func (a *App) AddSpell(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	level, ok := spellLevelPath(w, r)
	if !ok {
		return
	}

	spellID := ulid.Make()
	// The statement selects from characters, so a character that is not this
	// user's matches nothing and inserts nothing. Zero rows is that, and it is
	// the only thing it can be: the id is freshly minted, so a duplicate key is
	// not on the table.
	result, err := a.Queries.InsertSpell(ctx, queries.InsertSpellParams{
		ID:          spellID,
		Level:       level,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to add spell", "error", err)
		htmx.ServerError(w)
		return
	}
	if inserted, err := result.RowsAffected(); err == nil && inserted == 0 {
		htmx.NotFound(w, "character")
		return
	}

	spell, err := a.Queries.GetSpell(ctx, queries.GetSpellParams{
		ID:          spellID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
		Level:       level,
	})
	if err != nil {
		slog.Error("Failed to read back new spell", "error", err)
		htmx.ServerError(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.SpellRow(characterID.String(), spellPageRow(spell)))
}

// SaveSpell is the row's autosave. It writes eight columns and reads eight
// fields, and the form that posts them renders all eight together -- which is
// what makes a narrow read of a wide write impossible here. See buildSpellInput
// for the one field where that is load-bearing rather than incidental.
//
// level is not among them. A spell cannot change level, so no control renders
// one and the statement does not name the column; the level in the URL is a
// filter, not a value.
func (a *App) SaveSpell(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	level, ok := spellLevelPath(w, r)
	if !ok {
		return
	}
	spellID, ok := spellRowID(w, r)
	if !ok {
		return
	}

	panel := pages.SpellRowPanel(spellID.String())
	if !parsePanelForm(w, r, panel) {
		return
	}

	input, problems := buildSpellInput(r)
	if len(problems) > 0 {
		renderPanelBlock(w, r, panel, problems)
		return
	}

	result, err := a.Queries.UpdateSpell(ctx, queries.UpdateSpellParams{
		Name:         input.Name,
		School:       input.School,
		Components:   input.Components,
		CastingTime:  input.CastingTime,
		CastingRange: input.CastingRange,
		Duration:     input.Duration,
		Description:  input.Description,
		IsPrepared:   input.Prepared,
		ID:           spellID,
		CharacterID:  characterID,
		OwnerID:      sess.UserID,
		Level:        level,
	})
	finishSpellRow(w, r, panel, input.Name, result, err)
}

// DeleteSpell drops one row. The reply carries no body, and it MUST be a 200:
// base.templ's noSwap config lists 204, and a status in that list sets the swap
// to "none", which overrides the hx-swap="delete" on the button and leaves the
// row sitting on screen after the database has dropped it. DeleteInventoryItem
// is the same shape for the same reason.
func (a *App) DeleteSpell(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	level, ok := spellLevelPath(w, r)
	if !ok {
		return
	}
	spellID, ok := spellRowID(w, r)
	if !ok {
		return
	}

	result, err := a.Queries.DeleteSpell(ctx, queries.DeleteSpellParams{
		ID:          spellID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
		Level:       level,
	})
	if err != nil {
		slog.Error("Failed to delete spell", "error", err)
		htmx.ServerError(w)
		return
	}
	if deleted, err := result.RowsAffected(); err == nil && deleted == 0 {
		htmx.NotFound(w, "spell")
		return
	}

	htmx.Toast(w, "Spell deleted.")
}

// SaveSpellSlots writes one level's counters. Cantrips are excluded rather than
// stored as zeroes: they have no slots in the rules, the cantrips page renders
// no counters at all, and a route that accepted level 0 would be accepting
// something no page can send.
func (a *App) SaveSpellSlots(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	level, ok := spellLevelPath(w, r)
	if !ok {
		return
	}
	if level == 0 {
		unknownSpellLevel(w, r.PathValue("level"))
		return
	}

	panel := pages.SpellSlotsPanel(int(level))
	if !parsePanelForm(w, r, panel) {
		return
	}

	slots := parseSlotCount(r.PostFormValue("slots"))
	used := parseSlotCount(r.PostFormValue("used"))
	// A level cannot have more slots spent than it has. Both fields post
	// together from the same form, so the ceiling is whatever the slots field
	// says at the moment of the save rather than whatever is stored.
	if used > slots {
		used = slots
	}

	result, err := a.Queries.UpsertSpellSlots(ctx, queries.UpsertSpellSlotsParams{
		Level:       level,
		Slots:       slots,
		Used:        used,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	finishPanel(w, r, panel, pages.SpellLevelName(int(level))+" slots", result, err)
}

// finishSpellRow is finishInventoryRow's shape with one word changed, and the
// word is the reason it is not finishPanel. Zero matched rows on a panel means
// the character is not this user's; here it means this spell is gone -- deleted
// in another tab, most likely -- and telling someone their character no longer
// exists because a row does would send them to look for the wrong problem.
func finishSpellRow(w http.ResponseWriter, r *http.Request, panel string, name string, result sql.Result, err error) {
	if err != nil {
		slog.Error("Failed to save spell", "error", err)
		htmx.ServerError(w)
		return
	}

	if matched, err := result.RowsAffected(); err == nil && matched == 0 {
		htmx.NotFound(w, "spell")
		return
	}

	htmx.Toast(w, spellToastLabel(name)+" saved.")
	renderPanelBlock(w, r, panel, nil)
}

// A row spends its first seconds nameless, and a debounce landing in there
// should not toast " saved.".
func spellToastLabel(name string) string {
	if name == "" {
		return "Spell"
	}

	return name
}

type spellInput struct {
	Name         string
	School       string
	Components   string
	CastingTime  string
	CastingRange string
	Duration     string
	Description  string
	Prepared     bool
}

// buildSpellInput reads one row off the form. Nothing here is required --
// specifically not the name. A row is created empty and named afterwards, so a
// required name would mean the browser refused to post the row that most needs
// posting.
//
// PREPARED IS READ FROM THE ABSENCE OF A FIELD, because an unchecked box posts
// nothing at all. That is correct for a checkbox and it is also the exact shape
// the panel handlers are built to avoid -- a reader treating "not sent" as a
// value. It is safe only because the row form renders all eight controls
// together, so a post that omits `prepared` really is an unticked box rather
// than a partial form. That is a property of the markup, not of this function,
// which is why there is a test pinning it.
//
// The school is normalised rather than validated. The control is a select with
// no empty option, so a value outside the eight did not come from the form, and
// failing a save over a field the user cannot mistype would cost more than it
// protects.
func buildSpellInput(r *http.Request) (spellInput, []string) {
	name := strings.TrimSpace(r.PostFormValue("name"))
	components := strings.TrimSpace(r.PostFormValue("components"))
	castingTime := strings.TrimSpace(r.PostFormValue("casting_time"))
	castingRange := strings.TrimSpace(r.PostFormValue("casting_range"))
	duration := strings.TrimSpace(r.PostFormValue("duration"))
	description := strings.TrimSpace(r.PostFormValue("description"))

	var problems []string
	for _, field := range []struct {
		value   string
		limit   int
		message string
	}{
		{name, spellNameLimit, "Spell name must be 128 characters or fewer."},
		{components, spellComponentsLimit, "Components must be 128 characters or fewer."},
		{castingTime, spellCastingTimeLimit, "Cast time must be 64 characters or fewer."},
		{castingRange, spellRangeLimit, "Range must be 64 characters or fewer."},
		{duration, spellDurationLimit, "Duration must be 64 characters or fewer."},
	} {
		if len([]rune(field.value)) > field.limit {
			problems = append(problems, field.message)
		}
	}

	if len(description) > spellDescriptionLimit {
		problems = append(problems, "Spell text is too long to save.")
	}

	return spellInput{
		Name:         name,
		School:       pages.NormalizeSpellSchool(strings.TrimSpace(r.PostFormValue("school"))),
		Components:   components,
		CastingTime:  castingTime,
		CastingRange: castingRange,
		Duration:     duration,
		Description:  description,
		Prepared:     r.PostFormValue("prepared") != "",
	}, problems
}

// An empty counter is 0, matching the column default: the field is empty for a
// moment every time someone retypes it, and a debounce landing in that moment
// should store the zero rather than refuse the save. Anything unparseable is 0
// for a different reason -- type=number cannot produce it, so it is a hand-built
// post and not a user to explain anything to.
func parseSlotCount(raw string) uint8 {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0
	}

	count, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0
	}

	return uint8(min(max(count, 0), spellSlotLimit))
}

// parseSpellLevel bounds a level to the ten that exist. It is the only thing
// standing between the {level} segment and a query, and it is why the segment
// can be interpolated into a panel id: what comes out is a number from 0 to 9.
func parseSpellLevel(raw string) (uint8, bool) {
	level, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 8)
	if err != nil || level > pages.MaxSpellLevel {
		return 0, false
	}

	return uint8(level), true
}

func spellLevelPath(w http.ResponseWriter, r *http.Request) (uint8, bool) {
	level, ok := parseSpellLevel(r.PathValue("level"))
	if !ok {
		unknownSpellLevel(w, r.PathValue("level"))
		return 0, false
	}

	return level, true
}

// unknownSpellLevel answers a {level} segment no page could have produced --
// every level the editor sends is a number baked into a link or a button. So the
// message says the page is wrong rather than the character is gone, the way
// unknownBonusKind does for the same reason.
func unknownSpellLevel(w http.ResponseWriter, raw string) {
	slog.Warn("unknown spell level requested", "level", raw)
	htmx.Error(w, "Not Found", "That part of the character sheet does not exist. Refresh the page and try again.", http.StatusNotFound)
}

func spellRowID(w http.ResponseWriter, r *http.Request) (ulid.ULID, bool) {
	spellID, err := ulid.Parse(r.PathValue("spellId"))
	if err != nil {
		htmx.NotFound(w, "spell")
		return ulid.ULID{}, false
	}

	return spellID, true
}

// preparedSpellGroups splits the prepared rows into one group per level for the
// read-only view on the Character tab.
//
// IT RELIES ON THE QUERY'S ORDER BY, comparing each row against the group it is
// building rather than collecting into a map and sorting. ListPreparedSpells
// orders by level and then id, so rows of a level arrive together and in the
// order they were added -- the same order the level page shows them in. A
// statement that dropped the ordering would not fail here; it would quietly
// render Level 3 twice.
func preparedSpellGroups(rows []queries.Spell) []pages.PreparedSpellGroup {
	groups := make([]pages.PreparedSpellGroup, 0, pages.MaxSpellLevel+1)
	for _, row := range rows {
		spell := spellPageRow(row)
		if last := len(groups) - 1; last >= 0 && groups[last].Level == spell.Level {
			groups[last].Spells = append(groups[last].Spells, spell)
			continue
		}

		groups = append(groups, pages.PreparedSpellGroup{
			Level:  spell.Level,
			Name:   pages.SpellLevelName(spell.Level),
			Spells: []pages.Spell{spell},
		})
	}

	return groups
}

func spellPageRows(rows []queries.Spell) []pages.Spell {
	spells := make([]pages.Spell, 0, len(rows))
	for _, row := range rows {
		spells = append(spells, spellPageRow(row))
	}

	return spells
}

func spellPageRow(row queries.Spell) pages.Spell {
	return pages.Spell{
		ID:    row.ID.String(),
		Level: int(row.Level),
		Name:  row.Name,
		// Normalised on the way out as well as on the way in. The column takes
		// any 32 characters, and the select can only render one of the eight --
		// a value that is not among them would otherwise silently become
		// whichever option happened to be first.
		School:       pages.NormalizeSpellSchool(row.School),
		Components:   row.Components,
		CastingTime:  row.CastingTime,
		CastingRange: row.CastingRange,
		Duration:     row.Duration,
		Description:  row.Description,
		Prepared:     row.IsPrepared,
	}
}
