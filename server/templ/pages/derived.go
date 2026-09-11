package pages


































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




var spellcastingAbilityOptions = []Option{
	{Label: "None", Value: "none"},
	{Label: "Strength", Value: "str"},
	{Label: "Dexterity", Value: "dex"},
	{Label: "Constitution", Value: "con"},
	{Label: "Intelligence", Value: "int"},
	{Label: "Wisdom", Value: "wis"},
	{Label: "Charisma", Value: "cha"},
}



const SpellcastingAbilityNone = "none"




func NormalizeSpellcastingAbility(value string) string {
	for _, option := range spellcastingAbilityOptions {
		if option.Value == value {
			return value
		}
	}

	return SpellcastingAbilityNone
}






func SpellcastingAbilityLabel(value string) string {
	if value == SpellcastingAbilityNone {
		return ""
	}

	for _, option := range spellcastingAbilityOptions {
		if option.Value == value {
			return option.Label
		}
	}

	return ""
}
