package pages

// CharacterID is the ULID as a string, and it is the only thing the editor
// needs to build a URL: every panel posts to /characters/<id>/<panel>.
type EditCharacterPageData struct {
	CharacterID   string
	Name          string
	Race          string
	Background    string
	Classes       string
	Size          string
	Alignment     string
	XP            string
	Languages     string
	Proficiencies string
	// The two details panels. They are ten plain strings and not a nested
	// struct because every other field on here is a plain string: the template
	// reads them the same way, and a struct would buy grouping the panel
	// headings already give.
	//
	// Nothing formats them and nothing falls back. An empty box is a box the
	// player has not filled in, which is a different thing from an empty
	// alignment -- that one gets DefaultAlignment because a select with nothing
	// chosen is broken, and a blank textarea is just blank.
	PersonalityTraits string
	Ideals            string
	Bonds             string
	Flaws             string
	Age               string
	Height            string
	Weight            string
	Eyes              string
	Skin              string
	Hair              string
	Str               string
	Dex               string
	Con               string
	Int               string
	Wis               string
	Cha               string
	AC                string
	Speed             string
	InitiativeBonus   string
	MaxHP             string
	CurrentHP         string
	TempHP            string
	SpellSaveDC       string
	SpellAtkBonus     string
	// The Vitals panel, and the one group on here that is not several of the
	// same type. The two counters render into a value attribute and so are
	// strings like every other number on this struct; the death saves are a
	// count the markup compares against, so they stay ints; and inspiration is
	// one bit that decides an attribute rather than fills one.
	HitDice            string
	HitDiceSpent       string
	DeathSaveSuccesses int
	DeathSaveFailures  int
	HeroicInspiration  bool
	Exhaustion         string
	Skills             map[string]int
	SavingThrows       map[string]int
	Features           []Feature
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
