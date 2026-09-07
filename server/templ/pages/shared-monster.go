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

	// Actions is the row above the block: the Markdown export every reader
	// gets, and the copy button the ones who can use it get. It is the shared
	// character sheet's type as well -- see SharedActions.
	Actions SharedActions
}

// SharedMonsterTitle is the <title> for the page: the monster's name and nothing
// about the app around it. monsterName covers one shared before it was named,
// which the editor's bar has to cover too.
func SharedMonsterTitle(name string) string {
	return monsterName(name) + " | Tabletopper"
}
