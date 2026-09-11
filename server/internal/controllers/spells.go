package controllers

import (
	"context"
	"database/sql"
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

const (
	spellNameLimit        = 128
	spellComponentsLimit  = 128
	spellCastingTimeLimit = 64
	spellRangeLimit       = 64
	spellDurationLimit    = 64
	spellDescriptionLimit = 65535
	spellSlotLimit        = 99
)

func (a *App) CharacterSpellsRedirect(w http.ResponseWriter, r *http.Request) {
	characterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		redirect(w, r, "/characters")
		return
	}
	redirect(w, r, "/characters/"+characterID.String()+"/edit/spells/0")
}
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
func (a *App) loadSpellLevels(w http.ResponseWriter, r *http.Request, characterID, ownerID ulid.ULID) ([]pages.SpellLevel, bool) {
	levels, err := a.spellLevels(r.Context(), characterID, ownerID)
	if err != nil {
		slog.Error("Failed to load spell levels", "error", err)
		redirectToError(w, r)
		return nil, false
	}
	return levels, true
}
func (a *App) spellLevels(ctx context.Context, characterID, ownerID ulid.ULID) ([]pages.SpellLevel, error) {
	slots, err := a.Queries.ListSpellSlots(ctx, queries.ListSpellSlotsParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
	})
	if err != nil {
		return nil, fmt.Errorf("spell slots: %w", err)
	}
	counts, err := a.Queries.CountSpellsByLevel(ctx, queries.CountSpellsByLevelParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
	})
	if err != nil {
		return nil, fmt.Errorf("spell counts: %w", err)
	}
	return mergeSpellLevels(slots, counts), nil
}
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
	finishRow(w, r, panel, spellToastLabel(input.Name), "spell", result, err)
}
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
	finishRow(w, r, panel, pages.SpellLevelName(int(level))+" slots", "character", result, err)
}
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
		ID:           row.ID.String(),
		Level:        int(row.Level),
		Name:         row.Name,
		School:       pages.NormalizeSpellSchool(row.School),
		Components:   row.Components,
		CastingTime:  row.CastingTime,
		CastingRange: row.CastingRange,
		Duration:     row.Duration,
		Description:  row.Description,
		Prepared:     row.IsPrepared,
	}
}
