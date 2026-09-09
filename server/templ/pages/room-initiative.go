package pages

import "strconv"

// THE TURN ORDER IS A STRIP OF FACES ACROSS THE TOP OF THE TABLE, and every
// decision in this file follows from that one.
//
// A TURN ORDER IS SCANNED AND NOT READ. The question a fight asks of it, twenty
// times a round, is "whose go is it and who is next" -- and a face answers that
// in the time a name takes to be focused on. This rebuild is built on every
// creature having a picture, so the picture is the whole design: a round
// portrait, the name under it small and truncated, and nothing else on a
// resting line. Everything a GM might also want -- armour class, the floor, the
// full condition list -- is one double click away in the pawn window, which
// already exists.
//
// IT IS NOT A WINDOW AND THERE IS NO EDITOR BESIDE IT. The rule that a panel
// which is not the table is a floating window was made about the player list: a
// column taking a fifth of the table for something read once an hour. This is
// read by everybody, every few seconds, for exactly the minutes a fight lasts,
// and a window would have to be opened by each person at the table from a menu
// the players do not have. And there is no second surface for editing it,
// because with the numbers gone that window's whole content would have been the
// same faces in a column -- a second thing to build, to style, to keep live and
// to pin in tests, for the privilege of dragging vertically instead of
// horizontally. The GM edits the thing they are already looking at.
//
// THE NUMBERS ARE GONE ENTIRELY. No box on a line, no box in a dialog, no Sort,
// no renumber route. The slice order was always the turn order and the number
// was always informational; with a drag there is nothing left for it to inform.
// A GM who rolls on paper drags into the order they read off the paper, which
// is one gesture instead of twelve keystrokes and a press.
//
// EVERY LINE WEARS ITS CREATURE'S WOUNDS, and it reads exactly what the canvas
// reads: one band, from room.Health, spent on blood at the rim, the colour
// draining out, a pulse inside the rim at two rates, a splatter, and a skull.
// The whole of that is CSS on [data-band] in server/css/app.css -- see the note
// there -- because all of it hangs off one attribute this file renders, and
// because the numbers behind it are wounds.ts's, which is already the mirror of
// hpBand in the Go.
//
// IT LEAKS NOTHING. projectPawn withholds a monster's armour class from players
// and sends its hit points on purpose, because the canvas draws blood from the
// number; a bloodied line is the bloodied sprite in a second place. What the
// label setting still governs is the TEXT, and that is the HP field below.
//
// THE WORD "CARD" APPEARS NOWHERE IN THE MARKUP, and that is the trap this
// feature walks straight into -- a card is what everybody calls these. `card`
// is a DaisyUI component and Tailwind reads a .templ file as text, so the word
// in a class, an attribute name, an attribute value or a Go identifier inside
// the template emits the whole .card family into the stylesheet with nothing
// anywhere failing. In the markup a line of the tracker is an ENTRY. The same
// goes for `list`, `table`, `tab`, `status`, `stack`, `swap`, `indicator`,
// `steps`, `timeline`, `countdown`, `chat`, `mask`, `dock` and `diff`. A
// template test asserts the rendered strip contains none of them.

// RoomInitiativeID is the strip's element id. The placeholder room.templ
// renders and the fragment that replaces it share it, because the fragment
// swaps itself outerHTML and an id that drifted would leave two of them.
const RoomInitiativeID = "room-initiative"

// InitiativeTrigger is the fragment's own refetch, and both halves of it are
// load-bearing.
//
// IT LISTENS FOR room:initiative, WHICH panels.ts RAISES FOR MORE THAN THE
// TRACKER. initiative.updated raises it, and so does a pawn.updated for a pawn
// the tracker names -- because damage arrives as pawn.updated and without that
// the blood on these faces would be stale until the turn advanced, which is the
// one thing the wound treatment cannot afford.
//
// THE FILTER IS WHAT KEEPS A REFETCH FROM DROPPING A HELD LINE. A swap that
// replaced the strip under a pointer mid-drag would take the element being
// dragged out of the document. Sortable sets data-dragging on the root at the
// start of a drag and clears it at the end; an update that arrives during one
// is lost, and the POST on drop brings a fresh one back a moment later.
//
// NEITHER A SQUARE BRACKET NOR A COMMA MAY APPEAR INSIDE THE FILTER. The filter
// is delimited by the brackets around it and the attribute is split on commas,
// so either one ends the expression early and leaves the rest parsed as trigger
// modifiers -- which is not an error, it is a strip that has quietly stopped
// refetching. See RoomPawnData.Trigger, which learned this the same way.
const InitiativeTrigger = "room:initiative[!this.hasAttribute('data-dragging')] from:window"

// InitiativeLoadTrigger is the placeholder's, which fetches once on load and
// then behaves like the fragment it is replaced by. The fragment's own root
// must NOT carry `load`: a root that did would fetch itself again on every
// swap, for ever.
const InitiativeLoadTrigger = "load, " + InitiativeTrigger

// The three shapes a line of the tracker takes. They are values rather than two
// booleans because the three are exclusive and a template switching on one
// string cannot render half of two of them.
const (
	// EntrySolo is one creature: a portrait, a name, its wounds, and its
	// conditions when it is acting.
	EntrySolo = "solo"

	// EntryGroup is several of one monster acting together: the portrait of
	// the worst-hurt one still standing, and a dot per member.
	EntryGroup = "group"

	// EntryNamed is a line with no pawn at all -- a lair action, a legendary
	// action, the thing that acts on a count and is not a creature.
	EntryNamed = "named"
)

// InitiativePipMax is how many members a group draws one dot each for before
// the dots become a count. Twenty four-pixel discs under a portrait is a
// texture rather than a reading.
const InitiativePipMax = 12

// initiativeBloodVariants is how many splatters the sheet was cut into, and it
// is BLOOD_VARIANTS in server/js/room/render/wounds.ts. It is written again
// here rather than imported because the number belongs to the pictures in
// server/public/images/blood and not to the protocol, and templ/pages renders
// HTML rather than holding protocol types.
const initiativeBloodVariants = 9

// RoomInitiativeData is the strip for one viewer. Every string on it has
// already been decided; the templates print them.
type RoomInitiativeData struct {
	RoomID string

	// IsGM decides three things at once: whether the lines are draggable,
	// whether they can be clicked to activate, and whether hit points are
	// printed on every line rather than on the acting one alone.
	IsGM bool

	// Empty renders the whole strip hidden. An empty tracker is no strip at
	// all -- not an empty panel -- because the table underneath it is what the
	// room is for.
	Empty bool

	Entries []RoomInitiativeEntry
}

// RoomInitiativeEntry is one line of the tracker as the strip draws it.
type RoomInitiativeEntry struct {
	ID   string
	Name string

	// Kind is EntrySolo, EntryGroup or EntryNamed.
	Kind string

	// Image is the portrait, and Band is what room.Health answered for the
	// creature it belongs to -- empty for a viewer who was told nothing, which
	// draws the face plain.
	Image string
	Band  string

	// Blood is which of the nine splatters this line wears, as the digit the
	// stylesheet keys on. It is chosen from the pawn's id so that everybody at
	// the table sees the same one, and it is deliberately NOT the one the
	// canvas chose: matching would mean mirroring seed() in Go for a difference
	// nobody can see at forty-eight pixels, and what the nine are for is
	// variety across a strip of twelve.
	Blood string

	// HP is the hit-point text -- "12 / 20", or the band's word for a viewer
	// who gets words. It follows the room's label setting exactly as the pawn
	// panel does, and it is empty in a room labelling nothing.
	HP string

	// Active is the line whose turn it is. Mine is that line belonging to the
	// person looking, which is what draws the timer and End turn.
	Active bool
	Mine   bool

	// Hidden badges a line whose pawn players cannot see. A player never
	// receives such a line at all, so this is never true on their copy.
	Hidden bool

	// Solo is the pawn id a double click opens the window for, and it is set
	// for a one-creature line alone: a group is nine windows and a named line
	// is none.
	Solo string

	// Pips is one dot per member of a group, in that member's own colour, and
	// Count is what replaces them past InitiativePipMax.
	Pips  []RoomInitiativePip
	Count string

	// Conditions are the chips on the acting line, and a group carries none:
	// nine goblins have nine sets of them and the rings on the table are where
	// that lives.
	Conditions []RoomPawnCondition
}

// RoomInitiativePip is one member of a group, and it is a band and nothing
// else. The stylesheet draws it from the same [data-band] rules the portrait
// uses, at dot size, so six up and three down is six coloured discs and three
// grey ones with one rule set doing both.
type RoomInitiativePip struct {
	Band string
}

// Path is the fragment's own URL, so the copy swapped in refetches itself the
// way the first one did. The room travels as a query parameter rather than in
// the path for the reason MembersPath's does: this is a representation of a
// room's state and not a resource of its own.
func (d RoomInitiativeData) Path() string {
	return "/fragment/room/initiative?room=" + d.RoomID
}

func (d RoomInitiativeData) Trigger() string { return InitiativeTrigger }

// The four verbs that are not a gesture on a line. They are mutations, so they
// keep their resource URLs; each ends in initiative.updated and answers 204,
// and the strip refetches from the event rather than from the reply -- which is
// what corrects the GM's second tab at the same instant as the first.
func (d RoomInitiativeData) SyncPath() string {
	return "/rooms/" + d.RoomID + "/initiative/sync"
}

// NextPath is rendered as a hidden button on the GM's strip and as End turn on
// a player's own acting line.
//
// THE GM'S IS HIDDEN BECAUSE NOTHING THERE IS PRESSED. A Next button used to
// sit at the far end of the row; it was a control inside a display, it moved
// every time the order changed, and it was the only thing on this surface a GM
// operated rather than read. What is left is the element the N key presses --
// see initiative.ts, which learns no route -- and the Initiative menu, which
// posts the same URL and prints the key beside its label.
func (d RoomInitiativeData) NextPath() string {
	return "/rooms/" + d.RoomID + "/initiative/next"
}

// ClearPath is a POST rather than a DELETE, and the reason is the menu rather
// than the verb: a menu item is a button carrying hx-post, and making this one
// the exception would mean a second branch in RoomMenuItem's markup for one
// route. Clear tabletop is a POST for the same reason.
func (d RoomInitiativeData) ClearPath() string {
	return "/rooms/" + d.RoomID + "/initiative/clear"
}

// OrderPath is where a drop posts the whole order. The client writes the ids
// into the hidden button's hx-vals and presses it, which is pawn-menu.ts's
// pattern and is what keeps every request htmx's rather than half of them the
// module's.
func (d RoomInitiativeData) OrderPath() string {
	return "/rooms/" + d.RoomID + "/initiative/order"
}

// The two gestures on a line.
func (d RoomInitiativeData) ActivatePath(e RoomInitiativeEntry) string {
	return "/rooms/" + d.RoomID + "/initiative/" + e.ID + "/activate"
}

func (d RoomInitiativeData) RemovePath(e RoomInitiativeEntry) string {
	return "/rooms/" + d.RoomID + "/initiative/" + e.ID
}

// ActivateLabel is what a screen reader is told pressing a line does, and it is
// built in Go rather than written in the markup for the reason
// RoomPawnData.RemoveLabel is: Tailwind reads attribute VALUES as class-name
// candidates too, so an aria-label in a template is prose in the stylesheet's
// input. It names the creature because a strip is a row of a dozen of these and
// "make active" on its own does not say make what active.
func (d RoomInitiativeData) ActivateLabel(e RoomInitiativeEntry) string {
	return "Give the turn to " + e.Name
}

// Face is whether this line has a portrait to draw at all. A named line does
// not, and takes its own text in the disc's place at the size the portrait
// would have been, so the strip's rhythm survives.
func (e RoomInitiativeEntry) Face() bool { return e.Kind != EntryNamed }

// Grouped is whether the dots under the portrait are drawn.
func (e RoomInitiativeEntry) Grouped() bool { return e.Kind == EntryGroup }

// ShowHP is who reads the hit-point text: the GM on every line, and a player on
// the acting line alone.
//
// A PLAYER READS IT ON THE ACTING LINE BECAUSE THAT LINE IS BIG ENOUGH TO CARRY
// IT and because what is happening to the creature acting right now is the
// thing everybody at the table is already discussing. On the other eleven it
// would be a column of numbers under a row of faces, which is the spreadsheet
// this design exists to not be.
func (d RoomInitiativeData) ShowHP(e RoomInitiativeEntry) bool {
	return e.HP != "" && (d.IsGM || e.Active)
}

// InitiativeRoundText is the counter's text: the number, or a dash for a
// tracker that has been built and not started. The counter itself is in the
// menu bar; see room-initiative-round.go.
func InitiativeRoundText(round int) string {
	if round < 1 {
		return "--"
	}

	return strconv.Itoa(round)
}

// InitiativePips turns a group's members into dots, or into a count when there
// are more of them than anybody can read as dots.
func InitiativePips(bands []string) ([]RoomInitiativePip, string) {
	if len(bands) > InitiativePipMax {
		return nil, "x" + strconv.Itoa(len(bands))
	}

	pips := make([]RoomInitiativePip, 0, len(bands))
	for _, b := range bands {
		pips = append(pips, RoomInitiativePip{Band: b})
	}

	return pips, ""
}

// InitiativeBloodVariant is which of the nine splatters a creature wears, as
// the digit the stylesheet keys on.
//
// IT IS FNV-1a OVER THE PAWN'S ID, which is seed() in wounds.ts written again
// here -- the same hash of the same string, so that everybody at the table sees
// the same splatter on the same goblin. What it is NOT is the variant the
// CANVAS chose for that goblin: the canvas seeds per mark rather than per pawn,
// and matching the two would be mirroring a second function for a difference
// that is invisible at forty-eight pixels.
func InitiativeBloodVariant(pawnID string) string {
	var h uint32 = 0x811c9dc5
	for i := range len(pawnID) {
		h ^= uint32(pawnID[i])
		h *= 0x01000193
	}

	return strconv.Itoa(int(h%initiativeBloodVariants) + 1)
}

// RoomInitiativeEntryData is the Add entry dialog: one field, for the line that
// has no pawn behind it.
//
// IT EXISTS BECAUSE SYNC CANNOT REACH A LAIR ACTION. Sync covers every creature
// a player can see on a floor they are standing on, which is the fight; what it
// cannot reach is a thing that acts on a count and is not a creature. The other
// half of the same gap -- a creature the GM wants in the order that Sync would
// not take -- is Add to initiative on the pawn's own menu, which is where the
// GM already is when they are looking at that goblin.
type RoomInitiativeEntryData struct {
	RoomID string
	Errors []string
}

// RoomInitiativeEntryPanel is the dialog's error slot. It needs no id of its
// own: there is one content modal on the page and it holds one dialog at a
// time.
const RoomInitiativeEntryPanel = "initiative-entry"

func (d RoomInitiativeEntryData) SavePath() string {
	return "/rooms/" + d.RoomID + "/initiative"
}

// EntryNameMax is the maxlength the field prints, and it is room.NameLimit
// written where the markup can reach it. A test pins the two together so a
// change to one is a failure rather than a field that accepts what the server
// will not.
const EntryNameMax = "128"
