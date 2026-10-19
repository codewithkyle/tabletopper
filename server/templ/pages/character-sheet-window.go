package pages

import (
	"strconv"

	"tabletopper/internal/uievents"
)

const SheetBodyID = "character-sheet-body"
const SheetWindow = "character-sheet"
const SpellsPrepared = -1

const (
	SheetSectionMain      = "main"
	SheetSectionAbilities = "abilities"
	SheetSectionSkills    = "skills"
	SheetSectionAttacks   = "attacks"
	SheetSectionInventory = "inventory"
	SheetSectionSpells    = "spells"
	SheetSectionIdentity  = "identity"
	SheetSectionCoreStats = "core-stats"
	SheetSectionVitals    = "vitals"
)

type SheetTab struct {
	Section string
	Label   string
}

func SheetTabs() []SheetTab {
	return []SheetTab{
		{Section: SheetSectionMain, Label: "Character"},
		{Section: SheetSectionAbilities, Label: "Abilities"},
		{Section: SheetSectionSkills, Label: "Skills"},
		{Section: SheetSectionAttacks, Label: "Attacks"},
		{Section: SheetSectionInventory, Label: "Inventory"},
		{Section: SheetSectionSpells, Label: "Spells"},
	}
}

func CharacterEditPath(characterID string, section string) string {
	base := "/characters/" + characterID + "/edit"
	if section == SheetSectionMain {
		return base
	}
	return base + "/" + section
}

func SpellLevelEditPath(characterID string, level int) string {
	if level == SpellsPrepared {
		return CharacterEditPath(characterID, SheetSectionSpells)
	}
	return CharacterEditPath(characterID, SheetSectionSpells) + "/" + strconv.Itoa(level)
}

func sheetSectionTitle(section string) string {
	switch section {
	case SheetSectionMain:
		return "Character Editor"
	case SheetSectionSpells:
		return "Prepared Spells"
	}
	for _, tab := range SheetTabs() {
		if tab.Section == section {
			return tab.Label
		}
	}
	return "Character Editor"
}

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
func (d SheetWindowData) Path() string {
	if d.Section == SheetSectionSpells {
		return d.LevelPath(d.Level)
	}
	return d.SectionPath(d.Section)
}

func SheetLevelPath(roomID string, level int) string {
	if level == SpellsPrepared {
		return SheetSectionPath(roomID, SheetSectionSpells)
	}
	return SheetSectionPath(roomID, SheetSectionSpells) + "&level=" + strconv.Itoa(level)
}

func SheetWindowPath(roomID string) string {
	return "/fragment/character/sheet?room=" + roomID
}

func LiveSheetSections() []string {
	return []string{SheetSectionIdentity, SheetSectionCoreStats, SheetSectionVitals}
}
func SheetSectionPath(roomID string, section string) string {
	return SheetWindowPath(roomID) + "&section=" + section
}
func CharacterPanelPath(characterID string, section string) string {
	return "/fragment/character/panel?character=" + characterID + "&section=" + section
}
func characterPanelTrigger(characterID string) string {
	return uievents.Character + "[detail.id === '" + characterID + "' && !" + typingInPanel + "] from:window"
}

const SheetFollowScript = "/js/sheet-follow.js"
const SheetOrderScript = "/static/sheet-order.js"
