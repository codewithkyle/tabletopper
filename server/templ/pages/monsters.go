package pages
























const monsterCardsID = "monster-cards"








type MonsterListData struct {
	Monsters []MonsterSummary
	Query    string
}





func (d MonsterListData) noMatch() string {
	return noMatchHeading("monsters", d.Query)
}










type MonsterSummary struct {
	ID       string
	Name     string
	Subtitle string
	
	
	
	ImageID string

	
	
	
	CR string
	AC string
	HP string
}
