package pages

// The bonus grids, after the derivation pass. A skill or saving throw bonus is
// an ability modifier plus what a proficiency state grants plus a misc bonus,
// and the first of those three is not stored anywhere on this panel -- it comes
// off the ability scores. So the row renders two controls and one number, and
// the number is not an input.

// The four proficiency states, and the whole reason this panel changed. Half is
// here for Jack of All Trades and the handful of features shaped like it;
// Expertise doubles the bonus and is what a rogue's chosen skills carry.
//
// The empty state is spelled "none" rather than "" because it is stored: a
// missing key and a key set to none mean the same thing, and one of them is what
// a sheet nobody has touched holds.
const (
	ProficiencyNone       = "none"
	ProficiencyHalf       = "half"
	ProficiencyProficient = "proficient"
	ProficiencyExpertise  = "expertise"
)

var proficiencyOptions = []Option{
	{Label: "—", Value: ProficiencyNone},
	{Label: "Half", Value: ProficiencyHalf},
	{Label: "Proficient", Value: ProficiencyProficient},
	{Label: "Expertise", Value: ProficiencyExpertise},
}

// NormalizeProficiency is the allowlist, and it answers anything it does not
// recognise with none -- the same shape NormalizeDamageType has, and for the
// same reason: the select offers nothing else, so an unrecognised value is a
// hand-built request rather than a typo worth an error message.
func NormalizeProficiency(value string) string {
	for _, option := range proficiencyOptions {
		if option.Value == value {
			return value
		}
	}

	return ProficiencyNone
}

// BonusEntry is one row of one grid: which key it stores under, which ability
// modifier it starts from, and what it is called.
//
// ABILITY IS A FIELD RATHER THAN A LOWERCASED Abbr, even though every value
// today is exactly that. The abbreviation is what the row prints and the ability
// is what the arithmetic reads, and a sheet where those two disagree is a normal
// thing to want -- a skill that keys off a different ability is a common
// feature, and the day one exists here the two must be able to differ.
type BonusEntry struct {
	Key     string
	Ability string
	Abbr    string
	Label   string
}

// BonusRow is one row already computed, for the markup. Misc is a string like
// every other input value on the page; Total is not an input at all.
type BonusRow struct {
	Key         string
	Label       string
	Abbr        string
	Proficiency string
	Misc        string
	Total       string
}

// SkillEntries and SavingThrowEntries are read by the controller, which needs
// the ability each row keys off to compute its total, and by the templates,
// which need the labels. One list each, in one package, because two would drift
// the day a skill moved.
func SkillEntries() []BonusEntry { return skills }

func SavingThrowEntries() []BonusEntry { return savingThrows }

// PassivePerceptionBase is the 10 a passive score is built on. It is here rather
// than inline in the arithmetic because it is the one number in that sum that is
// not a bonus.
const PassivePerceptionBase = 10

// SpellSaveDCBase is the 8 a spell save DC is built on, the way
// PassivePerceptionBase is the 10 a passive score is. Both are here so the
// arithmetic reads as a sum of named things.
const SpellSaveDCBase = 8

// PerceptionKey is the skill a passive perception is derived from. Naming it
// keeps the derivation from depending on a string literal that matches an entry
// in the list above only by luck.
const PerceptionKey = "perception"
