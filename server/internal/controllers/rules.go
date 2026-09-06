package controllers

import (
	"strconv"
	"strings"

	"tabletopper/internal/queries"
	"tabletopper/templ/pages"
)

// The 5e arithmetic the character sheet derives rather than asks for. Level
// follows from XP and proficiency follows from level; every bonus on the two
// grids follows from an ability score, a proficiency state and a misc bonus; and
// the spell numbers follow from all of it. None of them is a field the player
// can set, and none of them is stored.

// xpThresholds[i] is the XP at which a character reaches level i+1.
var xpThresholds = [...]uint32{
	0,
	300,
	900,
	2700,
	6500,
	14000,
	23000,
	34000,
	48000,
	64000,
	85000,
	100000,
	120000,
	140000,
	165000,
	195000,
	225000,
	265000,
	305000,
	355000,
}

func levelFromXP(xp uint32) uint8 {
	for level := len(xpThresholds) - 1; level >= 0; level-- {
		if xp >= xpThresholds[level] {
			return uint8(level + 1)
		}
	}

	return 1
}

func proficiencyBonusForLevel(level uint8) uint16 {
	switch {
	case level <= 4:
		return 2
	case level <= 8:
		return 3
	case level <= 12:
		return 4
	case level <= 16:
		return 5
	}

	return 6
}

// abilityModifier is what every other derivation on this sheet starts from.
//
// IT IS score/2 - 5 AND NOT (score - 10)/2, which is how the rules phrase it and
// how it is usually written. Go truncates integer division toward zero, so the
// second form answers a score of 9 with 0 where the rules say -1, and is wrong
// for every odd score below 10 -- the scores a dump stat lands on. The first
// form cannot go wrong because the score is unsigned: the division never sees a
// negative number, and the subtraction happens after it.
func abilityModifier(score uint8) int {
	return int(score)/2 - 5
}

// proficiencyGrant is what a proficiency state adds on top of the ability
// modifier. Half rounds down, which is what the rules say and what Go's integer
// division already does for a proficiency bonus that is never negative.
func proficiencyGrant(state string, proficiencyBonus int) int {
	switch state {
	case pages.ProficiencyHalf:
		return proficiencyBonus / 2
	case pages.ProficiencyProficient:
		return proficiencyBonus
	case pages.ProficiencyExpertise:
		return proficiencyBonus * 2
	}

	return 0
}

// characterDerived computes every number the Character tab shows but does not
// store. It takes the whole row because that is what the arithmetic needs --
// six ability scores, a proficiency bonus, two misc blobs, two proficiency blobs
// and the spellcasting pair -- and because the row is what both callers have:
// the page render, and the out-of-band refresh a save sends back.
func characterDerived(character queries.Character) pages.Derived {
	modifiers := map[string]int{
		"str": abilityModifier(character.Str),
		"dex": abilityModifier(character.Dex),
		"con": abilityModifier(character.Con),
		"int": abilityModifier(character.Int),
		"wis": abilityModifier(character.Wis),
		"cha": abilityModifier(character.Cha),
	}
	proficiency := int(character.ProficiencyBonus)

	skills := bonusRows(
		pages.SkillEntries(),
		parseStatBonuses(character.Skills),
		parseProficiencies(character.SkillProficiencies),
		modifiers,
		proficiency,
	)
	saves := bonusRows(
		pages.SavingThrowEntries(),
		parseStatBonuses(character.SavingThrows),
		parseProficiencies(character.SavingThrowProficiencies),
		modifiers,
		proficiency,
	)

	derived := pages.Derived{
		StrMod:            pages.SignedNumber(modifiers["str"]),
		DexMod:            pages.SignedNumber(modifiers["dex"]),
		ConMod:            pages.SignedNumber(modifiers["con"]),
		IntMod:            pages.SignedNumber(modifiers["int"]),
		WisMod:            pages.SignedNumber(modifiers["wis"]),
		ChaMod:            pages.SignedNumber(modifiers["cha"]),
		Skills:            skills,
		SavingThrows:      saves,
		PassivePerception: strconv.Itoa(pages.PassivePerceptionBase + skillTotal(skills, pages.PerceptionKey)),
	}
	derived.SpellSaveDC, derived.SpellAttackBonus = spellNumbers(character, modifiers, proficiency)

	return derived
}

// bonusRows turns one grid's stored halves into rows the markup can print. The
// total is the whole point of the pass: ability modifier, plus what the
// proficiency state grants, plus whatever the player added by hand.
func bonusRows(entries []pages.BonusEntry, misc map[string]int, states map[string]string, modifiers map[string]int, proficiency int) []pages.BonusRow {
	rows := make([]pages.BonusRow, 0, len(entries))
	for _, entry := range entries {
		state := pages.NormalizeProficiency(states[entry.Key])
		total := modifiers[entry.Ability] + proficiencyGrant(state, proficiency) + misc[entry.Key]

		rows = append(rows, pages.BonusRow{
			Key:         entry.Key,
			Label:       entry.Label,
			Abbr:        entry.Abbr,
			Proficiency: state,
			Misc:        strconv.Itoa(misc[entry.Key]),
			Total:       pages.SignedNumber(total),
		})
	}

	return rows
}

// skillTotal reads one computed row back out, for passive perception. It parses
// the rendered string rather than recomputing, so a passive score can never
// disagree with the skill it is ten plus.
func skillTotal(rows []pages.BonusRow, key string) int {
	for _, row := range rows {
		if row.Key == key {
			total, err := strconv.Atoi(strings.TrimPrefix(row.Total, "+"))
			if err != nil {
				return 0
			}

			return total
		}
	}

	return 0
}

// spellNumbers renders the save DC and the attack bonus, or a dash for each. A
// character who casts nothing has no spell save DC, and printing the 8 that the
// arithmetic produces for one would be a number a fighter has to learn to
// ignore.
func spellNumbers(character queries.Character, modifiers map[string]int, proficiency int) (string, string) {
	ability := pages.NormalizeSpellcastingAbility(string(character.SpellcastingAbility))
	if ability == pages.SpellcastingAbilityNone {
		return "—", "—"
	}

	attack := proficiency + modifiers[ability] + int(character.SpellBonusMisc)

	return strconv.Itoa(pages.SpellSaveDCBase + attack), pages.SignedNumber(attack)
}
