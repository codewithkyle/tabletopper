package controllers

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"tabletopper/internal/queries"
	"tabletopper/templ/pages"
)

// What a shared character link opens, and the one place that decides what a
// stranger is shown. share.go owns the token, the password and the miss;
// character-share.go is the owner minting the link; this file is the sheet.
//
// IT IS THE CHARACTER TAB, READ-ONLY, AND THE SCOPE IS DELIBERATE. The editor
// has five tabs and this mirrors one of them, so a link hands over what that
// page shows and nothing that lives behind the other four: the inventory tab is
// represented by the items ticked as equipped, the spells tab by the ones ticked
// as prepared, and the journal not at all -- an entry is shared by its own link,
// with its own expiry and its own password, and a sheet that listed them would
// be a way around all three.
//
// THE READ IS THE EDITOR'S OWN GetCharacter AND THE NARROWING HAPPENS HERE. The
// alternative was a statement selecting the forty-odd columns this page prints,
// which is what GetSharedJournalEntry does for the five the journal banner
// prints -- but the derived numbers are worked out from a dozen of those columns
// by characterDerived, and a second statement would either feed the same
// arithmetic or duplicate it. So the boundary is sharedCharacterSheet below:
// every value on the page is a string written down here by name, and a column
// added to the row reaches a reader only when somebody adds a line to this file.
//
// AN EMPTY VALUE IS NOT RENDERED AT ALL, which is the one place this stops
// mirroring the editor. The editor shows a box for every field because it is
// talking to the person who can fill it in; a reader cannot, so a zero
// exhaustion, a blank hit dice pool and an unweighed temporary hit point are
// absences here rather than boxes reading 0. The exceptions are the readings a
// table wants even at zero -- armour class, hit points, the ability scores --
// which are emitted whatever they say.

// sharedCharacterSheet renders the shared sheet. It runs past the password gate
// in SharePage and takes the grant rather than re-reading it, so the question is
// asked exactly once per request.
//
// EVERY ID IT QUERIES WITH COMES OFF THE GRANT. The character is grant.
// ResourceID and the owner is grant.OwnerID, and the four list statements below
// are the editor's own -- they are already scoped by character and owner, which
// is exactly the pair a share row carries, so a link cannot be edited into
// asking for a character it does not name.
func (a *App) sharedCharacterSheet(w http.ResponseWriter, r *http.Request, token string, grant queries.GetShareByTokenRow) {
	ctx := r.Context()

	character, err := a.Queries.GetCharacter(ctx, queries.GetCharacterParams{
		ID:      grant.ResourceID,
		OwnerID: grant.OwnerID,
	})
	// The character was deleted after the link went out. Deleting one takes its
	// shares with it, so this is a race rather than a steady state -- and it
	// reads as a dead link, which is what it is.
	if errors.Is(err, sql.ErrNoRows) {
		shareUnavailable(w, r)
		return
	}
	if err != nil {
		slog.Error("Failed to load shared character", "error", err)
		redirectToError(w, r)
		return
	}

	attacks, err := a.Queries.ListCharacterAttacks(ctx, queries.ListCharacterAttacksParams{
		CharacterID: grant.ResourceID,
		OwnerID:     grant.OwnerID,
	})
	if err != nil {
		slog.Error("Failed to load shared character attacks", "error", err)
		redirectToError(w, r)
		return
	}

	equipped, err := a.Queries.ListEquippedInventory(ctx, queries.ListEquippedInventoryParams{
		CharacterID: grant.ResourceID,
		OwnerID:     grant.OwnerID,
	})
	if err != nil {
		slog.Error("Failed to load shared character equipment", "error", err)
		redirectToError(w, r)
		return
	}

	prepared, err := a.Queries.ListPreparedSpells(ctx, queries.ListPreparedSpellsParams{
		CharacterID: grant.ResourceID,
		OwnerID:     grant.OwnerID,
	})
	if err != nil {
		slog.Error("Failed to load shared character spells", "error", err)
		redirectToError(w, r)
		return
	}

	levels, ok := a.loadSpellLevels(w, r, grant.ResourceID, grant.OwnerID)
	if !ok {
		return
	}

	sheet := sharedCharacterSheet(character, attacks, equipped, prepared, levels)
	if character.AssetID != nil {
		sheet.Avatar = "/share/" + token + "/avatar"
	}

	shareHeaders(w)
	render(w, r, pages.SharedCharacterPage(sheet))
}

// sharedCharacterSheet is the boundary: forty-odd values named one at a time,
// and a column that is not named here does not leave the process.
func sharedCharacterSheet(
	character queries.Character,
	attacks []queries.Attack,
	equipped []queries.Inventory,
	prepared []queries.Spell,
	levels []pages.SpellLevel,
) pages.SharedCharacterSheet {
	derived := characterDerived(character)
	header := characterHeaderFrom(character, derived)

	// AvatarID names /assets/images/{id}, which needs a session and would render
	// as a broken picture for every reader. The portrait comes from the share's
	// own route instead, which the caller fills in.
	header.AvatarID = ""

	ability := pages.NormalizeSpellcastingAbility(string(character.SpellcastingAbility))
	casts := ability != pages.SpellcastingAbilityNone

	sheet := pages.SharedCharacterSheet{
		Header: header,
		Identity: sharedFacts(
			pages.SharedFact{Label: "Species", Value: nullStringValue(character.Race)},
			pages.SharedFact{Label: "Class", Value: nullStringValue(character.Classes)},
			pages.SharedFact{Label: "Background", Value: nullStringValue(character.Background)},
			pages.SharedFact{Label: "Alignment", Value: pages.AlignmentLabel(nullStringValue(character.Alignment))},
			pages.SharedFact{Label: "Size", Value: pages.SizeLabel(strings.TrimSpace(character.Size))},
		),
		CoreStats: sharedFacts(
			pages.SharedFact{Label: "Experience", Value: strconv.FormatUint(uint64(character.XP), 10)},
			pages.SharedFact{Label: "Speed", Value: fallbackString(strings.TrimSpace(character.Speed), "30 ft.")},
			pages.SharedFact{Label: "Armor Class", Value: strconv.FormatUint(uint64(character.AC), 10)},
			pages.SharedFact{Label: "Initiative Bonus", Value: pages.SignedNumber(int(character.InitiativeBonus))},
			pages.SharedFact{Label: "Spellcasting Ability", Value: pages.SpellcastingAbilityLabel(ability)},
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
		Attacks:    sharedAttacks(attacks),
		Features:   sharedFeatures(parseFeatures(character.Features)),
		Equipped:   sharedItems(equipped),
		SpellSlots: sharedSpellLevels(levels),
		Prepared:   sharedSpellGroups(prepared),
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

	// The two spell numbers and the bonus that feeds them, for a character who
	// casts. spellNumbers answers a dash for one who does not, and a strip
	// reading "Spell Save DC —" beside "Spell Attack —" is two boxes saying the
	// same nothing the absent Spellcasting Ability above already said.
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

// sharedFacts drops the ones with nothing in them, which is what makes every
// panel above a list of what this character actually has rather than a form with
// the inputs taken off.
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

// countValue renders a counter that means something only when it is not zero --
// spent hit dice, exhaustion, death saves, temporary hit points -- and renders
// nothing when it is, so sharedFacts drops the box.
func countValue[T ~uint8 | ~uint16](count T) string {
	if count == 0 {
		return ""
	}

	return strconv.FormatUint(uint64(count), 10)
}

// flagValue is countValue for the one boolean on the sheet. False is absent
// rather than the word No, for the same reason: a reader is looking for what
// this character has.
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
			Label: row.Label,
			Abbr:  row.Abbr,
			Total: row.Total,
		})
	}

	return bonuses
}

// sharedAttacks drops a row with nothing on it, which is what an attack looks
// like between being added and being filled in. The editor renders it as an
// empty form to type into; here it would be a card with a fallback name and no
// other content.
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

// sharedItems is the equipped inventory. Quantity is rendered only when it is
// not one, the way the editor's equipped list renders it, so a sword does not
// read "Longsword × 1".
func sharedItems(rows []queries.Inventory) []pages.SharedItem {
	items := make([]pages.SharedItem, 0, len(rows))
	for _, row := range rows {
		item := pages.SharedItem{
			Name:        fallbackString(strings.TrimSpace(row.Name), "Unnamed item"),
			Description: row.Description,
		}
		if row.Quantity != 1 {
			item.Quantity = strconv.FormatUint(uint64(row.Quantity), 10)
		}

		items = append(items, item)
	}

	return items
}

// sharedFeatures reuses SharedFact, because a feature is a name and some text
// and that is what a fact is. A row with neither is dropped for the reason an
// empty attack is.
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

func sharedSpellGroups(rows []queries.Spell) []pages.SharedSpellGroup {
	groups := make([]pages.SharedSpellGroup, 0, pages.MaxSpellLevel+1)
	for _, group := range preparedSpellGroups(rows) {
		spells := make([]pages.SharedSpell, 0, len(group.Spells))
		for _, spell := range group.Spells {
			spells = append(spells, pages.SharedSpell{
				Name:        fallbackString(strings.TrimSpace(spell.Name), "Unnamed spell"),
				Meta:        pages.SpellMetaLine(spell),
				Description: spell.Description,
			})
		}

		groups = append(groups, pages.SharedSpellGroup{Name: group.Name, Spells: spells})
	}

	return groups
}

// sharedSpellLevels is the slots panel without the used count -- see
// pages.SharedSpellLevel for why that one number is left out.
//
// A LEVEL WITH NO SLOTS AND NO SPELLS IS NOT A LEVEL THIS CHARACTER HAS. Ten are
// always built, because nothing seeds the table and the editor renders all of
// them so any can be filled in; here the empty ones are dropped, so a level 3
// caster's panel stops at three rather than trailing six rows of zero.
func sharedSpellLevels(levels []pages.SpellLevel) []pages.SharedSpellLevel {
	active := make([]pages.SharedSpellLevel, 0, len(levels))
	for _, level := range levels {
		if level.Count == 0 && level.Slots == "0" {
			continue
		}

		active = append(active, pages.SharedSpellLevel{
			Name:   pages.SpellLevelName(level.Level),
			Slots:  slotCountLabel(level),
			Spells: pages.SpellCountLabel(level.Count),
		})
	}

	return active
}

// slotCountLabel is what a level says it has. Cantrips are unlimited and say so
// rather than reading "0 slots", which is the editor's answer to the same thing.
func slotCountLabel(level pages.SpellLevel) string {
	if level.Level == 0 {
		return "Unlimited"
	}
	if level.Slots == "1" {
		return "1 slot"
	}

	return level.Slots + " slots"
}
