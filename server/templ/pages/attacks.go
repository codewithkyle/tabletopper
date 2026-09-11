package pages
type Attack struct {
	ID   string
	Name string
	Bonus      string
	Damage     string
	DamageType string
	Mastery    string
	Notes      string
}
func AttackRowPanel(attackID string) string {
	return "attack-" + attackID
}
var damageTypeOptions = []Option{
	{Label: "—", Value: ""},
	{Label: "Acid", Value: "Acid"},
	{Label: "Bludgeoning", Value: "Bludgeoning"},
	{Label: "Cold", Value: "Cold"},
	{Label: "Fire", Value: "Fire"},
	{Label: "Force", Value: "Force"},
	{Label: "Lightning", Value: "Lightning"},
	{Label: "Necrotic", Value: "Necrotic"},
	{Label: "Piercing", Value: "Piercing"},
	{Label: "Poison", Value: "Poison"},
	{Label: "Psychic", Value: "Psychic"},
	{Label: "Radiant", Value: "Radiant"},
	{Label: "Slashing", Value: "Slashing"},
	{Label: "Thunder", Value: "Thunder"},
}
var masteryOptions = []Option{
	{Label: "—", Value: ""},
	{Label: "Cleave", Value: "Cleave"},
	{Label: "Graze", Value: "Graze"},
	{Label: "Nick", Value: "Nick"},
	{Label: "Push", Value: "Push"},
	{Label: "Sap", Value: "Sap"},
	{Label: "Slow", Value: "Slow"},
	{Label: "Topple", Value: "Topple"},
	{Label: "Vex", Value: "Vex"},
}
func NormalizeDamageType(value string) string {
	return normalizeChoice(value, damageTypeOptions)
}
func NormalizeMastery(value string) string {
	return normalizeChoice(value, masteryOptions)
}
func normalizeChoice(value string, options []Option) string {
	for _, option := range options {
		if option.Value == value {
			return value
		}
	}
	return ""
}
