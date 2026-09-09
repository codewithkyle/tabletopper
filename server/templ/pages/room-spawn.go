package pages

import (
	"net/url"
	"strconv"

	"tabletopper/internal/room"

	"github.com/a-h/templ"
)

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
// A TOKEN IS ALWAYS AN OBJECT AND THE DIALOG ASKS NOTHING ABOUT IT. It used to
// ask two questions and both were wrong. The first was creature or object, and
// a token is an object: it is a picture of a thing on the table -- a wagon, a
// door, a crate -- and a creature is a monster out of the manual or a player's
// character, both of which arrive with a stat line this dialog could never
// collect. The second was how big, in cells, which asked the GM to measure by
// eye something the assets row already records to the pixel. Both are gone, and
// what is left of that half is a search, a Players-see-it switch and a wall of
// pictures.
//
// AN NPC IS THE THIRD HALF AND IT IS THE ONE THAT ASKS. A face out of the
// avatar library is a creature with no stat line anywhere to read one from --
// there is no manual row behind it and no sheet -- so the four numbers a
// creature needs to be fought are typed, once, on the way to the table. That is
// what separates it from the token wall, which places a thing rather than a
// somebody, and from the monster wall, whose stat line is already written down.
//
// SO THE DIALOG COLLECTS VISIBILITY EVERYWHERE AND A STAT LINE IN ONE PLACE.
// Visibility is a real question with no answer anywhere else: whether the GM is
// putting a thing down in front of the party or setting up the next room while
// they talk. The stat line is asked only where nothing else knows it.
//
// IT IS THREE ROUTES: THE DIALOG, THE RESULTS AND THE NPC FORM, and the first
// two are the shape the asset manager and the map picker already have. A search
// that replaced the whole dialog would replace the box being typed into, and
// the caret would jump to the end of the field on every keystroke; a search that
// replaces the grid alone does not. The kind switch still replaces the whole
// dialog rather than the grid, because it changes the pressed button, the
// placeholder and the empty state as well as the results -- and so does picking
// a face, which is a second step rather than a second list.

const (
	// RoomSpawnMonsters, RoomSpawnTokens and RoomSpawnNPCs are the three halves
	// of the dialog and the only three values the route accepts. They are
	// matched against these constants before anything reaches a statement,
	// which is the rule every kind-parameterised route in this app follows.
	RoomSpawnMonsters = "monsters"
	RoomSpawnTokens   = "tokens"
	RoomSpawnNPCs     = "npcs"
)

// roomSpawnResultsID is the grid the search box swaps and points its
// aria-controls at. It is written once because three attributes read it, and a
// box aimed at an element that is not there fails silently.
const roomSpawnResultsID = "spawn-results"

// NPCDefaultHP and NPCDefaultAC are what the NPC form opens with.
//
// THEY ARE THE HUB'S PLACEHOLDER, DELIBERATELY. npcHP and npcAC in
// internal/hub are what a spawn carrying no stat line at all is given, so a GM
// who opens the form and presses Place without touching it gets exactly what
// the server would have written anyway. One hit point and armour class ten is
// the least misleading pair available: it is obviously a placeholder rather
// than a plausible creature, so somebody who meant to fill it in and did not
// finds out on the first hit rather than after a fight balanced against numbers
// nobody chose.
const (
	NPCDefaultHP = 1
	NPCDefaultAC = 10
)

// RoomSpawnData is the whole dialog.
type RoomSpawnData struct {
	RoomID string

	// Kind is RoomSpawnMonsters, RoomSpawnTokens or RoomSpawnNPCs, already
	// validated.
	Kind string

	// Query is the search term, kept so the empty state can repeat it back and
	// so the kind switch does not silently drop it.
	Query string

	Monsters []MonsterSummary
	Tokens   []RoomSpawnToken
	Avatars  []RoomSpawnAvatar
}

// RoomSpawnToken is one token in the library as its pick card needs it: a
// picture, a name, and how big the picture is.
type RoomSpawnToken struct {
	ID   string
	Name string

	// Image is the whole URL rather than an id, because a token card is the one
	// place in this dialog where the picture IS the thing being chosen and an
	// empty one is a card with nothing on it.
	Image string

	// Width and Height are the stored picture's own pixels, and the card
	// carries them so that the ghost following the pointer is the size of the
	// thing about to be placed.
	//
	// THE SERVER DOES NOT TRUST THEM BACK. They ride out to the client and the
	// client draws with them; the pawn's actual size is read from the same
	// assets row again when the spawn is resolved, so a browser that edited
	// these has changed its own preview and nothing else.
	Width  int
	Height int
}

// RoomSpawnAvatar is one face in the library, which is a picture and a name and
// nothing else.
//
// IT CARRIES NO PIXEL SIZE, and that is the difference from RoomSpawnToken
// rather than an omission. An avatar is stored square and lands as a CREATURE,
// which is measured in cells off the size select on the form -- so the picture's
// own dimensions say nothing about how much table it covers.
type RoomSpawnAvatar struct {
	ID    string
	Name  string
	Image string
}

// WidthText and HeightText are the two data attributes the card prints. An
// unknown dimension is an empty attribute rather than a zero, which is what
// lets the client tell "this row predates the size columns" from "this token is
// nothing wide" and fall back to one cell for the ghost.
func (t RoomSpawnToken) WidthText() string { return pixelText(t.Width) }

func (t RoomSpawnToken) HeightText() string { return pixelText(t.Height) }

func pixelText(value int) string {
	if value < 1 {
		return ""
	}

	return strconv.Itoa(value)
}

// The three questions the markup asks of the kind.
func (d RoomSpawnData) IsMonsters() bool { return d.Kind == RoomSpawnMonsters }

func (d RoomSpawnData) IsTokens() bool { return d.Kind == RoomSpawnTokens }

func (d RoomSpawnData) IsNPCs() bool { return d.Kind == RoomSpawnNPCs }

// Path is the dialog for one kind, which is what the three kind buttons fetch.
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

// NPCPath is the form behind one face, which is what an avatar card fetches.
// The card includes the search box as well, so the Back button on the form can
// carry the term back to the wall it came from.
func (d RoomSpawnData) NPCPath(assetID string) string {
	return "/fragment/room/spawn-npc?room=" + d.RoomID + "&asset=" + assetID
}

// ResultsID is the element the search replaces.
func (d RoomSpawnData) ResultsID() string { return roomSpawnResultsID }

// SearchLabel is the placeholder and the accessible name, which differ by kind
// because "Search" over a grid of pictures says nothing about what is in it.
func (d RoomSpawnData) SearchLabel() string {
	switch d.Kind {
	case RoomSpawnMonsters:
		return "Search the manual"
	case RoomSpawnNPCs:
		return "Search your faces"
	default:
		return "Search your tokens"
	}
}

// NoMatch is the heading a search that found nothing gets, through the same
// builder the asset manager's four lists use.
func (d RoomSpawnData) NoMatch() string {
	switch d.Kind {
	case RoomSpawnMonsters:
		return noMatchHeading("monsters", d.Query)
	case RoomSpawnNPCs:
		return noMatchHeading("faces", d.Query)
	default:
		return noMatchHeading("tokens", d.Query)
	}
}

// SearchLimit is the longest term the box accepts, which is also what the route
// refuses past. It is the asset name limit because that is the longest thing
// anybody could be searching for.
func (d RoomSpawnData) SearchLimit() string { return strconv.Itoa(AssetNameLimit) }

// EmptyHeading and EmptyBlurb are the states of "there is nothing here", which
// are different sentences and not one with a word swapped: an empty manual
// wants pointing at the Monster Manual, and an empty token library wants
// pointing at the asset manager. Each is one navigation away and none of them
// is reachable from inside this dialog, so saying where is the whole of the
// help available.
func (d RoomSpawnData) EmptyHeading() string {
	switch d.Kind {
	case RoomSpawnMonsters:
		return "Nothing in your manual yet."
	case RoomSpawnNPCs:
		return "No faces in your library yet."
	default:
		return "No tokens in your library yet."
	}
}

func (d RoomSpawnData) EmptyBlurb() string {
	switch d.Kind {
	case RoomSpawnMonsters:
		return "Write up what you plan to run in the Monster Manual, then come back and place it."
	case RoomSpawnNPCs:
		return "Upload faces to Avatars on the Assets page. A face is any portrait you want a townsfolk or a villain to wear."
	default:
		return "Upload tokens on the Assets page. A token is any picture you want to put on a table."
	}
}

// RoomSpawnNPCData is the second step: one face out of the library, and the
// four numbers that turn it into a creature.
//
// IT IS THE WHOLE DIALOG AND NOT A PANEL INSIDE IT, which is why the
// Players-see-it switch is rendered again here rather than left behind on the
// wall. The switch is read at the moment something is armed, and what is armed
// out of this fragment is armed from this fragment -- so a copy that stayed on
// a grid this swap replaced would be a control nothing could see.
//
// THE NAME BOX STARTS EMPTY AND IS REQUIRED, which is the one field here that
// COULD have been filled in and deliberately is not. The picture has a name --
// it is the caption under it in the asset manager -- but that name is what the
// GM calls the FILE, and a library of faces is organised by what is in the
// picture: "bearded man", "hooded woman", "guard 3". The party does not meet a
// bearded man, they meet Aldric, and one face is worn by four different
// townsfolk over a campaign. So the file's name is a label on a picture and the
// pawn's name is a person, and prefilling the second with the first would make
// pressing Place without reading it the path of least resistance.
type RoomSpawnNPCData struct {
	RoomID string

	// Query is the term the wall was filtered by when the face was picked, so
	// that Back returns to the wall the GM was looking at rather than to all of
	// them. It is baked into that one URL rather than included live, because
	// this fragment has no search box of its own for htmx to read.
	Query string

	Avatar RoomSpawnAvatar
}

// BackPath is the avatar wall this face came off, with the term it was filtered
// by.
func (d RoomSpawnNPCData) BackPath() string {
	path := "/fragment/room/spawn?room=" + d.RoomID + "&kind=" + RoomSpawnNPCs
	if d.Query == "" {
		return path
	}

	return path + "&q=" + url.QueryEscape(d.Query)
}

// The form's opening values, as the strings an input takes.
func (d RoomSpawnNPCData) HPValue() string { return strconv.Itoa(NPCDefaultHP) }

func (d RoomSpawnNPCData) ACValue() string { return strconv.Itoa(NPCDefaultAC) }

func (d RoomSpawnNPCData) SizeValue() string { return DefaultSize }

// NameLimit is what the name box accepts, which is the pawn name limit the
// server holds every other name on the table to -- see NameLimit in
// internal/room.
func (d RoomSpawnNPCData) NameLimit() string { return strconv.Itoa(room.NameLimit) }

// HPMax and ACMax are the server's own limits, printed onto the inputs so that
// the browser refuses what room.checkPawn would refuse -- see HPLimit and
// ACLimit in internal/room. The client's check is a courtesy and the server's
// is the rule; this is only what stops the courtesy disagreeing with it.
func (d RoomSpawnNPCData) HPMax() string { return strconv.Itoa(room.HPLimit) }

func (d RoomSpawnNPCData) ACMax() string { return strconv.Itoa(room.ACLimit) }

// npcField is the hook the room bundle reads one of the form's four controls
// by. They are data attributes rather than names because nothing here is a
// form: the values are read out of the DOM when the Place button arms the
// canvas, and there is no submission for a name to belong to.
func npcField(name string) templ.Attributes {
	return templ.Attributes{"data-npc-" + name: true}
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
