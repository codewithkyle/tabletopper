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

var sheetSections = sheetSectionSet()

func sheetSectionSet() map[string]bool {
	out := make(map[string]bool, len(pages.SheetTabs()))
	for _, tab := range pages.SheetTabs() {
		out[tab.Section] = true
	}
	return out
}
func (a *App) CharacterSheetFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	if sess.CharacterID == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	row, _, ok := a.member(w, r)
	if !ok {
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
	data.Sheet.WindowRoomID = data.RoomID
	renderFragment(w, r, pages.CharacterSheetWindow(data))
}
func (a *App) CharacterPanelFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	characterID, err := ulid.Parse(r.URL.Query().Get("character"))
	section := r.URL.Query().Get("section")
	if err != nil || !slices.Contains(pages.LiveSheetSections(), section) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	character, err := a.Queries.GetCharacter(ctx, queries.GetCharacterParams{ID: characterID, OwnerID: sess.UserID})
	if err != nil {
		refuseSheet(w, err)
		return
	}
	data := characterToEditPageData(characterID.String(), character)
	a.applySeatedPawn(ctx, &data, characterID)
	renderFragment(w, r, pages.CharacterSheetSection(data, section))
	render(w, r, pages.CharacterBarChips(data.Header))
}
func (a *App) applySeatedPawn(ctx context.Context, data *pages.EditCharacterPageData, characterID ulid.ULID) {
	sess := session.FromContext(ctx)
	if sess.RoomID == nil || sess.CharacterID == nil || *sess.CharacterID != characterID {
		return
	}
	applyPawnVitals(data, a.characterPawn(ctx, *sess.RoomID, characterID))
}
func (a *App) sheetWindowData(ctx context.Context, characterID, ownerID ulid.ULID, section string, level int) (pages.SheetWindowData, error) {
	data := pages.SheetWindowData{Section: section, Level: level}
	switch {
	case section == pages.SheetSectionInventory:
		character, err := a.Queries.GetCharacter(ctx, queries.GetCharacterParams{ID: characterID, OwnerID: ownerID})
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
		data.Sheet.Header = characterHeaderFrom(character, characterDerived(character))
		data.Inventory = pages.InventoryPageData{
			CharacterID: characterID.String(),
			Items:       inventoryPageItems(items),
			Purse:       pages.NewCoinPurse(characterID.String(), characterPurse(character)),
		}
	case section == pages.SheetSectionSpells && level != pages.SpellsPrepared:
		header, err := a.sheetHeader(ctx, characterID, ownerID)
		if err != nil {
			return data, err
		}
		counters, err := a.spellSlotCounters(ctx, characterID, ownerID, uint8(level))
		if err != nil {
			return data, err
		}
		rows, err := a.Queries.ListSpellsAtLevel(ctx, queries.ListSpellsAtLevelParams{
			CharacterID: characterID,
			OwnerID:     ownerID,
			Level:       uint8(level),
		})
		if err != nil {
			return data, err
		}
		data.Sheet.Header = header
		data.Spells = pages.SpellLevelPageData{
			CharacterID: characterID.String(),
			Level:       level,
			Current:     counters,
			Spells:      spellPageRows(rows),
		}
	default:
		character, err := a.Queries.GetCharacter(ctx, queries.GetCharacterParams{ID: characterID, OwnerID: ownerID})
		if err != nil {
			return data, err
		}
		sheet, err := a.sheetSectionData(ctx, characterID, ownerID, character, section)
		if err != nil {
			return data, err
		}
		data.Sheet = sheet
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
func sheetLevel(raw string) (int, bool) {
	if raw == "" {
		return pages.SpellsPrepared, true
	}
	level, err := strconv.Atoi(raw)
	if err != nil || level < 0 || level > pages.MaxSpellLevel {
		return 0, false
	}
	return level, true
}
func (a *App) characterPawn(ctx context.Context, roomID, characterID ulid.ULID) *room.Pawn {
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
