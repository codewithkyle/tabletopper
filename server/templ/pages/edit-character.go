package pages

// CharacterID is the ULID as a string, and it is the only thing the editor
// needs to build a URL: every panel posts to /characters/<id>/<panel>.
type EditCharacterPageData struct {
	CharacterID     string
	Name            string
	Race            string
	Background      string
	Classes         string
	Size            string
	Alignment       string
	XP              string
	Languages       string
	Proficiencies   string
	Str             string
	Dex             string
	Con             string
	Int             string
	Wis             string
	Cha             string
	AC              string
	Speed           string
	InitiativeBonus string
	MaxHP           string
	CurrentHP       string
	TempHP          string
	SpellSaveDC     string
	SpellAtkBonus   string
	Skills          map[string]int
	SavingThrows    map[string]int
	Features        []InfoRow
	Weapons         []InfoRow
	Resources       []InfoRow
	SpellLevels     []SpellLevel
}
