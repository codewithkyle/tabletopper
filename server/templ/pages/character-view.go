package pages

import (
	"strconv"
	"strings"

	"tabletopper/internal/uievents"
)

const CharacterViewWindowPrefix = "character:"
const (
	SheetWindowWidth  = 520
	SheetWindowHeight = 640
)

type CharacterViewData struct {
	RoomID      string
	CharacterID string
	PawnID      string
	Section     string
	Level       int
	Sheet       SharedCharacterSheet
	Inventory   []InventoryItem
	Levels      []SpellLevel
	Spells      SpellLevelPageData
}

func CharacterViewWindow(characterID string) string {
	return CharacterViewWindowPrefix + characterID
}
func CharacterViewPath(roomID string, characterID string) string {
	return "/fragment/room/character-sheet?room=" + roomID + "&character=" + characterID
}
func CharacterViewSectionPath(roomID string, characterID string, section string) string {
	return CharacterViewPath(roomID, characterID) + "&section=" + section
}
func characterViewLabel(name string) string {
	return "Character sheet for " + characterBarName(CharacterHeader{Name: name})
}

func (d CharacterViewData) BodyID() string {
	return "character-view-" + d.CharacterID
}
func (d CharacterViewData) SectionPath(section string) string {
	return CharacterViewSectionPath(d.RoomID, d.CharacterID, section)
}
func (d CharacterViewData) LevelPath(level int) string {
	if level == SpellsPrepared {
		return d.SectionPath(SheetSectionSpells)
	}
	return d.SectionPath(SheetSectionSpells) + "&level=" + strconv.Itoa(level)
}
func (d CharacterViewData) Path() string {
	if d.Section == SheetSectionSpells {
		return d.LevelPath(d.Level)
	}
	return d.SectionPath(d.Section)
}
func (d CharacterViewData) DetailsGroup(kind string) string {
	return "view-" + kind + "-" + d.CharacterID
}
func (d CharacterViewData) Live() bool {
	return d.PawnID != ""
}
func (d CharacterViewData) Trigger() string {
	return uievents.Pawn + "[detail.id === '" + d.PawnID + "'] from:window"
}

var sheetWindowWidth = strconv.Itoa(SheetWindowWidth)
var sheetWindowHeight = strconv.Itoa(SheetWindowHeight)

func SpellSlotsLeft(level SpellLevel) string {
	if level.Level == 0 {
		return "Unlimited"
	}
	slots, _ := strconv.Atoi(level.Slots)
	used, _ := strconv.Atoi(level.Used)
	if slots <= 0 {
		return "No slots"
	}
	unit := " slots left"
	if slots == 1 {
		unit = " slot left"
	}
	return strconv.Itoa(max(slots-used, 0)) + " of " + strconv.Itoa(slots) + unit
}
func spellFacts(spell Spell) []SharedFact {
	facts := []SharedFact{}
	for _, fact := range []SharedFact{
		{Label: "School", Value: spell.School},
		{Label: "Range", Value: spell.CastingRange},
		{Label: "Duration", Value: spell.Duration},
		{Label: "Components", Value: spell.Components},
	} {
		if strings.TrimSpace(fact.Value) != "" {
			facts = append(facts, fact)
		}
	}
	return facts
}
func inventoryItemName(item InventoryItem) string {
	if strings.TrimSpace(item.Name) == "" {
		return "Unnamed item"
	}
	return item.Name
}
