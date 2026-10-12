package pages

import "tabletopper/internal/dnd"

type Derived struct {
	StrMod            string
	DexMod            string
	ConMod            string
	IntMod            string
	WisMod            string
	ChaMod            string
	Skills            []BonusRow
	SavingThrows      []BonusRow
	PassivePerception string
	SpellSaveDC       string
	SpellAttackBonus  string
}

var spellcastingAbilityOptions = dnd.SpellcastingAbilities()
