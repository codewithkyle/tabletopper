package pages

import "strings"

// The monster editor, and the stat block that sits beside it. It is the
// character sheet's shape -- a bar across the top, panels that autosave one at
// a time, and rows that are each their own form -- with one thing the sheet does
// not have: a rendered copy of what is being edited, redrawn out-of-band after
// every save.
//
// EVERY FIELD ON EVERY STRUCT HERE IS A STRING THE CONTROLLER WROTE DOWN, which
// is the same promise SharedCharacterSheet makes and it is made here for the
// same reason: the stat block is what the VTT will one day hand to a player, so
// what reaches it is decided one field at a time in the controller rather than
// by handing the row to a template. A column added to monsters later reaches a
// reader only if somebody adds it here on purpose.

// MonsterID is the ULID as a string, and it is all the editor needs to build a
// URL: every panel posts to /monsters/<id>/<panel> and every action row to
// /monsters/<id>/actions/<kind>/<row>.
type EditMonsterPageData struct {
	MonsterID string

	// Header is the bar across the top: the picture, the name, the subtitle and
	// six chips. Built by the same function and from the same derived values the
	// stat block is, so the two cannot disagree about what a monster's
	// initiative is.
	Header MonsterHeader

	// StatBlock is the left column, and the whole reason this page is a split
	// rather than a stack. It is the same component the manual's View dialog
	// renders, so what a GM sees while editing is what the table will see.
	StatBlock StatBlock

	Name      string
	Size      string
	Type      string
	Tags      string
	Alignment string

	Str string
	Dex string
	Con string
	Int string
	Wis string
	Cha string

	AC                        string
	HP                        string
	HitDice                   string
	Speed                     string
	InitiativeBonus           string
	CR                        string
	LegendaryActionUses       string
	LegendaryActionUsesInLair string

	Vulnerabilities string
	Resistances     string
	Immunities      string
	Gear            string
	Senses          string
	Languages       string

	Habitat     string
	Treasure    string
	Description string

	// Derived is every number on the page that is worked out rather than typed:
	// the six modifiers, both grids with their totals, the passive scores and
	// the three numbers that follow from the challenge rating. None of it is
	// stored -- see monsterDerived in the controller for why a stored copy
	// would go stale the moment another panel saved.
	Derived MonsterDerived

	// Actions is the seven sections' rows, keyed by the kind that names the
	// section. It is a map rather than seven fields because the page draws the
	// sections by ranging over MonsterActionSections() -- so a section cannot be
	// left off the page by forgetting to write its markup, and a kind added to
	// the ENUM appears here with no field to add.
	Actions map[string][]MonsterAction
}

// MonsterHeader is the bar. It is CharacterHeader's shape with monster chips:
// no current hit points, because the manual holds the stat block as printed and
// the wounded copy lives in the room; and a challenge rating, which is the one
// reading a GM checks before putting a monster in front of anyone.
type MonsterHeader struct {
	// MonsterID is here rather than read from EditMonsterPageData because the
	// bar's two halves are also rendered on their own, as the out-of-band
	// swaps a panel save answers with, and the upload control in the figure
	// needs the monster's own URL to post to.
	MonsterID string
	Name      string
	// Subtitle is size, type, tags and alignment as the book prints them, built
	// by the controller because it is the only place that knows which of the
	// four the row actually has.
	Subtitle string
	// ImageID is empty when the monster has no picture, and the bar falls back
	// to the initial the manual card already uses.
	ImageID string

	// The six chips. AC, hit points and speed are stored columns; initiative,
	// proficiency and the rating's own numbers are worked out.
	AC          string
	HP          string
	Speed       string
	Initiative  string
	Proficiency string
	CR          string
}

// MonsterDerived is every computed number on the editor, in one struct, for the
// reason Derived is one struct on the character sheet: it is rendered twice --
// inside the page beside the controls that feed it, and on its own after a save
// as out-of-band swaps -- and one component cannot disagree with itself.
type MonsterDerived struct {
	StrMod string
	DexMod string
	ConMod string
	IntMod string
	WisMod string
	ChaMod string

	// The two grids are the character sheet's grids, rows and all: the same
	// components post them and the same arithmetic totals them. What the stat
	// block does differently is print them -- the saves go in the ability
	// table's own column, and the skills line lists only the rows that have a
	// proficiency state or a bonus.
	Skills       []BonusRow
	SavingThrows []BonusRow

	PassivePerception string

	// Initiative is the modifier a monster rolls with -- Dexterity plus the
	// stored misc bonus -- and PassiveInitiative is ten plus that, which is what
	// the 2024 block prints in brackets after it.
	Initiative        string
	PassiveInitiative string

	// The three that follow from the challenge rating. XP is already formatted
	// with its thousands separators, because every field here is what gets
	// printed rather than what it was worked out from.
	//
	// InLairXP is EMPTY UNLESS THE MONSTER HAS A LAIR ACTION, which is what puts
	// "or 20,000 in lair" on the CR line and keeps it off every other monster's.
	// It is also empty at CR 30, which has no rating above it to be harder than.
	Proficiency string
	XP          string
	InLairXP    string
}

// StatBlock is the rendered monster: the manual's View dialog, the editor's left
// column, and the thing a pawn will open. Nothing on it is an input and nothing
// on it is a number -- see the note at the top of this file for why every value
// is a string somebody wrote down by name.
//
// IT IS ONE BODY IN TWO FRAMES, which is statBlockBody and the two components
// that call it. On the editor it is a panel on the desk like every other panel,
// so it carries the raised surface and the article the out-of-band swap
// replaces. In the dialog it carries neither: .modal-box is already that
// surface, and a panel drawn inside a panel is two hairlines and two shadows
// around one thing, with the outer one always a few millimetres from the inner.
// The dialog gets the body flush against its own background, and the insets
// inside -- the AC strip, the ability rows -- are what give it its structure.
//
// A THIRD FRAME IS THE SHARED PAGE, which is the editor's panel with nothing
// around it, and it is why Image below is a URL rather than an id.
//
// A FOURTH IS THE ROOM'S WINDOW, WHICH IS THE DIALOG MINUS ITS Close. A modal
// must ship a labelled way out beside its affirmative action, so
// MonsterStatBlockFragment ends in one; a window is dismissed by the corner
// controls on its own title bar, and a second Close at the bottom of the block
// scrolled with the content, sat under whatever the last legendary action was,
// and closed nothing -- it fired modal:close at a modal that was not open. The
// two frames are separate components rather than a flag because the difference
// is which surface is asking, and only the surface knows.
type StatBlock struct {
	Name     string
	Subtitle string
	// Image is the whole URL of the picture, empty when the monster has none.
	//
	// IT IS A URL AND NOT AN ID BECAUSE THE TWO CALLERS REACH IT DIFFERENTLY.
	// Inside the app it is /assets/images/{id}, which serves monster images to
	// any signed-in user for the reason it serves maps to them: what is on the
	// table is shown to the table. On a shared page there is nobody signed in,
	// so that route would render as a broken picture for every reader, and the
	// URL is the share's own portrait route instead. An id here would have made
	// the markup pick between them, which is a decision the markup cannot make.
	Image string

	// The three lines under the name. Initiative carries its passive score in
	// brackets, HitDice carries its own brackets, and each is empty when the
	// column behind it is.
	AC         string
	Initiative string
	HP         string
	HitDice    string
	Speed      string

	// The six scores, in the book's order, each with the modifier and the save
	// worked out. The 2024 block prints saves in this table rather than on a
	// line of their own, which is why there is no Saving Throws entry below.
	Abilities []StatBlockAbility

	// Lines is the labelled block between the ability table and the sections:
	// Skills, the three defense lines, Gear, Senses, Languages and CR. They are
	// one slice rather than nine fields because the block omits what a monster
	// has not -- so the controller decides what is on the list, and the markup
	// prints the list it is given rather than nine conditionals.
	Lines []StatBlockEntry

	// Sections is Traits through Regional Effects, each with its rows and the
	// sentence it opens with. A section with no rows is not in the slice.
	Sections []StatBlockSection

	// The two lines the 2024 book prints under the block rather than in it.
	Habitat  string
	Treasure string
}

// StatBlockAbility is one of the six scores with both numbers derived from it.
// Save is the modifier plus what the proficiency state grants plus a misc bonus,
// which is the saving-throw grid's own total for that row.
type StatBlockAbility struct {
	Label string
	Score string
	Mod   string
	Save  string
}

// StatBlockEntry is a label and some text, and it is one type for the two things
// on the block shaped that way -- a line like "Senses Darkvision 60 ft." and a
// row like "Bite. Melee Attack Roll: +7..." -- for the reason SharedFact is one
// type for five: every one of them is a name and a piece of prose, and they
// differ in how they are laid out rather than in what they hold.
//
// A regional effect has a blank Label, because the book prints those as a list
// without names, and the markup renders that as a bare paragraph.
type StatBlockEntry struct {
	Label string
	Value string
}

// StatBlockSection is one of the seven, already narrowed to what this monster
// has. Intro is the sentence the book opens the section with, and for Legendary
// Actions it is that sentence with the uses count in front of it -- "Legendary
// Action Uses: 3 (4 in Lair)." -- because the count is a column and the sentence
// is not.
type StatBlockSection struct {
	Heading string
	Intro   string
	Entries []StatBlockEntry
}

// MonsterAction is one editable row: a name, a description, and the two things
// its form needs to address itself. ID is the ULID as a string because it lands
// in attributes and a URL and never in arithmetic, which is the shape Attack's
// is.
//
// Kind is on the row as well as on the section holding it. The row's form posts
// to /monsters/{id}/actions/{kind}/{actionId}, so the row that renders the form
// is the thing that has to know it -- and a row cannot change kind, so the two
// copies cannot come apart.
type MonsterAction struct {
	ID          string
	Kind        string
	Name        string
	Description string
}

// monsterName covers the monster whose name is blank. The Identity panel
// requires one and creation takes one, so this is reachable only by an empty
// render -- but an empty heading reads as a broken page rather than as an
// unnamed monster. The bar and the stat block both print it, which is why it
// takes the name rather than either of their structs.
func monsterName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "Unnamed monster"
	}

	return name
}

// monsterChips is the order the bar reads in, and it is Go rather than markup
// for the reason characterChips is: the list is data, and one of the six is
// split back apart into a number and its unit.
//
// IT IS SIX READINGS AND NOT THE CHARACTER'S SIX. There is no current-hit-point
// half to the hit points, because the manual holds the stat block as printed and
// the wounded copy of a monster lives in the room it was spawned into; and the
// passive perception the sheet ends on is replaced by the challenge rating,
// which is the reading a GM checks before putting a monster in front of anyone.
func monsterChips(header MonsterHeader) []headerChip {
	speed, speedUnit := splitMeasurement(header.Speed)

	return []headerChip{
		{Label: "AC", Value: header.AC},
		{Label: "Hit Points", Value: header.HP},
		{Label: "Speed", Value: speed, Sub: speedUnit},
		{Label: "Initiative", Value: header.Initiative},
		{Label: "Proficiency", Value: header.Proficiency},
		{Label: "CR", Value: header.CR},
	}
}

// monsterActionsID is the container one section's rows live in, and the target
// its add button appends to. It is built from the kind so the seven ids cannot
// collide and none of them has to be written down twice.
func monsterActionsID(kind string) string {
	return "actions-" + kind
}

// MonsterActionRowPanel is the error-block id a row owns, built the way
// AttackRowPanel is. The argument carries a ULID out of the URL, which is safe
// for the narrow reason that the handler parses it as a ULID before anything
// renders -- so it is 26 characters of Crockford base32 or the request never got
// here.
func MonsterActionRowPanel(actionID string) string {
	return "monster-action-" + actionID
}
