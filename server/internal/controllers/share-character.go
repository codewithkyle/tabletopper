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

	"tabletopper/internal/dnd"
	"tabletopper/internal/queries"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

func (a *App) sharedCharacterSheet(w http.ResponseWriter, r *http.Request, token string, grant queries.GetShareByTokenRow) {
	sheet, portrait, err := a.loadCharacterSheet(r.Context(), grant.ResourceID, grant.OwnerID)
	if errors.Is(err, sql.ErrNoRows) {
		shareUnavailable(w, r)
		return
	}
	if err != nil {
		slog.Error("Failed to load shared character", "error", err)
		redirectToError(w, r)
		return
	}
	if portrait {
		sheet.Avatar = sharePortraitURL(token)
	}
	sheet.Actions = pages.SharedActions{
		Blurb:  "This sheet is read-only and stays in step with the character as it is edited. Export it as Markdown for your own notes.",
		Export: shareExportURL(token),
	}
	shareHeaders(w)
	render(w, r, pages.SharedCharacterPage(sheet))
}
func (a *App) loadCharacterSheet(ctx context.Context, characterID, ownerID ulid.ULID) (pages.SharedCharacterSheet, bool, error) {
	character, err := a.Queries.GetCharacter(ctx, queries.GetCharacterParams{
		ID:      characterID,
		OwnerID: ownerID,
	})
	if err != nil {
		return pages.SharedCharacterSheet{}, false, err
	}
	attacks, err := a.Queries.ListCharacterAttacks(ctx, queries.ListCharacterAttacksParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
	})
	if err != nil {
		return pages.SharedCharacterSheet{}, false, fmt.Errorf("attacks: %w", err)
	}
	items, err := a.Queries.ListCharacterInventory(ctx, queries.ListCharacterInventoryParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
	})
	if err != nil {
		return pages.SharedCharacterSheet{}, false, fmt.Errorf("inventory: %w", err)
	}
	spells, err := a.Queries.ListCharacterSpells(ctx, queries.ListCharacterSpellsParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
	})
	if err != nil {
		return pages.SharedCharacterSheet{}, false, fmt.Errorf("spells: %w", err)
	}
	levels, err := a.spellLevels(ctx, characterID, ownerID)
	if err != nil {
		return pages.SharedCharacterSheet{}, false, err
	}
	return sharedCharacterSheet(character, attacks, items, spells, levels), character.AssetID != nil, nil
}
func sharedCharacterSheet(
	character queries.Character,
	attacks []queries.Attack,
	items []queries.Inventory,
	spells []queries.Spell,
	levels []pages.SpellLevel,
) pages.SharedCharacterSheet {
	derived := characterDerived(character)
	header := characterHeaderFrom(character, derived)
	header.AvatarID = ""
	ability := dnd.NormalizeSpellcastingAbility(string(character.SpellcastingAbility))
	casts := ability != dnd.SpellcastingAbilityNone
	sheet := pages.SharedCharacterSheet{
		Header: header,
		Identity: sharedFacts(
			pages.SharedFact{Label: "Species", Value: nullStringValue(character.Race)},
			pages.SharedFact{Label: "Class", Value: nullStringValue(character.Classes)},
			pages.SharedFact{Label: "Background", Value: nullStringValue(character.Background)},
			pages.SharedFact{Label: "Alignment", Value: dnd.AlignmentLabel(nullStringValue(character.Alignment))},
			pages.SharedFact{Label: "Size", Value: dnd.SizeLabel(strings.TrimSpace(character.Size))},
		),
		CoreStats: sharedFacts(
			pages.SharedFact{Label: "Experience", Value: strconv.FormatUint(uint64(character.XP), 10)},
			pages.SharedFact{Label: "Speed", Value: fallbackString(strings.TrimSpace(character.Speed), "30 ft.")},
			pages.SharedFact{Label: "Armor Class", Value: strconv.FormatUint(uint64(character.AC), 10)},
			pages.SharedFact{Label: "Initiative Bonus", Value: pages.SignedNumber(int(character.InitiativeBonus))},
			pages.SharedFact{Label: "Spellcasting Ability", Value: dnd.SpellcastingAbilityLabel(ability)},
		),
		Vitals: sharedFacts(
			pages.SharedFact{Label: "Hit Point Maximum", Value: strconv.FormatUint(uint64(character.MaxHP), 10)},
			pages.SharedFact{Label: "Current Hit Points", Value: strconv.FormatUint(uint64(character.CurrentHP), 10)},
			pages.SharedFact{Label: "Temporary Hit Points", Value: countValue(character.TempHP)},
			pages.SharedFact{Label: "Hit Dice", Value: strings.TrimSpace(character.HitDice)},
			pages.SharedFact{Label: "Hit Dice Spent", Value: countValue(character.HitDiceSpent)},
			pages.SharedFact{Label: "Exhaustion", Value: countValue(character.Exhaustion)},
			pages.SharedFact{Label: "Death Save Successes", Value: countValue(character.DeathSaveSuccesses)},
			pages.SharedFact{Label: "Death Save Failures", Value: countValue(character.DeathSaveFailures)},
			pages.SharedFact{Label: "Heroic Inspiration", Value: flagValue(character.HeroicInspiration)},
		),
		Abilities: []pages.SharedAbility{
			{Label: "Strength", Score: strconv.FormatUint(uint64(character.Str), 10), Mod: derived.StrMod},
			{Label: "Dexterity", Score: strconv.FormatUint(uint64(character.Dex), 10), Mod: derived.DexMod},
			{Label: "Constitution", Score: strconv.FormatUint(uint64(character.Con), 10), Mod: derived.ConMod},
			{Label: "Intelligence", Score: strconv.FormatUint(uint64(character.Int), 10), Mod: derived.IntMod},
			{Label: "Wisdom", Score: strconv.FormatUint(uint64(character.Wis), 10), Mod: derived.WisMod},
			{Label: "Charisma", Score: strconv.FormatUint(uint64(character.Cha), 10), Mod: derived.ChaMod},
		},
		SavingThrows:      sharedBonuses(derived.SavingThrows),
		Skills:            sharedBonuses(derived.Skills),
		PassivePerception: derived.PassivePerception,
		Training: sharedFacts(
			pages.SharedFact{Label: "Languages", Value: strings.TrimSpace(character.Languages)},
			pages.SharedFact{Label: "Other Proficiencies", Value: strings.TrimSpace(character.Proficiencies)},
		),
		Attacks:   sharedAttacks(attacks),
		Features:  sharedFeatures(parseFeatures(character.Features)),
		Inventory: sharedItems(items),
		Coins:     pages.HeldCoins(characterPurse(character)),
		CoinTotal: pages.CoinTotalLabel(characterPurse(character)),
		Spells:    sharedSpellBook(spells, levels),
		Personality: sharedFacts(
			pages.SharedFact{Label: "Personality Traits", Value: character.PersonalityTraits},
			pages.SharedFact{Label: "Ideals", Value: character.Ideals},
			pages.SharedFact{Label: "Bonds", Value: character.Bonds},
			pages.SharedFact{Label: "Flaws", Value: character.Flaws},
		),
		Appearance: sharedFacts(
			pages.SharedFact{Label: "Age", Value: character.Age},
			pages.SharedFact{Label: "Height", Value: character.Height},
			pages.SharedFact{Label: "Weight", Value: character.Weight},
			pages.SharedFact{Label: "Eyes", Value: character.Eyes},
			pages.SharedFact{Label: "Skin", Value: character.Skin},
			pages.SharedFact{Label: "Hair", Value: character.Hair},
		),
	}
	if casts {
		sheet.CoreStats = append(sheet.CoreStats, pages.SharedFact{
			Label: "Spell Bonus (items, feats)",
			Value: pages.SignedNumber(int(character.SpellBonusMisc)),
		})
		sheet.Spellcasting = []pages.SharedFact{
			{Label: "Spell Save DC", Value: derived.SpellSaveDC},
			{Label: "Spell Attack", Value: derived.SpellAttackBonus},
		}
	}
	return sheet
}
func sharedFacts(facts ...pages.SharedFact) []pages.SharedFact {
	kept := make([]pages.SharedFact, 0, len(facts))
	for _, fact := range facts {
		if strings.TrimSpace(fact.Value) == "" {
			continue
		}
		kept = append(kept, fact)
	}
	return kept
}
func countValue[T ~uint8 | ~uint16](count T) string {
	if count == 0 {
		return ""
	}
	return strconv.FormatUint(uint64(count), 10)
}
func flagValue(on bool) string {
	if !on {
		return ""
	}
	return "Yes"
}
func sharedBonuses(rows []pages.BonusRow) []pages.SharedBonus {
	bonuses := make([]pages.SharedBonus, 0, len(rows))
	for _, row := range rows {
		bonuses = append(bonuses, pages.SharedBonus{
			Label:       row.Label,
			Abbr:        row.Abbr,
			Proficiency: dnd.ProficiencyLabel(row.Proficiency),
			Total:       row.Total,
		})
	}
	return bonuses
}
func sharedAttacks(rows []queries.Attack) []pages.SharedAttack {
	attacks := make([]pages.SharedAttack, 0, len(rows))
	for _, row := range rows {
		attack := pages.SharedAttack{
			Name:       strings.TrimSpace(row.Name),
			Bonus:      strings.TrimSpace(row.AttackBonus),
			Damage:     strings.TrimSpace(row.Damage),
			DamageType: strings.TrimSpace(row.DamageType),
			Mastery:    strings.TrimSpace(row.Mastery),
			Notes:      row.Notes,
		}
		if attack == (pages.SharedAttack{}) {
			continue
		}
		if attack.Name == "" {
			attack.Name = "Unnamed attack"
		}
		attacks = append(attacks, attack)
	}
	return attacks
}
func sharedItems(rows []queries.Inventory) []pages.SharedItem {
	items := make([]pages.SharedItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, pages.SharedItem{
			Name:        fallbackString(strings.TrimSpace(row.Name), "Unnamed item"),
			Quantity:    strconv.FormatUint(uint64(row.Quantity), 10),
			Value:       strings.TrimSpace(row.Value),
			Weight:      formatInventoryWeight(row.Weight),
			Equipped:    row.Equipped,
			Description: row.Description,
		})
	}
	return items
}
func sharedFeatures(rows []pages.Feature) []pages.SharedFact {
	features := make([]pages.SharedFact, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.Name) == "" && strings.TrimSpace(row.Value) == "" {
			continue
		}
		features = append(features, pages.SharedFact{
			Label: fallbackString(strings.TrimSpace(row.Name), "Unnamed feature"),
			Value: row.Value,
		})
	}
	return features
}
func sharedSpellBook(rows []queries.Spell, levels []pages.SpellLevel) []pages.SharedSpellLevel {
	slots := make(map[int]pages.SpellLevel, len(levels))
	for _, level := range levels {
		slots[level.Level] = level
	}
	book := make([]pages.SharedSpellLevel, 0, pages.MaxSpellLevel+1)
	for level := 0; level <= pages.MaxSpellLevel; level++ {
		entry := pages.SharedSpellLevel{Name: pages.SpellLevelName(level)}
		for _, row := range rows {
			if int(row.Level) == level {
				entry.Spells = append(entry.Spells, sharedSpell(spellPageRow(row)))
			}
		}
		known, counted := slots[level]
		if len(entry.Spells) == 0 && (!counted || known.Slots == "0") {
			continue
		}
		if counted {
			entry.Slots = pages.SpellSlotsLeft(known)
		}
		book = append(book, entry)
	}
	return book
}
func sharedSpell(spell pages.Spell) pages.SharedSpell {
	return pages.SharedSpell{
		Name:        fallbackString(strings.TrimSpace(spell.Name), "Unnamed spell"),
		Prepared:    spell.Prepared,
		CastingTime: strings.TrimSpace(spell.CastingTime),
		School:      spell.School,
		Range:       strings.TrimSpace(spell.CastingRange),
		Duration:    strings.TrimSpace(spell.Duration),
		Components:  strings.TrimSpace(spell.Components),
		Description: spell.Description,
	}
}
