package pages
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
func NormalizeProficiency(value string) string {
	for _, option := range proficiencyOptions {
		if option.Value == value {
			return value
		}
	}
	return ProficiencyNone
}
type BonusEntry struct {
	Key     string
	Ability string
	Abbr    string
	Label   string
}
type BonusRow struct {
	Key         string
	Label       string
	Abbr        string
	Proficiency string
	Misc        string
	Total       string
}
func SkillEntries() []BonusEntry { return skills }
func SavingThrowEntries() []BonusEntry { return savingThrows }
const PassiveScoreBase = 10
const SpellSaveDCBase = 8
const PerceptionKey = "perception"
