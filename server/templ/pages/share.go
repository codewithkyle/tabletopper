package pages

import "strings"



















type SharedCharacter struct {
	Name    string
	Level   string
	Classes string
	Race    string

	
	
	
	
	
	
	
	
	
	Avatar string
}








type SharedJournalData struct {
	Character SharedCharacter
	Title     string
	Body      string
}










type ShareLockedData struct {
	
	
	
	Action string

	
	
	
	Problem string
}





func shareEntryTitle(title string) string {
	if strings.TrimSpace(title) == "" {
		return "Untitled entry"
	}

	return title
}



func ShareTitle(title string) string {
	return shareEntryTitle(title) + " | Tabletopper"
}
