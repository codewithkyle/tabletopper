package pages

import "strings"

type EditMonsterPageData struct {
	MonsterID                 string
	Header                    MonsterHeader
	StatBlock                 StatBlock
	Name                      string
	Size                      string
	Type                      string
	Tags                      string
	Alignment                 string
	Str                       string
	Dex                       string
	Con                       string
	Int                       string
	Wis                       string
	Cha                       string
	AC                        string
	HP                        string
	HitDice                   string
	Speed                     string
	InitiativeBonus           string
	CR                        string
	LegendaryActionUses       string
	LegendaryActionUsesInLair string
	Vulnerabilities           string
	Resistances               string
	Immunities                string
	Gear                      string
	Senses                    string
	Languages                 string
	Habitat                   string
	Treasure                  string
	Description               string
	Derived                   MonsterDerived
	Actions                   map[string][]MonsterAction
}
type MonsterHeader struct {
	MonsterID   string
	Name        string
	Subtitle    string
	ImageID     string
	AC          string
	HP          string
	Speed       string
	Initiative  string
	Proficiency string
	CR          string
}
type MonsterDerived struct {
	StrMod            string
	DexMod            string
	ConMod            string
	IntMod            string
	WisMod            string
	ChaMod            string
	Skills            []BonusRow
	SavingThrows      []BonusRow
	PassivePerception string
	Initiative        string
	PassiveInitiative string
	Proficiency       string
	XP                string
	InLairXP          string
}
type StatBlock struct {
	Name       string
	Subtitle   string
	Image      string
	AC         string
	Initiative string
	HP         string
	HitDice    string
	Speed      string
	Abilities  []StatBlockAbility
	Lines      []StatBlockEntry
	Sections   []StatBlockSection
	Habitat    string
	Treasure   string
}
type StatBlockAbility struct {
	Label string
	Score string
	Mod   string
	Save  string
}
type StatBlockEntry struct {
	Label string
	Value string
}
type StatBlockSection struct {
	Heading string
	Intro   string
	Entries []StatBlockEntry
}
type MonsterAction struct {
	ID          string
	Kind        string
	Name        string
	Description string
}

func monsterName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "Unnamed monster"
	}
	return name
}
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
func monsterActionsID(kind string) string {
	return "actions-" + kind
}
func MonsterActionRowPanel(actionID string) string {
	return "monster-action-" + actionID
}
