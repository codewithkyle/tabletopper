package pages

























type backTarget struct {
	Href  string
	Label string
}







func homeBack() backTarget {
	return backTarget{Href: "/", Label: "Home"}
}



func charactersBack() backTarget {
	return backTarget{Href: "/characters", Label: "Characters"}
}




func monsterManualBack() backTarget {
	return backTarget{Href: "/monsters", Label: "Monsters"}
}






func roomsBack() backTarget {
	return backTarget{Href: "/rooms", Label: "Rooms"}
}











func journalBack(characterID string) backTarget {
	return backTarget{Href: "/characters/" + characterID + "/edit/journal", Label: "Journal"}
}



func (l shellLayout) back() backTarget {
	if l.Back.Href == "" {
		return charactersBack()
	}

	return l.Back
}
