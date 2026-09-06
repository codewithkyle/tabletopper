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
	// Spellcasting is an ability and a misc bonus now, not a save DC and an
	// attack bonus. Those two were numbers the player worked out and typed in;
	// these two are what they are worked out FROM, and Derived carries the
	// answer back.
	SpellcastingAbility string
	SpellBonusMisc      string
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
	Features           []Feature
	// Derived is every computed number on the page: the ability modifiers, both
	// bonus grids with their totals, the passive perception and the two spell
	// numbers. It replaced the two map[string]int fields the bonus grids used to
	// carry, which held a bonus somebody typed rather than one anything worked
	// out.
	Derived Derived
	// Attacks is the attacks table, rendered on this page as rows that are each
	// their own form. It is the one editable thing here that is not a panel and
	// not on a tab of its own: inventory and spells earned their own pages by
	// being long, and a character has three or four attacks that they read every
	// round of every fight.
	Attacks []Attack
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
