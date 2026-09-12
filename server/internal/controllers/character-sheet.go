package controllers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strconv"

	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

var sheetSections = sheetSectionSet(
	pages.SheetSectionMain,
	pages.SheetSectionInventory,
	pages.SheetSectionSpells,
)

func sheetSectionSet(extra ...string) map[string]bool {
	out := make(map[string]bool, len(extra)+len(pages.LiveSheetSections()))
	for _, name := range append(extra, pages.LiveSheetSections()...) {
		out[name] = true
	}
	return out
}
func (a *App) CharacterSheetFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, _, err := a.roomMember(ctx, sess, r.URL.Query().Get("room"))
	if err != nil || sess.CharacterID == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	section := r.URL.Query().Get("section")
	if section == "" {
		section = pages.SheetSectionMain
	}
	level, levelled := sheetLevel(r.URL.Query().Get("level"))
	if !sheetSections[section] || !levelled {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	data, err := a.sheetWindowData(ctx, *sess.CharacterID, sess.UserID, section, level)
	if err != nil {
		refuseSheet(w, err)
		return
	}
	data.RoomID = row.ID.String()
	applyPawnVitals(&data.Sheet, a.characterPawn(ctx, row.ID, *sess.CharacterID))
	data.Sheet.Live = pages.SheetLive{RoomID: data.RoomID}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if slices.Contains(pages.LiveSheetSections(), section) {
		render(w, r, pages.CharacterSheetSection(data.Sheet, section))
		return
	}
	render(w, r, pages.CharacterSheetWindow(data))
}
func (a *App) sheetWindowData(ctx context.Context, characterID, ownerID ulid.ULID, section string, level uint8) (pages.SheetWindowData, error) {
	data := pages.SheetWindowData{Section: section, Level: int(level)}
	switch section {
	case pages.SheetSectionInventory:
		header, err := a.sheetHeader(ctx, characterID, ownerID)
		if err != nil {
			return data, err
		}
		items, err := a.Queries.ListCharacterInventory(ctx, queries.ListCharacterInventoryParams{
			CharacterID: characterID,
			OwnerID:     ownerID,
		})
		if err != nil {
			return data, err
		}
		data.Sheet.Header = header
		data.Inventory = pages.InventoryPageData{
			CharacterID: characterID.String(),
			Items:       inventoryPageItems(items),
		}
	case pages.SheetSectionSpells:
		header, err := a.sheetHeader(ctx, characterID, ownerID)
		if err != nil {
			return data, err
		}
		counters, err := a.spellSlotCounters(ctx, characterID, ownerID, level)
		if err != nil {
			return data, err
		}
		rows, err := a.Queries.ListSpellsAtLevel(ctx, queries.ListSpellsAtLevelParams{
			CharacterID: characterID,
			OwnerID:     ownerID,
			Level:       level,
		})
		if err != nil {
			return data, err
		}
		data.Sheet.Header = header
		data.Spells = pages.SpellLevelPageData{
			CharacterID: characterID.String(),
			Level:       int(level),
			Current:     counters,
			Spells:      spellPageRows(rows),
		}
	case pages.SheetSectionMain:
		sheet, err := a.sheetData(ctx, characterID, ownerID)
		if err != nil {
			return data, err
		}
		data.Sheet = sheet
	default:
		character, err := a.Queries.GetCharacter(ctx, queries.GetCharacterParams{ID: characterID, OwnerID: ownerID})
		if err != nil {
			return data, err
		}
		data.Sheet = characterToEditPageData(characterID.String(), character)
	}
	return data, nil
}
func (a *App) sheetHeader(ctx context.Context, characterID, ownerID ulid.ULID) (pages.CharacterHeader, error) {
	character, err := a.Queries.GetCharacter(ctx, queries.GetCharacterParams{ID: characterID, OwnerID: ownerID})
	if err != nil {
		return pages.CharacterHeader{}, err
	}
	return characterHeaderFrom(character, characterDerived(character)), nil
}
func refuseSheet(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	slog.Error("Failed to read a character for its window", "error", err)
	htmx.ServerError(w)
}
func sheetLevel(raw string) (uint8, bool) {
	if raw == "" {
		return 0, true
	}
	level, err := strconv.Atoi(raw)
	if err != nil || level < 0 || level > pages.MaxSpellLevel {
		return 0, false
	}
	return uint8(level), true
}
func (a *App) characterPawn(ctx context.Context, roomID, characterID ulid.ULID) *room.Pawn {
	if a.Hub == nil {
		return nil
	}
	pawn, ok := a.Hub.CharacterPawn(ctx, roomID, characterID)
	if !ok {
		return nil
	}
	return pawn
}
func applyPawnVitals(data *pages.EditCharacterPageData, pawn *room.Pawn) {
	vitals, ok := room.PawnVitals(pawnOrEmpty(pawn))
	if !ok {
		return
	}
	data.CurrentHP = strconv.Itoa(vitals.HP)
	data.MaxHP = strconv.Itoa(vitals.MaxHP)
	data.AC = strconv.Itoa(vitals.AC)
	data.Size = string(vitals.Size)
	data.Header.CurrentHP = data.CurrentHP
	data.Header.MaxHP = data.MaxHP
	data.Header.AC = data.AC
}
func pawnOrEmpty(pawn *room.Pawn) room.Pawn {
	if pawn == nil {
		return room.Pawn{}
	}
	return *pawn
}
