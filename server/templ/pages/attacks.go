package pages

// Attack is one row of the Attacks panel, already formatted for the markup. ID
// is the ULID as a string because it lands in two attributes and a URL and never
// in arithmetic -- the shape InventoryItem's is, for the same reason.
type Attack struct {
	ID   string
	Name string
	// Bonus is the sheet's ATK/DC column and it is a string because that column
	// holds two different things: a longsword writes +7 and a save cantrip
	// writes DC 15. A number could hold one of them.
	Bonus      string
	Damage     string
	DamageType string
	Mastery    string
	Notes      string
}

// AttackRowPanel is the error-block id a row owns, built the way
// InventoryRowPanel is. The argument carries a ULID out of the URL, which is
// safe for the narrow reason that the handler parses it as a ULID before
// anything renders -- so it is 26 characters of Crockford base32 or the request
// never got here.
func AttackRowPanel(attackID string) string {
	return "attack-" + attackID
}

// The two closed sets an attack picks from, each with its empty member first
// because most rows want it: a fresh row has no damage type yet, and mastery is
// blank for an unarmed strike, for a spell, and for every character who has not
// been given the Weapon Mastery feature.
//
// They are Option slices rather than string slices with a separate list of
// labels, so the select and the validator read the same twelve lines. The
// normalisers below iterate them, which is what keeps a posted value that is not
// on the list from reaching the column.
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

// The eight weapon mastery properties the 2024 rules define. This is the one
// field on the sheet that did not exist before that edition, and it is why the
// attacks table is not just a name and a damage die: a Vex hit changes what the
// next attack rolls, so it is worth having in front of you.
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

// NormalizeDamageType and NormalizeMastery answer an unrecognised value with the
// empty member rather than with an error. That differs from NormalizeSpellSchool,
// which falls back to Evocation, and the difference is that a spell always has a
// school while an attack need not have either of these -- so "not on the list"
// and "not set" are the same answer here and there is no third state to invent.
//
// They are the real validator. The column is a VARCHAR rather than an ENUM
// precisely because this runs anyway: a select posts a string, something has to
// check it, and an ENUM would be a second copy of the list that answers a bad
// value with a 500 instead of a correction.
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
