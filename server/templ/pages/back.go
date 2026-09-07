package pages

// THE BACK LINK, which is one control on five pages and used to be five copies
// of an anchor with the word "Back" written into it.
//
// IT SITS AT THE TOP LEFT, ahead of the page's own title or the character it is
// showing. That is the corner a hand goes to: the browser's own back button is
// there, so is every mobile navigation bar, and so is the up arrow in every file
// manager. On the right it was findable rather than reachable -- the cost was
// never that it took a second look, it was that it took one at all, on every
// page, forever.
//
// IT NAMES WHERE IT GOES rather than saying "Back", and that is the half of this
// that is not about position. "Back" is a direction, and a direction can only be
// read by someone who remembers how they arrived. A journal entry is opened from
// the journal tab, from a link somebody pasted, and from the browser's own
// history, and the one word meant a different place each time. "Journal" is true
// however the page was reached.
//
// THE ARROW IS NOT A SUBSTITUTE FOR THE WORD. An arrow alone says "back" and
// says nothing about back to where, which is the ambiguity above with the label
// removed rather than fixed. It is there to mark the control as navigation at a
// glance, in the corner where a glance is all it gets. So the label stays at
// every width, and what yields room beside it on a narrow screen is the
// character's name, which truncates on a page that is showing that character
// everywhere else anyway.
type backTarget struct {
	Href  string
	Label string
}

// homeBack is the main menu, which is the page above the roster, the manual and
// the asset manager alike.
//
// The menu's own heading is the app's name, so the link says "Home" rather than
// "Tabletopper": the label has to name a destination a reader already has a word
// for, and every reader has that word for the page a site starts on.
func homeBack() backTarget {
	return backTarget{Href: "/", Label: "Home"}
}

// charactersBack is the roster, which is the page above every editor tab except
// the one journalBack covers.
func charactersBack() backTarget {
	return backTarget{Href: "/characters", Label: "Characters"}
}

// monsterManualBack is the manual, which is the page above the monster editor.
// The label is "Monsters" and not "Monster Manual", because a back link is read
// in a corner at a glance and the shorter of two true names wins there.
func monsterManualBack() backTarget {
	return backTarget{Href: "/monsters", Label: "Monsters"}
}

// roomsBack is the GM's own rooms, which is the page above a room they own. A
// player reached the same room from the join page rather than from a list of
// rooms, and /rooms would show them their own empty one -- so the room page
// picks between this and homeBack by role rather than linking one of them
// unconditionally. See RoomPageData.back.
func roomsBack() backTarget {
	return backTarget{Href: "/rooms", Label: "Rooms"}
}

// journalBack is the character's own journal tab, and it is the reason
// shellLayout carries a Back field at all.
//
// AN ENTRY IS TWO STEPS DOWN THE SHEET, NOT ONE. Every other editor page is a
// tab, so the roster is the page above it and the tab strip already handles
// moving sideways. An entry is a document inside one of those tabs: the page
// above it is the list of entries, and the tab strip cannot offer that, because
// the Journal tab reads as the current tab the whole time an entry is open. So
// the back link went to the roster, two levels up, and the one page a writer
// wanted next had nothing pointing at it.
func journalBack(characterID string) backTarget {
	return backTarget{Href: "/characters/" + characterID + "/edit/journal", Label: "Journal"}
}

// back is where the bar's link goes on the tab being rendered. The zero Back is
// the roster, which is right for four of the five pages that use the shell.
func (l shellLayout) back() backTarget {
	if l.Back.Href == "" {
		return charactersBack()
	}

	return l.Back
}
