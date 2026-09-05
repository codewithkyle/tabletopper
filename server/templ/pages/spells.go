package pages

import (
	"strconv"
	"strings"
)

// The spells tab, which is ten pages rather than one.
//
// Spellcasting used to be a single panel holding all ten levels stacked, each
// level a bordered section, each spell inside it a card with seven fields open
// at once. It was correct and unreadable. The levels are pages now, linked by
// the sub-nav below the character tabs, and a spell shows what you scan for with
// the rest behind a disclosure.
//
// There is no index page above them. The Spells tab opens on cantrips, which is
// where a level-1 caster's whole spell list lives, and the level strip is how
// you get anywhere else -- an index would have been one more click in front of
// the page everybody wants.
//
// Fields are strings for the same reason EditCharacterPageData's are: the
// controller does every conversion once, and the template does none.

// Spell is one row, already formatted for the markup. ID is the ULID as a string
// because it lands in two attributes and a URL and never in arithmetic. Level
// is an int because it lands in a URL and in the ten-level arithmetic the tabs
// do, and it is not editable -- a spell cannot change level, so no control
// renders it and no update writes it.
type Spell struct {
	ID           string
	Level        int
	Name         string
	School       string
	Components   string
	CastingTime  string
	CastingRange string
	Duration     string
	Description  string
	Prepared     bool
}

// MaxSpellLevel is nine, and this is the only place that says so. The tab strip
// counts to it, the controller bounds the {level} segment by it, and the column
// carries a CHECK that agrees.
const MaxSpellLevel = 9

// SpellLevel is one level's slot counters. Only the level being shown has them
// -- the tab strip is ten static labels and needs no data at all -- so a page
// carries exactly one of these, and cantrips carry none.
type SpellLevel struct {
	Level int
	Slots string
	Used  string
}

// SpellLevelPageData backs /edit/spells/{level}.
//
// Current is this level's slot counters, and it is the zero value on the
// cantrips page: cantrips have no slots, so nothing renders them and nothing
// reads them.
type SpellLevelPageData struct {
	CharacterID string
	Level       int
	Current     SpellLevel
	Spells      []Spell
}

var spellSchools = []string{
	"Abjuration",
	"Conjuration",
	"Divination",
	"Enchantment",
	"Evocation",
	"Illusion",
	"Necromancy",
	"Transmutation",
}

// DefaultSpellSchool is what an unrecognised school falls back to, which is a
// different job from the column's DEFAULT 'Evocation'. The column answers what a
// brand-new row starts as; this answers what a posted value that is not one of
// the eight becomes. They agree, and neither is derived from the other.
const DefaultSpellSchool = "Evocation"

func NormalizeSpellSchool(value string) string {
	for _, school := range spellSchools {
		if school == value {
			return school
		}
	}

	return DefaultSpellSchool
}

// SpellLevelName is the level in prose, for headings and page titles.
func SpellLevelName(level int) string {
	if level == 0 {
		return "Cantrips"
	}

	return "Level " + strconv.Itoa(level)
}

// SpellLevelTab is the level on a tab, where eleven of them share one row and
// "Level 7" is six characters nobody needs to read twice.
func SpellLevelTab(level int) string {
	if level == 0 {
		return "Cantrips"
	}

	return strconv.Itoa(level) + spellLevelOrdinal(level)
}

func spellLevelOrdinal(level int) string {
	switch level {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	default:
		return "th"
	}
}

// SpellRowPanel and SpellSlotsPanel are the error-block ids a spell row and a
// level's counters own. PanelFormErrors documents that its argument is never
// user input; these carry a ULID and a level out of the URL, which is safe for
// the narrow reason that the handler parses both -- as a ULID and as a number
// bounded to 0-9 -- before anything renders.
func SpellRowPanel(spellID string) string {
	return "spell-" + spellID
}

func SpellSlotsPanel(level int) string {
	return "spell-slots-" + strconv.Itoa(level)
}

// PreparedSpellGroup is one level's prepared spells, for the read-only view on
// the Character tab. The grouping is done in the controller off a query ordered
// by level, so the template renders what it is given and does no sorting.
//
// Name is the level in prose rather than the number, because this list is read
// beside Equipment and away from the tab strip that translates 3 into Level 3.
type PreparedSpellGroup struct {
	Level  int
	Name   string
	Spells []Spell
}

// spellMetaLine is what a prepared spell says under its name: the three things
// you need before deciding to cast it, and nothing you would have to scroll.
// The spell text stays on the spells page -- ten paragraphs on the Character tab
// would be the wall this whole rework was meant to remove.
func spellMetaLine(spell Spell) string {
	parts := make([]string, 0, 3)
	for _, part := range []string{spell.CastingTime, spell.CastingRange, spell.Duration} {
		if part != "" {
			parts = append(parts, part)
		}
	}

	return strings.Join(parts, " \u00b7 ")
}

// preparedSpellName covers the row that was ticked before it was named. An empty
// entry on the sheet reads as a rendering bug rather than as an unfinished row,
// and the level page is where it gets fixed.
func preparedSpellName(spell Spell) string {
	if spell.Name == "" {
		return "Unnamed spell"
	}

	return spell.Name
}

// spellRowName covers the row that was added before it was named, which is every
// row for its first few seconds. The delete button's label is the only place it
// shows, and "Delete spell" beats "Delete ".
func spellRowName(spell Spell) string {
	if spell.Name == "" {
		return "spell"
	}

	return spell.Name
}
