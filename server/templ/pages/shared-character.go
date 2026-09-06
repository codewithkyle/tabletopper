package pages

// The shared character sheet, as a stranger sees it: the editor's Character tab
// with every control taken off it, and the counters that only mean anything
// mid-session left out.
//
// EVERY VALUE ON HERE IS A STRING THE CONTROLLER WROTE DOWN, and that is where
// the privacy boundary lives -- not in the statement behind it. The read is the
// same GetCharacter the editor runs, because the derived numbers are worked out
// from a dozen columns and a second narrower statement would mean a second copy
// of that arithmetic; what a stranger sees is decided one field at a time in
// sharedCharacterSheet, so a column added to the row later reaches this page
// only if somebody adds it here on purpose. SharedJournalData makes the same
// promise for the other share and keeps it a different way, because that page
// shows five values and this one shows most of a sheet.
//
// THERE IS NO CHARACTER ID ANYWHERE ON THIS STRUCT, and that is load-bearing
// rather than an oversight. Every read-only component the editor already has --
// equippedItems, preparedSpells, spellSlotsOverview -- takes one so its empty
// state can link back to the tab that fills it, and those links go to routes a
// signed-out reader cannot open. Withholding the id is what makes it impossible
// to render one here: not a rule about which components to reuse, which is a
// rule somebody has to remember, but an argument that does not exist.
//
// THE SECTIONS ARE THE PANELS OF THE CHARACTER TAB, IN ITS ORDER, and an empty
// one is not rendered at all. The editor prints "Nothing is equipped yet" over a
// link because it is talking to the person who can fix that; a reader cannot,
// so a sheet with no attacks has no Attacks panel rather than an empty one.

// SharedFact is one read-only label and value: an identity field, a vital, a
// feature, a line of personality. It is one type and not five because every one
// of them is a name and some text, and the panels differ in how they are laid
// out rather than in what they hold.
type SharedFact struct {
	Label string
	Value string
}

// SharedAbility is one of the six scores with the modifier worked out from it.
type SharedAbility struct {
	Label string
	Score string
	Mod   string
}

// SharedBonus is one skill or saving throw: what it is called, the ability it
// keys off, and the total. The proficiency and the misc bonus that produced the
// total are the editor's inputs and are not here -- a reader wants the number
// they would roll against, not the sum that made it.
type SharedBonus struct {
	Label string
	Abbr  string
	Total string
}

// SharedAttack is one row of the attacks table, id excluded.
type SharedAttack struct {
	Name       string
	Bonus      string
	Damage     string
	DamageType string
	Mastery    string
	Notes      string
}

// SharedItem is one equipped thing. Value and weight are on the inventory tab
// and are not on the Character tab, so they are not here either.
type SharedItem struct {
	Name        string
	Quantity    string
	Description string
}

// SharedSpell is one prepared spell. Meta is the casting time, range and
// duration already joined, because it is joined for the editor too and the
// joining is the only thing either page does with those three.
type SharedSpell struct {
	Name        string
	Meta        string
	Description string
}

// SharedSpellGroup is the prepared spells of one level, under that level's name.
type SharedSpellGroup struct {
	Name   string
	Spells []SharedSpell
}

// SharedSpellLevel is one level of the slots panel.
//
// THERE IS NO USED COUNT, which is the one omission that was asked for by name.
// How many slots a character has is a fact about them; how many are left is
// where they are in tonight's session, and a link pasted into a chat window is
// read hours after it was sent. The same argument would take the spent hit dice
// and the death saves, and those are kept: they sit in the Vitals panel beside
// the current hit points, which is the reading a table actually wants live.
type SharedSpellLevel struct {
	Name   string
	Slots  string
	Spells string
}

// SharedCharacterSheet is the whole page.
type SharedCharacterSheet struct {
	// Header is the six readings across the top, built by the same function the
	// editor's bar uses so the two cannot disagree about what a character's
	// initiative is.
	//
	// ITS AvatarID IS DELIBERATELY BLANK. That field names /assets/images/{id},
	// which needs a session and would render as a broken picture for every
	// reader; the portrait on this page comes from Avatar below, which is the
	// share's own route. Nothing here reads AvatarID -- sharedSheetBanner takes
	// the name and the subtitle off Header and the picture off Avatar.
	Header CharacterHeader

	// Avatar is the share-scoped URL of the portrait, empty when the character
	// has none. The initial underneath shows through either way, which is what
	// a reader sees if the object has gone from the bucket.
	Avatar string

	Identity     []SharedFact
	CoreStats    []SharedFact
	Spellcasting []SharedFact
	Vitals       []SharedFact
	Abilities    []SharedAbility
	SavingThrows []SharedBonus
	Skills       []SharedBonus

	// PassivePerception heads the skills panel the way it does in the editor,
	// which is why it is here rather than as another skill in the list.
	PassivePerception string

	Training   []SharedFact
	Attacks    []SharedAttack
	Features   []SharedFact
	Equipped   []SharedItem
	SpellSlots []SharedSpellLevel
	Prepared   []SharedSpellGroup

	Personality []SharedFact
	Appearance  []SharedFact
}

// SharedCharacterTitle is the <title> for a shared sheet: the character's name
// and nothing about the app around it.
func SharedCharacterTitle(name string) string {
	return characterBarName(CharacterHeader{Name: name}) + " | Tabletopper"
}
