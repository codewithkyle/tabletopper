package pages

import "tabletopper/internal/events"

// THE ROUND COUNTER LIVES IN THE MENU BAR, NOT ON THE STRIP.
//
// IT WAS AT THE LEADING EDGE OF THE TURN ORDER AND IT WAS IN THE WAY. The strip
// is a row of faces that has to stay readable at twelve combatants on a laptop,
// and every pixel in front of the first face is a pixel the faces do not get.
// The round is also the one number on that surface nobody scans -- it changes
// once every twelve turns and is quoted aloud maybe twice a fight -- so it was
// paying prime horizontal space for a reading that is occasional by nature.
//
// AND THE BAR ALREADY HAS THE SLOT. The right-hand end of the room bar is where
// the table's ambient state sits: which floor is live, and for a GM the picker
// that changes it. "Round 3" is exactly that kind of fact -- true of the whole
// table, read when somebody wonders, never acted on -- so it goes beside its
// neighbour rather than on top of the thing it is about.
//
// IT IS ALSO WHAT PUSHES THE FLOOR ELEMENT RIGHT. The bar is a flex row and the
// right-hand group used to begin at the floor; it begins here now, which is why
// this span carries ml-auto and room-layer-name.templ no longer does. Two
// elements with an auto left margin would split the free space between them and
// leave the round stranded in the middle of the bar.
//
// NOTHING IS RENDERED FOR AN EMPTY TRACKER. A room that is not in a fight is
// every room most of the time, and a bar that permanently reads "Round --" is a
// label for a thing that is not happening. The span itself stays -- it is what
// hears the next event -- and it is empty, which in a flex row is nothing at
// all.

// RoomInitiativeRoundID is the counter's element id. The page render and the
// fragment that replaces it share it, because the fragment swaps itself
// outerHTML and an id that drifted would leave two of them in the bar.
const RoomInitiativeRoundID = "room-initiative-round"

// RoomInitiativeRoundData is the counter for one viewer, and the round is
// everybody's: a player asking which round it is gets the same answer the GM
// reads.
type RoomInitiativeRoundData struct {
	RoomID string

	// Round is what the counter prints, and an empty one prints nothing at all
	// -- which is the whole of the rule above and is why it is one field
	// rather than a string beside a flag.
	//
	// A TRACKER THAT HAS BEEN BUILT AND NOT STARTED READS AS A DASH rather
	// than as zero, which is a round nobody is in. See InitiativeRoundText.
	//
	// AND THE PAGE RENDER LEAVES IT EMPTY, because the page does not reach
	// into the hub for it: the span arrives blank, its load fetch answers a
	// moment later, and a room that is not in a fight never draws anything.
	// The zero value doing the right thing is what keeps a bare "Round" label
	// off the bar for the length of one round trip.
	Round string

	// Fetched marks the render that IS the answer to the load fetch, and it is
	// here for the reason RoomLayerNameData.Fetched is: hx-trigger="load"
	// fires when htmx processes an element, htmx processes whatever it swaps
	// in, and an answer that re-armed its own trigger is a GET per round trip
	// for ever with the page under a wait cursor the whole time.
	Fetched bool
}

// Path is the counter's own URL. It sits under the strip's rather than beside
// it because it is the same feature read a second way, and the room travels as
// a query parameter for the reason InitiativePath's does: this is a
// representation of a room's state and not a resource of its own.
func (d RoomInitiativeRoundData) Path() string {
	return "/fragment/room/initiative/round?room=" + d.RoomID
}

// Trigger is what the counter listens for, which is one event more on the page
// render than it is in the answer. See Fetched.
//
// IT IS THE STRIP'S EVENT AND NOT ONE OF ITS OWN. room:initiative is raised by
// initiative.updated and also by a pawn.updated for a pawn the tracker names,
// so a goblin taking damage costs this span a GET it did not need -- the round
// cannot change that way. A second DOM event raised only by initiative.updated
// would save it, and would be a new name in the socket-to-DOM bridge whose one
// consumer is a two-word span that refetches beside a strip already refetching
// for the same reason. The round and the strip are one surface read in two
// places; they update together.
//
// IT CARRIES NO FILTER, WHERE THE STRIP'S DOES. The strip declines an update
// while a line is being dragged, because a swap would take the held element out
// of the document; nothing here is ever held, so the filter would be a
// condition on an attribute this span cannot have.
func (d RoomInitiativeRoundData) Trigger() string {
	const live = events.Initiative + " from:window"

	if d.Fetched {
		return live
	}

	return "load, " + live
}
