package pages

// Derived is every number on the Character tab that is computed rather than
// typed. It is one struct and not a field per panel because it is rendered
// twice: once inside the page, where each value sits beside the controls that
// feed it, and once on its own after a save, where the same values go back as
// out-of-band swaps.
//
// THE SECOND RENDER IS WHY THESE VALUES ARE NOT STORED. A skill total depends on
// an ability score, a proficiency bonus that follows from experience, and the
// skill row itself -- three columns owned by three different panels. A stored
// total would be correct until any of the other two saved; a computed one is
// correct on every page load, and DerivedValues carries it to the ones already
// on screen.
type Derived struct {
	StrMod string
	DexMod string
	ConMod string
	IntMod string
	WisMod string
	ChaMod string

	Skills       []BonusRow
	SavingThrows []BonusRow

	PassivePerception string
	SpellSaveDC       string
	SpellAttackBonus  string
}

// The seven values spellcasting_ability can hold. "None" is first and is what a
// sheet starts as, because most characters cast nothing -- and a fighter whose
// spell save DC read 8 would be worse than one whose spell save DC reads a dash.
var spellcastingAbilityOptions = []Option{
	{Label: "None", Value: "none"},
	{Label: "Strength", Value: "str"},
	{Label: "Dexterity", Value: "dex"},
	{Label: "Constitution", Value: "con"},
	{Label: "Intelligence", Value: "int"},
	{Label: "Wisdom", Value: "wis"},
	{Label: "Charisma", Value: "cha"},
}

// SpellcastingAbilityNone is the stored value for a character who casts nothing,
// and the fallback NormalizeSpellcastingAbility answers with.
const SpellcastingAbilityNone = "none"

// NormalizeSpellcastingAbility is the allowlist behind the select. The column is
// an ENUM as well, which would refuse a bad value on its own -- as a 500. This
// is what keeps it from having to.
func NormalizeSpellcastingAbility(value string) string {
	for _, option := range spellcastingAbilityOptions {
		if option.Value == value {
			return value
		}
	}

	return SpellcastingAbilityNone
}
