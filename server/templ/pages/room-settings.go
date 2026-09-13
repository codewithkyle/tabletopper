package pages

const RoomSettingsPanel = "room-settings"

type RoomSettingsData struct {
	RoomID             string
	PawnLabels         string
	PlayersCanDraw     bool
	InitiativeGrouping string
	FogPrefill         bool
	Errors             []string
}

func (d RoomSettingsData) SavePath() string {
	return "/rooms/" + d.RoomID + "/settings"
}
func (d RoomSettingsData) Path() string {
	return "/fragment/room/settings?room=" + d.RoomID
}
func InitiativeGroupingChoices() []Choice {
	return []Choice{
		{Value: "grouped", Label: "Grouped", Hint: "Nine goblins take one turn together, on one line, with a dot each for how hurt they are."},
		{Value: "individual", Label: "One at a time", Hint: "Every monster gets a line of its own. Switch to this for a fight where each of them matters."},
	}
}
func PawnLabelChoices() []Choice {
	return []Choice{
		{Value: "none", Label: "None", Hint: "No panel over any pawn, for anybody. Every pawn's window is still yours."},
		{Value: "default", Label: "Default", Hint: "You read the numbers. Players read a name and a word: healthy, bruised, bloody, and so on down to near death."},
		{Value: "full", Label: "Full", Hint: "Everybody reads the name, the armour class and the hit points, the same as you do."},
	}
}
