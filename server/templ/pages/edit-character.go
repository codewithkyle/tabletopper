package pages

type EditCharacterPageData struct {
	FormAction       string
	Name             string
	Race             string
	Background       string
	Classes          string
	Size             string
	Alignment        string
	Xp               string
	Languages        string
	Proficiencies    string
	Str              string
	Dex              string
	Con              string
	Int              string
	Wis              string
	Cha              string
	Ac               string
	Speed            string
	InitiativeBonus  string
	MaxHP            string
	CurrentHP        string
	TempHP           string
	SpellSaveDC      string
	SpellAtkBonus    string
	SkillsJSON       string
	SavingThrows     map[string]int
	FeaturesJSON     string
	WeaponsJSON      string
	ResourcesJSON    string
	SpellSlotsJSON   string
}
