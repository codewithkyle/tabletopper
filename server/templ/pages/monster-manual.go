package pages

const (
	ManualWindow = "manual"
	ManualBodyID = "monster-manual"
)

const manualListID = "monster-manual-list"

func ManualWindowPath() string {
	return "/fragment/monster/manual"
}
func ManualMonsterPath(monsterID string) string {
	return ManualWindowPath() + "?monster=" + monsterID
}
