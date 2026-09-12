package pages

import (
	"strconv"

	"tabletopper/internal/uievents"
)

const SheetBodyID = "character-sheet-body"
const SheetWindow = "character-sheet"
const SheetSectionMain = "main"

type SheetWindowData struct {
	RoomID    string
	Section   string
	Level     int
	Sheet     EditCharacterPageData
	Inventory InventoryPageData
	Spells    SpellLevelPageData
}

func (d SheetWindowData) SectionPath(section string) string {
	return SheetSectionPath(d.RoomID, section)
}
func (d SheetWindowData) LevelPath(level int) string {
	return SheetLevelPath(d.RoomID, level)
}
func (d SheetWindowData) Tabs() []SheetTab {
	return []SheetTab{
		{Section: SheetSectionMain, Label: "Character"},
		{Section: SheetSectionInventory, Label: "Inventory"},
		{Section: SheetSectionSpells, Label: "Spells"},
	}
}

type SheetTab struct {
	Section string
	Label   string
}

func SheetLevelPath(roomID string, level int) string {
	return SheetSectionPath(roomID, SheetSectionSpells) + "&level=" + strconv.Itoa(level)
}

func SheetWindowPath(roomID string) string {
	return "/fragment/character/sheet?room=" + roomID
}

const (
	SheetSectionInventory = "inventory"
	SheetSectionSpells    = "spells"
	SheetSectionIdentity  = "identity"
	SheetSectionCoreStats = "core-stats"
	SheetSectionVitals    = "vitals"
)

func LiveSheetSections() []string {
	return []string{SheetSectionIdentity, SheetSectionCoreStats, SheetSectionVitals}
}
func SheetSectionPath(roomID string, section string) string {
	return SheetWindowPath(roomID) + "&section=" + section
}

type SheetLive struct {
	RoomID string
}

func (l SheetLive) On() bool { return l.RoomID != "" }
func (l SheetLive) Path(section string) string {
	return SheetSectionPath(l.RoomID, section)
}
func (l SheetLive) Trigger() string {
	return uievents.Character + "[!" + typingInPanel + "] from:window"
}
