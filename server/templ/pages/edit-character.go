package pages

// CharacterID is the ULID as a string, and it is the only thing the editor
// needs to build a URL: every panel posts to /characters/<id>/<panel>.
type EditCharacterPageData struct {
	CharacterID     string
	Name            string
	Race            string
	Background      string
	Classes         string
	Size            string
	Alignment       string
	XP              string
	Languages       string
	Proficiencies   string
	Str             string
	Dex             string
	Con             string
	Int             string
	Wis             string
	Cha             string
	AC              string
	Speed           string
	InitiativeBonus string
	MaxHP           string
	CurrentHP       string
	TempHP          string
	SpellSaveDC     string
	SpellAtkBonus   string
	Skills          map[string]int
	SavingThrows    map[string]int
	Features        []Feature
	// Equipped is the inventory rows ticked as equipped, rendered read-only on
	// the Character page. It is the only thing on that page that does not come
	// off the characters row, and the only thing on it with no form around it.
	Equipped []InventoryItem
	// Prepared is the spells ticked as prepared, grouped by level and rendered
	// read-only beside Equipment. Same shape and same reason: written down once
	// on the tab that owns it, read here.
	Prepared []PreparedSpellGroup
	// SpellSlots is all ten levels, for the panel beside Prepared Spells. It is
	// the only editable thing on this page that does not write a characters
	// column -- each level is its own little form posting to its own level.
	SpellSlots []SpellLevel
}
