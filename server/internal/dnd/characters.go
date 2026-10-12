package dnd

const (
	AbilityScoreLimit = 255
	CharacterXPLimit  = 9_999_999
	PassiveScoreBase  = 10
	SpellSaveDCBase   = 8
	PerceptionKey     = "perception"
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

func LevelFromXP(xp uint32) uint8 {
	for level := len(xpThresholds) - 1; level >= 0; level-- {
		if xp >= xpThresholds[level] {
			return uint8(level + 1)
		}
	}
	return 1
}
func ProficiencyBonus(level uint8) uint16 {
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
func AbilityModifier(score uint8) int {
	return int(score)/2 - 5
}

const (
	ProficiencyNone       = "none"
	ProficiencyHalf       = "half"
	ProficiencyProficient = "proficient"
	ProficiencyExpertise  = "expertise"
)

var proficiencies = []Option{
	{Label: "—", Value: ProficiencyNone},
	{Label: "Half", Value: ProficiencyHalf},
	{Label: "Proficient", Value: ProficiencyProficient},
	{Label: "Expertise", Value: ProficiencyExpertise},
}

func Proficiencies() []Option { return proficiencies }
func NormalizeProficiency(value string) string {
	return normalize(value, proficiencies, ProficiencyNone)
}
func ProficiencyLabel(value string) string {
	if value == ProficiencyNone {
		return ""
	}
	return label(value, proficiencies)
}
func ProficiencyGrant(state string, proficiencyBonus int) int {
	switch state {
	case ProficiencyHalf:
		return proficiencyBonus / 2
	case ProficiencyProficient:
		return proficiencyBonus
	case ProficiencyExpertise:
		return proficiencyBonus * 2
	}
	return 0
}

type BonusEntry struct {
	Key     string
	Ability string
	Abbr    string
	Label   string
}

var skills = []BonusEntry{
	{Key: "acrobatics", Ability: "dex", Abbr: "DEX", Label: "Acrobatics"},
	{Key: "animal_handling", Ability: "wis", Abbr: "WIS", Label: "Animal Handling"},
	{Key: "arcana", Ability: "int", Abbr: "INT", Label: "Arcana"},
	{Key: "athletics", Ability: "str", Abbr: "STR", Label: "Athletics"},
	{Key: "deception", Ability: "cha", Abbr: "CHA", Label: "Deception"},
	{Key: "history", Ability: "int", Abbr: "INT", Label: "History"},
	{Key: "insight", Ability: "wis", Abbr: "WIS", Label: "Insight"},
	{Key: "intimidation", Ability: "cha", Abbr: "CHA", Label: "Intimidation"},
	{Key: "investigation", Ability: "int", Abbr: "INT", Label: "Investigation"},
	{Key: "medicine", Ability: "wis", Abbr: "WIS", Label: "Medicine"},
	{Key: "nature", Ability: "int", Abbr: "INT", Label: "Nature"},
	{Key: "perception", Ability: "wis", Abbr: "WIS", Label: "Perception"},
	{Key: "performance", Ability: "cha", Abbr: "CHA", Label: "Performance"},
	{Key: "persuasion", Ability: "cha", Abbr: "CHA", Label: "Persuasion"},
	{Key: "religion", Ability: "int", Abbr: "INT", Label: "Religion"},
	{Key: "sleight_of_hand", Ability: "dex", Abbr: "DEX", Label: "Sleight of Hand"},
	{Key: "stealth", Ability: "dex", Abbr: "DEX", Label: "Stealth"},
	{Key: "survival", Ability: "wis", Abbr: "WIS", Label: "Survival"},
}
var savingThrows = []BonusEntry{
	{Key: "str", Ability: "str", Abbr: "STR", Label: "Strength"},
	{Key: "dex", Ability: "dex", Abbr: "DEX", Label: "Dexterity"},
	{Key: "con", Ability: "con", Abbr: "CON", Label: "Constitution"},
	{Key: "int", Ability: "int", Abbr: "INT", Label: "Intelligence"},
	{Key: "wis", Ability: "wis", Abbr: "WIS", Label: "Wisdom"},
	{Key: "cha", Ability: "cha", Abbr: "CHA", Label: "Charisma"},
}

func SkillEntries() []BonusEntry       { return skills }
func SavingThrowEntries() []BonusEntry { return savingThrows }

const SpellcastingAbilityNone = "none"

var spellcastingAbilities = []Option{
	{Label: "None", Value: SpellcastingAbilityNone},
	{Label: "Strength", Value: "str"},
	{Label: "Dexterity", Value: "dex"},
	{Label: "Constitution", Value: "con"},
	{Label: "Intelligence", Value: "int"},
	{Label: "Wisdom", Value: "wis"},
	{Label: "Charisma", Value: "cha"},
}

func SpellcastingAbilities() []Option { return spellcastingAbilities }
func NormalizeSpellcastingAbility(value string) string {
	return normalize(value, spellcastingAbilities, SpellcastingAbilityNone)
}
func SpellcastingAbilityLabel(value string) string {
	if value == SpellcastingAbilityNone {
		return ""
	}
	return label(value, spellcastingAbilities)
}
