package pages

type SharedMonsterData struct {
	Block       StatBlock
	Description string
	Actions     SharedActions
}

func SharedMonsterTitle(name string) string {
	return monsterName(name) + " | Tabletopper"
}
