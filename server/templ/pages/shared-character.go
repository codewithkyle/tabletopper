package pages

type SharedFact struct {
	Label string
	Value string
}
type SharedAbility struct {
	Label string
	Score string
	Mod   string
}
type SharedBonus struct {
	Label string
	Abbr  string
	Total string
}
type SharedAttack struct {
	Name       string
	Bonus      string
	Damage     string
	DamageType string
	Mastery    string
	Notes      string
}
type SharedItem struct {
	Name        string
	Quantity    string
	Description string
}
type SharedSpell struct {
	Name        string
	Meta        string
	Description string
}
type SharedSpellGroup struct {
	Name   string
	Spells []SharedSpell
}
type SharedSpellLevel struct {
	Name   string
	Slots  string
	Spells string
}
type SharedCharacterSheet struct {
	Header            CharacterHeader
	Avatar            string
	Identity          []SharedFact
	CoreStats         []SharedFact
	Spellcasting      []SharedFact
	Vitals            []SharedFact
	Abilities         []SharedAbility
	SavingThrows      []SharedBonus
	Skills            []SharedBonus
	PassivePerception string
	Training          []SharedFact
	Attacks           []SharedAttack
	Features          []SharedFact
	Equipped          []SharedItem
	SpellSlots        []SharedSpellLevel
	Prepared          []SharedSpellGroup
	Personality       []SharedFact
	Appearance        []SharedFact
	Actions           SharedActions
}

func SharedCharacterTitle(name string) string {
	return characterBarName(CharacterHeader{Name: name}) + " | Tabletopper"
}
