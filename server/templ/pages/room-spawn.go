package pages

import "strconv"

// THE SPAWN DIALOG IS A MODAL AND NOT A WINDOW, which is the same question the
// map picker answered the same way: is this something somebody keeps open while
// they work, or a task with an end? Choosing what to place is a task. It ends
// by ARMING the canvas -- the dialog shuts and the pointer carries a ghost --
// so the thing that stays open afterwards is the table, which is where the
// placing actually happens.
//
// PLACEMENT SURVIVES THE SPAWN, which is why arming is worth the round trip. An
// encounter is eight goblins, and eight clicks with one dialog visit is the
// whole reason this is not a form with an X and a Y in it.
//
// IT IS TWO ROUTES, THE DIALOG AND THE RESULTS, and that is the shape the
// asset manager and the map picker already have. A search that replaced the
// whole dialog would replace the box being typed into, and the caret would jump
// to the end of the field on every keystroke; a search that replaces the grid
// alone does not. The kind switch replaces the whole dialog, because the
// controls above the results are different for the two kinds.

const (
	// RoomSpawnMonsters and RoomSpawnTokens are the two halves of the dialog
	// and the only two values the route accepts. They are matched against these
	// constants before anything reaches a statement, which is the rule every
	// kind-parameterised route in this app follows.
	RoomSpawnMonsters = "monsters"
	RoomSpawnTokens   = "tokens"
)

// roomSpawnResultsID is the grid the search box swaps and points its
// aria-controls at. It is written once because three attributes read it, and a
// box aimed at an element that is not there fails silently.
const roomSpawnResultsID = "spawn-results"

// RoomSpawnData is the whole dialog.
type RoomSpawnData struct {
	RoomID string

	// Kind is RoomSpawnMonsters or RoomSpawnTokens, already validated.
	Kind string

	// Query is the search term, kept so the empty state can repeat it back and
	// so the kind switch does not silently drop it.
	Query string

	Monsters []MonsterSummary
	Tokens   []RoomSpawnToken
}

// RoomSpawnToken is one token in the library as its pick card needs it: a
// picture and a name, which is all a token is.
type RoomSpawnToken struct {
	ID   string
	Name string

	// Image is the whole URL rather than an id, because a token card is the one
	// place in this dialog where the picture IS the thing being chosen and an
	// empty one is a card with nothing on it.
	Image string
}

// IsMonsters is the one question the markup asks of the kind.
func (d RoomSpawnData) IsMonsters() bool { return d.Kind == RoomSpawnMonsters }

// Path is the dialog for one kind, which is what the two kind buttons fetch.
//
// IT CARRIES NO TERM AND THE BUTTON CARRIES hx-include INSTEAD. A button is not
// a form, so htmx sends nothing of the search box with it unless told to; an
// escaped term baked into the href here would be the term as it was when the
// dialog was RENDERED, which is not what is in the box by the time somebody
// switches kinds. Including the live field means switching from monsters to
// tokens keeps what was typed.
func (d RoomSpawnData) Path(kind string) string {
	return "/fragment/room/spawn?room=" + d.RoomID + "&kind=" + kind
}

// ListPath is the results alone, which is what the search box fetches. It
// carries no term: htmx appends the box's own value as q.
func (d RoomSpawnData) ListPath() string {
	return "/fragment/room/spawn-list?room=" + d.RoomID + "&kind=" + d.Kind
}

// PartyPath is the Spawn party button, which is a mutation and therefore a
// resource URL rather than a fragment.
func (d RoomSpawnData) PartyPath() string {
	return "/rooms/" + d.RoomID + "/pawns/party"
}

// ResultsID is the element the search replaces.
func (d RoomSpawnData) ResultsID() string { return roomSpawnResultsID }

// SearchLabel is the placeholder and the accessible name, which differ by kind
// because "Search" over a grid of pictures says nothing about what is in it.
func (d RoomSpawnData) SearchLabel() string {
	if d.IsMonsters() {
		return "Search the manual"
	}

	return "Search your tokens"
}

// NoMatch is the heading a search that found nothing gets, through the same
// builder the asset manager's four lists use.
func (d RoomSpawnData) NoMatch() string {
	if d.IsMonsters() {
		return noMatchHeading("monsters", d.Query)
	}

	return noMatchHeading("tokens", d.Query)
}

// SearchLimit is the longest term the box accepts, which is also what the route
// refuses past. It is the asset name limit because that is the longest thing
// anybody could be searching for.
func (d RoomSpawnData) SearchLimit() string { return strconv.Itoa(AssetNameLimit) }

// FootprintMaxText is the object width and height inputs' max attribute.
func (d RoomSpawnData) FootprintMaxText() string { return strconv.Itoa(FootprintCellsMax) }

// EmptyHeading and EmptyBlurb are the two states of "there is nothing here",
// which are different sentences and not one with a word swapped: an empty
// manual wants pointing at the Monster Manual, and an empty token library wants
// pointing at the asset manager. Both are one navigation away and neither is
// reachable from inside this dialog, so saying where is the whole of the help
// available.
func (d RoomSpawnData) EmptyHeading() string {
	if d.IsMonsters() {
		return "Nothing in your manual yet."
	}

	return "No tokens in your library yet."
}

func (d RoomSpawnData) EmptyBlurb() string {
	if d.IsMonsters() {
		return "Write up what you plan to run in the Monster Manual, then come back and place it."
	}

	return "Upload tokens on the Assets page. A token is any picture you want to put on a table."
}

// monsterImageURL is a monster's picture, or nothing when it has none. The
// route is the unscoped one, which serves a monster's image to any signed-in
// user for the reason the tile route serves a map to them: what is on the table
// is shown to the table.
func monsterImageURL(imageID string) string {
	if imageID == "" {
		return ""
	}

	return "/assets/images/" + imageID
}

// boolText is an attribute that takes the word rather than presence, which
// aria-pressed does -- absent and "false" are different states to a screen
// reader, and a templ conditional attribute can only express presence.
func boolText(value bool) string {
	if value {
		return "true"
	}

	return "false"
}

// Sizes is the six creature sizes, for the token view's size select. A token is
// a picture and brings no size with it, so this is where one is chosen -- and
// it is the only stat the dialog collects, because size decides the footprint
// and therefore where a click actually puts the pawn. Hit points and armour
// class are the pawn dialog's, filled in once the GM knows what the thing is.
func (d RoomSpawnData) Sizes() []Option { return sizeOptions }
