package pages

// The shared monster, as a stranger sees it: the stat block the editor draws
// down its left column, the GM's own paragraph under it, and -- for a reader who
// is signed in -- the offer of a copy.
//
// THE PAGE IS THE WHOLE MONSTER, which is where it parts company with the shared
// character sheet. That one hands over one of five tabs on purpose, because a
// journal sits behind the others with its own links and its own passwords. A
// monster has nothing behind it: what the owner is sharing is the stat block,
// and the import hands over a copy of every column of it anyway -- so a page
// that showed less than the button gives would be describing the wrong thing.
//
// IT CARRIES NO IDS. Not the monster's, not the owner's, not the asset's. The
// picture is the share's own portrait URL and the button posts to the share's
// own import URL, both built from the token by the controller, so there is
// nothing in this markup that names anything inside the app.

// SharedMonsterData is the whole page.
type SharedMonsterData struct {
	// Block is the same StatBlock the editor and the View dialog render, with
	// its Image pointed at the share's portrait route -- see StatBlock.Image
	// for why that field is a URL rather than an id.
	Block StatBlock

	// Description is the GM's own paragraph: lore, tactics, what the thing
	// wants. It is not part of the stat block and never has been -- the printed
	// block does not carry it -- so it is a field here and a panel of its own on
	// the page, rendered as typed and empty for most monsters.
	Description string

	Offer SharedMonsterOffer
}

// SharedMonsterOffer is the one thing on any shared page that is not the same
// for every reader.
//
// AT MOST ONE FIELD IS SET, AND BOTH BEING EMPTY IS A STATE. Signed in, Import
// holds the URL the form posts to; signed out, SignIn holds where to go and get
// an account, because a button that answered with a sign-in page would be a
// worse way of saying the same thing; and the owner reading their own link gets
// neither, since importing their own monster would hand them a duplicate they
// did not ask for. Two strings rather than a kind and a URL, because the URL is
// the thing the markup needs and a kind beside it could disagree with it.
type SharedMonsterOffer struct {
	Import string
	SignIn string
}

// Shown reports whether there is anything to draw, which is the check the panel
// is wrapped in -- so the owner's page ends at the monster.
func (o SharedMonsterOffer) Shown() bool {
	return o.Import != "" || o.SignIn != ""
}

// SharedMonsterTitle is the <title> for the page: the monster's name and nothing
// about the app around it. monsterName covers one shared before it was named,
// which the editor's bar has to cover too.
func SharedMonsterTitle(name string) string {
	return monsterName(name) + " | Tabletopper"
}
