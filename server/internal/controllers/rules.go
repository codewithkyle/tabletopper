package controllers

import (
	"strconv"
	"strings"

	"tabletopper/internal/queries"
	"tabletopper/templ/pages"
)

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
func abilityModifier(score uint8) int {
	return int(score)/2 - 5
}
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
		PassivePerception: strconv.Itoa(pages.PassiveScoreBase + skillTotal(skills, pages.PerceptionKey)),
	}
	derived.SpellSaveDC, derived.SpellAttackBonus = spellNumbers(character, modifiers, proficiency)
	return derived
}
func bonusRows(entries []pages.BonusEntry, misc map[string]int, states map[string]string, modifiers map[string]int, proficiency int) []pages.BonusRow {
	rows := make([]pages.BonusRow, 0, len(entries))
	for _, entry := range entries {
		state := pages.NormalizeProficiency(states[entry.Key])
		total := modifiers[entry.Ability] + proficiencyGrant(state, proficiency) + misc[entry.Key]
		rows = append(rows, pages.BonusRow{
			Key:         entry.Key,
			Label:       entry.Label,
			Abbr:        governingAbbr(entry),
			Proficiency: state,
			Misc:        strconv.Itoa(misc[entry.Key]),
			Total:       pages.SignedNumber(total),
		})
	}
	return rows
}
func governingAbbr(entry pages.BonusEntry) string {
	if entry.Ability == entry.Key {
		return ""
	}
	return entry.Abbr
}
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
func spellNumbers(character queries.Character, modifiers map[string]int, proficiency int) (string, string) {
	ability := pages.NormalizeSpellcastingAbility(string(character.SpellcastingAbility))
	if ability == pages.SpellcastingAbilityNone {
		return "—", "—"
	}
	attack := proficiency + modifiers[ability] + int(character.SpellBonusMisc)
	return strconv.Itoa(pages.SpellSaveDCBase + attack), pages.SignedNumber(attack)
}

const crZeroArmedXP = 10

func challengeRating(value string) (xp uint32, proficiency uint8, nextXP uint32) {
	ratings := pages.ChallengeRatings()
	index := 0
	for i, rating := range ratings {
		if rating.Value == value {
			index = i
			break
		}
	}
	if index+1 < len(ratings) {
		nextXP = ratings[index+1].XP
	}
	return ratings[index].XP, ratings[index].Proficiency, nextXP
}
func monsterDerived(monster queries.Monster, actions []queries.MonsterAction) pages.MonsterDerived {
	modifiers := monsterModifiers(monster)
	xp, proficiencyBonus, nextXP := challengeRating(monster.CR)
	proficiency := int(proficiencyBonus)
	skills := bonusRows(
		pages.SkillEntries(),
		parseStatBonuses(monster.Skills),
		parseProficiencies(monster.SkillProficiencies),
		modifiers,
		proficiency,
	)
	saves := bonusRows(
		pages.SavingThrowEntries(),
		parseStatBonuses(monster.SavingThrows),
		parseProficiencies(monster.SavingThrowProficiencies),
		modifiers,
		proficiency,
	)
	if xp == 0 && hasActionOfKind(actions, pages.MonsterActionKindAction) {
		xp = crZeroArmedXP
	}
	inLairXP := ""
	if nextXP > 0 && hasActionOfKind(actions, pages.MonsterActionKindLairAction) {
		inLairXP = formatXP(nextXP)
	}
	initiative := modifiers["dex"] + int(monster.InitiativeBonus)
	return pages.MonsterDerived{
		StrMod:            pages.SignedNumber(modifiers["str"]),
		DexMod:            pages.SignedNumber(modifiers["dex"]),
		ConMod:            pages.SignedNumber(modifiers["con"]),
		IntMod:            pages.SignedNumber(modifiers["int"]),
		WisMod:            pages.SignedNumber(modifiers["wis"]),
		ChaMod:            pages.SignedNumber(modifiers["cha"]),
		Skills:            skills,
		SavingThrows:      saves,
		PassivePerception: strconv.Itoa(pages.PassiveScoreBase + skillTotal(skills, pages.PerceptionKey)),
		Initiative:        pages.SignedNumber(initiative),
		PassiveInitiative: strconv.Itoa(pages.PassiveScoreBase + initiative),
		Proficiency:       pages.SignedNumber(proficiency),
		XP:                formatXP(xp),
		InLairXP:          inLairXP,
	}
}
func monsterModifiers(monster queries.Monster) map[string]int {
	return map[string]int{
		"str": abilityModifier(monster.Str),
		"dex": abilityModifier(monster.Dex),
		"con": abilityModifier(monster.Con),
		"int": abilityModifier(monster.Int),
		"wis": abilityModifier(monster.Wis),
		"cha": abilityModifier(monster.Cha),
	}
}
func hasActionOfKind(actions []queries.MonsterAction, kind string) bool {
	for _, action := range actions {
		if string(action.Kind) == kind {
			return true
		}
	}
	return false
}
func monsterStatBlock(monster queries.Monster, actions []queries.MonsterAction, derived pages.MonsterDerived) pages.StatBlock {
	image := ""
	if monster.AssetID != nil {
		image = "/assets/images/" + monster.AssetID.String()
	}
	return pages.StatBlock{
		Name:       monster.Name,
		Subtitle:   monsterSubtitle(monster),
		Image:      image,
		AC:         strconv.FormatUint(uint64(monster.AC), 10),
		Initiative: derived.Initiative + " (" + derived.PassiveInitiative + ")",
		HP:         strconv.FormatUint(uint64(monster.HP), 10),
		HitDice:    strings.TrimSpace(monster.HitDice),
		Speed:      fallbackString(monster.Speed, "30 ft."),
		Abilities:  statBlockAbilities(monster, derived),
		Lines:      statBlockLines(monster, derived),
		Sections:   statBlockSections(monster, actions),
		Habitat:    strings.TrimSpace(monster.Habitat),
		Treasure:   strings.TrimSpace(monster.Treasure),
	}
}
func statBlockAbilities(monster queries.Monster, derived pages.MonsterDerived) []pages.StatBlockAbility {
	scores := map[string]uint8{
		"str": monster.Str,
		"dex": monster.Dex,
		"con": monster.Con,
		"int": monster.Int,
		"wis": monster.Wis,
		"cha": monster.Cha,
	}
	abilities := make([]pages.StatBlockAbility, 0, len(scores))
	for _, entry := range pages.SavingThrowEntries() {
		abilities = append(abilities, pages.StatBlockAbility{
			Label: entry.Abbr,
			Score: strconv.FormatUint(uint64(scores[entry.Key]), 10),
			Mod:   pages.SignedNumber(abilityModifier(scores[entry.Key])),
			Save:  bonusRowTotal(derived.SavingThrows, entry.Key),
		})
	}
	return abilities
}
func statBlockLines(monster queries.Monster, derived pages.MonsterDerived) []pages.StatBlockEntry {
	lines := make([]pages.StatBlockEntry, 0, 8)
	if skills := statBlockSkills(derived.Skills); skills != "" {
		lines = append(lines, pages.StatBlockEntry{Label: "Skills", Value: skills})
	}
	for _, line := range []pages.StatBlockEntry{
		{Label: "Vulnerabilities", Value: strings.TrimSpace(monster.Vulnerabilities)},
		{Label: "Resistances", Value: strings.TrimSpace(monster.Resistances)},
		{Label: "Immunities", Value: strings.TrimSpace(monster.Immunities)},
		{Label: "Gear", Value: strings.TrimSpace(monster.Gear)},
	} {
		if line.Value != "" {
			lines = append(lines, line)
		}
	}
	senses := "Passive Perception " + derived.PassivePerception
	if special := strings.TrimSpace(monster.Senses); special != "" {
		senses = special + ", " + senses
	}
	lines = append(lines, pages.StatBlockEntry{Label: "Senses", Value: senses})
	lines = append(lines, pages.StatBlockEntry{
		Label: "Languages",
		Value: fallbackString(monster.Languages, "None"),
	})
	return append(lines, pages.StatBlockEntry{Label: "CR", Value: challengeRatingLine(monster, derived)})
}
func statBlockSkills(rows []pages.BonusRow) string {
	listed := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Proficiency == pages.ProficiencyNone && row.Misc == "0" {
			continue
		}
		listed = append(listed, row.Label+" "+row.Total)
	}
	return strings.Join(listed, ", ")
}
func challengeRatingLine(monster queries.Monster, derived pages.MonsterDerived) string {
	xp := "XP " + derived.XP
	if derived.InLairXP != "" {
		xp += ", or " + derived.InLairXP + " in lair"
	}
	return pages.ChallengeRatingLabel(pages.NormalizeChallengeRating(monster.CR)) +
		" (" + xp + "; PB " + derived.Proficiency + ")"
}
func statBlockSections(monster queries.Monster, actions []queries.MonsterAction) []pages.StatBlockSection {
	sections := make([]pages.StatBlockSection, 0, len(pages.MonsterActionSections()))
	for _, section := range pages.MonsterActionSections() {
		entries := make([]pages.StatBlockEntry, 0, len(actions))
		for _, action := range actions {
			if string(action.Kind) != section.Kind {
				continue
			}
			entries = append(entries, pages.StatBlockEntry{
				Label: strings.TrimSpace(action.Name),
				Value: strings.TrimSpace(action.Description),
			})
		}
		if len(entries) == 0 {
			continue
		}
		sections = append(sections, pages.StatBlockSection{
			Heading: section.Heading,
			Intro:   sectionIntro(monster, section),
			Entries: entries,
		})
	}
	return sections
}
func sectionIntro(monster queries.Monster, section pages.MonsterActionSection) string {
	if section.Kind != pages.MonsterActionKindLegendaryAction || monster.LegendaryActionUses == 0 {
		return section.Intro
	}
	uses := "Legendary Action Uses: " + strconv.FormatUint(uint64(monster.LegendaryActionUses), 10)
	if monster.LegendaryActionUsesInLair > 0 {
		uses += " (" + strconv.FormatUint(uint64(monster.LegendaryActionUsesInLair), 10) + " in Lair)"
	}
	return uses + ". " + section.Intro
}
func monsterHeader(monster queries.Monster, derived pages.MonsterDerived) pages.MonsterHeader {
	image := ""
	if monster.AssetID != nil {
		image = monster.AssetID.String()
	}
	return pages.MonsterHeader{
		MonsterID:   monster.ID.String(),
		Name:        monster.Name,
		Subtitle:    monsterSubtitle(monster),
		ImageID:     image,
		AC:          strconv.FormatUint(uint64(monster.AC), 10),
		HP:          strconv.FormatUint(uint64(monster.HP), 10),
		Speed:       fallbackString(monster.Speed, "30 ft."),
		Initiative:  derived.Initiative,
		Proficiency: derived.Proficiency,
		CR:          pages.ChallengeRatingLabel(pages.NormalizeChallengeRating(monster.CR)),
	}
}
func monsterSubtitle(monster queries.Monster) string {
	size := labelOrDefault(pages.SizeLabel, monster.Size, pages.DefaultSize)
	creature := labelOrDefault(pages.CreatureTypeLabel, monster.Type, pages.DefaultCreatureType)
	descriptor := strings.TrimSpace(size + " " + creature)
	if tags := strings.TrimSpace(monster.Tags); tags != "" && descriptor != "" {
		descriptor += " (" + tags + ")"
	}
	parts := make([]string, 0, 2)
	if descriptor != "" {
		parts = append(parts, descriptor)
	}
	if alignment := pages.AlignmentLabel(monster.Alignment); alignment != "" {
		parts = append(parts, alignment)
	}
	return strings.Join(parts, ", ")
}
func labelOrDefault(label func(string) string, value, fallback string) string {
	if rendered := label(value); rendered != "" {
		return rendered
	}
	return label(fallback)
}
func bonusRowTotal(rows []pages.BonusRow, key string) string {
	for _, row := range rows {
		if row.Key == key {
			return row.Total
		}
	}
	return pages.SignedNumber(0)
}
func formatXP(xp uint32) string {
	digits := strconv.FormatUint(uint64(xp), 10)
	if len(digits) <= 3 {
		return digits
	}
	var formatted strings.Builder
	lead := len(digits) % 3
	if lead > 0 {
		formatted.WriteString(digits[:lead])
	}
	for at := lead; at < len(digits); at += 3 {
		if formatted.Len() > 0 {
			formatted.WriteByte(',')
		}
		formatted.WriteString(digits[at : at+3])
	}
	return formatted.String()
}
