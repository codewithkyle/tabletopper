package pages

type MonsterImage struct {
	MonsterID string
	Name      string
	ImageID   string
}

func (m MonsterImage) Verb() string {
	if m.ImageID == "" {
		return "Upload"
	}
	return "Replace"
}
func monsterImageInputID(monsterID string) string {
	return "monster-image-" + monsterID
}

const newMonsterImageInputID = "new-monster-image"
