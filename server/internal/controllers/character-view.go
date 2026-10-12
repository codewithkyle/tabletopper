package controllers

import (
	"context"
	"net/http"
	"slices"

	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

func (a *App) RoomCharacterSheetFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, role, err := a.roomMember(ctx, sess, r.URL.Query().Get("room"))
	if err != nil || role != room.RoleGM || a.Hub == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	characterID, err := ulid.Parse(r.URL.Query().Get("character"))
	section := r.URL.Query().Get("section")
	if section == "" {
		section = pages.SheetSectionMain
	}
	level, levelled := sheetLevel(r.URL.Query().Get("level"))
	if err != nil || !sheetSections[section] || !levelled {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	ownerID, pawn, ok := a.roomCharacter(ctx, row.ID, characterID)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	character, err := a.Queries.GetCharacter(ctx, queries.GetCharacterParams{ID: characterID, OwnerID: ownerID})
	if err != nil {
		refuseSheet(w, err)
		return
	}
	pawnVitalsOnCharacter(&character, pawn)
	data, err := a.characterView(ctx, character, section, level)
	if err != nil {
		refuseSheet(w, err)
		return
	}
	data.RoomID = row.ID.String()
	data.CharacterID = characterID.String()
	if pawn != nil {
		data.PawnID = pawn.ID.String()
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if slices.Contains(pages.LiveSheetSections(), section) {
		render(w, r, pages.CharacterViewSection(data))
		return
	}
	render(w, r, pages.CharacterView(data))
}
func (a *App) roomCharacter(ctx context.Context, roomID, characterID ulid.ULID) (ulid.ULID, *room.Pawn, bool) {
	pawn := a.characterPawn(ctx, roomID, characterID)
	if pawn != nil && pawn.OwnerID != nil {
		return *pawn.OwnerID, pawn, true
	}
	players, _ := a.Hub.Players(ctx, roomID)
	for _, p := range players {
		if p.Role != room.RoleGM && p.CharacterID != nil && *p.CharacterID == characterID {
			return p.ID, pawn, true
		}
	}
	return ulid.ULID{}, nil, false
}
func (a *App) characterView(ctx context.Context, character queries.Character, section string, level int) (pages.CharacterViewData, error) {
	data := pages.CharacterViewData{Section: section, Level: level}
	var (
		attacks []queries.Attack
		items   []queries.Inventory
		spells  []queries.Spell
		err     error
	)
	switch {
	case section == pages.SheetSectionAttacks:
		attacks, err = a.Queries.ListCharacterAttacks(ctx, queries.ListCharacterAttacksParams{
			CharacterID: character.ID,
			OwnerID:     character.OwnerID,
		})
	case section == pages.SheetSectionInventory:
		items, err = a.Queries.ListCharacterInventory(ctx, queries.ListCharacterInventoryParams{
			CharacterID: character.ID,
			OwnerID:     character.OwnerID,
		})
	case section == pages.SheetSectionSpells && level == pages.SpellsPrepared:
		spells, err = a.Queries.ListPreparedSpells(ctx, queries.ListPreparedSpellsParams{
			CharacterID: character.ID,
			OwnerID:     character.OwnerID,
		})
		if err == nil {
			data.Levels, err = a.spellLevels(ctx, character.ID, character.OwnerID)
		}
	case section == pages.SheetSectionSpells:
		err = a.spellLevelReading(ctx, character, &data)
	}
	if err != nil {
		return data, err
	}
	data.Sheet = sharedCharacterSheet(character, attacks, items, spells, nil)
	if character.AssetID != nil {
		data.Sheet.Header.AvatarID = character.AssetID.String()
	}
	return data, nil
}
func (a *App) spellLevelReading(ctx context.Context, character queries.Character, data *pages.CharacterViewData) error {
	counters, err := a.spellSlotCounters(ctx, character.ID, character.OwnerID, uint8(data.Level))
	if err != nil {
		return err
	}
	rows, err := a.Queries.ListSpellsAtLevel(ctx, queries.ListSpellsAtLevelParams{
		CharacterID: character.ID,
		OwnerID:     character.OwnerID,
		Level:       uint8(data.Level),
	})
	if err != nil {
		return err
	}
	data.Current = counters
	for _, row := range rows {
		data.LevelSpells = append(data.LevelSpells, sharedSpell(spellPageRow(row)))
	}
	return nil
}
func pawnVitalsOnCharacter(character *queries.Character, pawn *room.Pawn) {
	vitals, ok := room.PawnVitals(pawnOrEmpty(pawn))
	if !ok {
		return
	}
	character.CurrentHP = uint16(max(vitals.HP, 0))
	character.MaxHP = uint16(max(vitals.MaxHP, 0))
	character.AC = uint16(max(vitals.AC, 0))
	character.Size = string(vitals.Size)
}
