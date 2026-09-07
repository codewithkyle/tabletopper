package controllers

import (
	"strconv"
	"strings"

	"tabletopper/internal/queries"
	"tabletopper/templ/pages"
)

// The 5e arithmetic this app derives rather than asks for. On a character sheet
// level follows from XP and proficiency follows from level; on a stat block both
// follow from the challenge rating instead. Everything after that is shared:
// every bonus on the two grids follows from an ability score, a proficiency
// state and a misc bonus, every passive score is ten plus one of those, and
// initiative is Dexterity plus what items and feats add. None of it is a field
// anybody sets and none of it is stored.
//
// THE CHARACTER ARITHMETIC IS FIRST AND THE MONSTER ARITHMETIC IS AFTER IT, and
// the middle of the file -- abilityModifier, proficiencyGrant, bonusRows,
// skillTotal -- is the part both halves call. That sharing is the reason the
// monster panels could reuse the sheet's bonus grids unchanged: the same
// components post them, so the same function totals them.

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
		PassivePerception: strconv.Itoa(pages.PassiveScoreBase + skillTotal(skills, pages.PerceptionKey)),
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
			Abbr:        governingAbbr(entry),
			Proficiency: state,
			Misc:        strconv.Itoa(misc[entry.Key]),
			Total:       pages.SignedNumber(total),
		})
	}

	return rows
}

// governingAbbr is the ability a row keys off, printed under its name -- and it
// is empty when that ability IS the row.
//
// The abbreviation earns its place on a skill: Acrobatics reads DEX, and which
// ability a skill runs on is the thing you have to know and cannot infer. On a
// saving throw it was the label abbreviated, set directly under the label --
// "Strength" over "STR" -- which is six wasted rows in a column where every row
// costs height.
//
// IT IS DERIVED RATHER THAN SWITCHED ON THE GRID. Saving throws are the grid
// where Ability and Key are the same word, and they are that way by
// construction rather than by coincidence; a homebrew save keyed off a
// different ability would want its abbreviation back, and would get it here
// without anybody adding a case for it. See BonusEntry for why those two are
// separate fields at all.
func governingAbbr(entry pages.BonusEntry) string {
	if entry.Ability == entry.Key {
		return ""
	}

	return entry.Abbr
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

// THE MONSTER ARITHMETIC.
//
// A stat block derives more than a character sheet does and stores less. The old
// monsters table held xp beside cr and both grids as free text somebody had
// already added up; what is stored now is the rating, the six scores, a misc
// bonus per row and a proficiency state per row, and every number the block
// prints is worked out from those on the way to the page.

// crZeroArmedXP is the second answer for CR 0, and the one rating in the rules
// with two. A creature of no rating is worth nothing to fight unless it can
// actually hurt somebody, in which case it is worth ten -- so the block asks
// whether the monster has an Actions section rather than storing which kind of
// CR 0 it is. That is the closest an editor can get to the question the rules
// are asking, and it costs a GM nothing to answer: they type the Bite either
// way.
const crZeroArmedXP = 10

// challengeRating looks a rating up and hands back the three numbers that follow
// from it: its XP, its proficiency bonus, and the XP of the rating above it,
// which is what a monster with lair actions is worth inside its lair.
//
// The next rating's XP is 0 at CR 30, which has nothing above it to be harder
// than -- and 0 is unambiguous there because it is the XP of CR 0, which is not
// above anything either.
//
// An unrecognised rating reads as CR 0, which is what NormalizeChallengeRating
// answers with as well, so a row written around the validator prints as the
// weakest thing in the book rather than as a monster with no numbers at all.
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

// monsterDerived computes every number the editor and the stat block show but do
// not store. It takes the whole row for the reason characterDerived does -- the
// arithmetic needs a dozen columns and the row is what both callers have -- and
// it takes the action rows as well, for two rules that depend on which sections
// exist rather than on what is in them: CR 0 is worth ten XP when the monster has
// an action, and the in-lair figure is printed only when it has a lair action.
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

	// The only rating worth nothing is CR 0, so this is that rule and reaches no
	// other row.
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

// monsterModifiers is the six ability modifiers under the keys both grids use,
// which is the map bonusRows reads.
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

// hasActionOfKind answers whether a section has anything in it. Two of the stat
// block's numbers turn on that and on nothing else about the rows.
func hasActionOfKind(actions []queries.MonsterAction, kind string) bool {
	for _, action := range actions {
		if string(action.Kind) == kind {
			return true
		}
	}

	return false
}

// monsterStatBlock builds the block every render reads: the editor's left
// column, the manual's View dialog, and the lookup a pawn will make. It is
// sharedCharacterSheet's shape and it is that shape for the same reason -- every
// value is a string written down here by name, so a column added to the row
// later reaches a reader only if somebody puts it on this struct on purpose.
func monsterStatBlock(monster queries.Monster, actions []queries.MonsterAction, derived pages.MonsterDerived) pages.StatBlock {
	image := ""
	if monster.AssetID != nil {
		image = monster.AssetID.String()
	}

	return pages.StatBlock{
		Name:     monster.Name,
		Subtitle: monsterSubtitle(monster),
		ImageID:  image,

		AC: strconv.FormatUint(uint64(monster.AC), 10),
		// The 2024 block prints the modifier and the passive score together,
		// which is why these two fields are one string rather than two: nothing
		// prints either half on its own.
		Initiative: derived.Initiative + " (" + derived.PassiveInitiative + ")",
		HP:         strconv.FormatUint(uint64(monster.HP), 10),
		HitDice:    strings.TrimSpace(monster.HitDice),
		Speed:      fallbackString(monster.Speed, "30 ft."),

		Abilities: statBlockAbilities(monster, derived),
		Lines:     statBlockLines(monster, derived),
		Sections:  statBlockSections(monster, actions),

		Habitat:  strings.TrimSpace(monster.Habitat),
		Treasure: strings.TrimSpace(monster.Treasure),
	}
}

// statBlockAbilities is the six-row table the 2024 block leads with: the score,
// the modifier, and the saving throw. The saves are a column here rather than a
// line of their own, which is the change that let the free-text saving_throws
// column go.
//
// IT RANGES OVER THE SAVING THROW GRID'S OWN ENTRIES rather than listing the six
// abilities again. That is what keeps the table in the same order as the panel
// that feeds it and keyed the same way, and it is why a save is read out of the
// computed grid rather than added up a second time here.
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

// statBlockLines is the labelled block between the ability table and the
// sections. THE OMISSIONS ARE THE POINT OF IT: a monster that resists nothing
// has no Resistances line at all, because the printed block does not carry empty
// headings. Two lines are always there -- Senses ends in a passive score every
// monster has, and Languages says "None" rather than going missing, because a
// creature that cannot speak is a fact about it.
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

// statBlockSkills is the Skills line: only the rows a GM has actually given the
// monster, which is what the book prints. A stat block listing all eighteen
// skills at their ability modifier would be printing the ability table again,
// eighteen rows wide.
//
// A row is on the line when it has a proficiency state or a misc bonus. The misc
// is compared as the string the grid rendered it as, because that is what the
// row carries -- and "0" is the only way a grid writes down no bonus.
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

// challengeRatingLine is the rating with everything that follows from it in
// brackets after it: `5 (XP 1,800; PB +3)`, and for a monster with a lair,
// `17 (XP 18,000, or 20,000 in lair; PB +6)`. None of the three numbers is
// stored, which is why they are printed beside the one that is.
func challengeRatingLine(monster queries.Monster, derived pages.MonsterDerived) string {
	xp := "XP " + derived.XP
	if derived.InLairXP != "" {
		xp += ", or " + derived.InLairXP + " in lair"
	}

	return pages.ChallengeRatingLabel(pages.NormalizeChallengeRating(monster.CR)) +
		" (" + xp + "; PB " + derived.Proficiency + ")"
}

// statBlockSections is Traits through Regional Effects, in the order the book
// prints them, with the empty ones left out. It ranges over the section list
// rather than switching on the seven kinds, so a section added to the ENUM
// appears here with nothing to write.
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

// sectionIntro is the sentence a section opens with. Six of the seven print the
// one written beside the section; Legendary Actions prints the uses count in
// front of it, because how many uses a monster has is a column and the rest of
// the sentence is not.
//
// A count of zero prints no sentence at all rather than "Legendary Action Uses:
// 0", which would be a stat block saying a monster has legendary actions it
// cannot take. The in-lair parenthetical is dropped the same way, and that is
// what a zero in that column means: the count does not change in the lair.
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

// monsterHeader builds the bar across the top of the editor: the picture, the
// name, the subtitle and six chips. It takes the derived values rather than
// working them out, so the chips and the stat block below them cannot disagree
// about a monster's initiative.
func monsterHeader(monster queries.Monster, derived pages.MonsterDerived) pages.MonsterHeader {
	image := ""
	if monster.AssetID != nil {
		image = monster.AssetID.String()
	}

	return pages.MonsterHeader{
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

// monsterSubtitle is the line under the name: `Small Humanoid (Goblinoid),
// Chaotic Neutral`. Size and type are one phrase, the tags follow in brackets
// when there are any, and the alignment is a clause of its own -- which is how
// both editions print it.
//
// UNALIGNED IS PRINTED HERE AND HIDDEN ON A CHARACTER, and the difference is who
// chose it. A character's alignment starts NULL and the editor falls back to
// unaligned, so it is the answer nobody gave; a beast is unaligned because the
// book says so, and the line is one of the few facts a stat block gives about
// what the thing is.
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

// labelOrDefault renders a stored choice through its label function, falling
// back to the label of the column's own default. The labels answer an
// unrecognised value with an empty string, which is right for a subtitle where
// the value is optional and wrong for these two: every monster is some size and
// some kind of thing, and a block reading "Humanoid" with no size in front of it
// looks like a rendering bug rather than a missing answer.
func labelOrDefault(label func(string) string, value, fallback string) string {
	if rendered := label(value); rendered != "" {
		return rendered
	}

	return label(fallback)
}

// bonusRowTotal reads one computed row's total back out by key. skillTotal
// parses its row back to an int for the arithmetic behind a passive score; this
// one wants the string the grid already rendered, because the block prints it.
func bonusRowTotal(rows []pages.BonusRow, key string) string {
	for _, row := range rows {
		if row.Key == key {
			return row.Total
		}
	}

	return pages.SignedNumber(0)
}

// formatXP puts the thousands separators in. The book prints 1,800 and 155,000,
// and an XP figure is the one number on a stat block long enough to be misread
// without them.
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
