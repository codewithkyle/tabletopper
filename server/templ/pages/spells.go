package pages
import (
	"strconv"
	"strings"
)
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
const MaxSpellLevel = 9
type SpellLevel struct {
	Level int
	Slots string
	Used  string
	Count int
}
type SpellLevelPageData struct {
	CharacterID string
	Header  CharacterHeader
	Level   int
	Current SpellLevel
	Spells  []Spell
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
const DefaultSpellSchool = "Evocation"
func NormalizeSpellSchool(value string) string {
	for _, school := range spellSchools {
		if school == value {
			return school
		}
	}
	return DefaultSpellSchool
}
func SpellLevelName(level int) string {
	if level == 0 {
		return "Cantrips"
	}
	return "Level " + strconv.Itoa(level)
}
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
func SpellRowPanel(spellID string) string {
	return "spell-" + spellID
}
func SpellSlotsPanel(level int) string {
	return "spell-slots-" + strconv.Itoa(level)
}
type PreparedSpellGroup struct {
	Level  int
	Name   string
	Spells []Spell
}
func SpellMetaLine(spell Spell) string {
	parts := make([]string, 0, 3)
	for _, part := range []string{spell.CastingTime, spell.CastingRange, spell.Duration} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, " \u00b7 ")
}
func preparedSpellName(spell Spell) string {
	if spell.Name == "" {
		return "Unnamed spell"
	}
	return spell.Name
}
func SpellCountLabel(count int) string {
	switch count {
	case 0:
		return "No spells"
	case 1:
		return "1 spell"
	default:
		return strconv.Itoa(count) + " spells"
	}
}
func activeSpellLevels(levels []SpellLevel) []SpellLevel {
	active := make([]SpellLevel, 0, len(levels))
	for _, level := range levels {
		if level.Count == 0 && level.Slots == "0" && level.Used == "0" {
			continue
		}
		active = append(active, level)
	}
	return active
}
func spellRowName(spell Spell) string {
	if spell.Name == "" {
		return "spell"
	}
	return spell.Name
}
