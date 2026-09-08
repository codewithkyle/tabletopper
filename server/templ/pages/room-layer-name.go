package pages

// THE ACTIVE LAYER'S NAME, IN THE MENU BAR. It is the one piece of table state
// that has to be glanceable: a GM who has forgotten which floor is live moves
// pawns onto one nobody can see, and a player whose map changed under them
// wants to know it was a floor and not a bug.
//
// IT IS A FRAGMENT AND NOT A VALUE THE CLIENT FILLS IN. The alternative is a
// script that reads the store and prints a name, which means a second thing to
// keep in step with the reducer for one string that changes when somebody
// clicks a menu item. The refetch costs one GET per floor change per client,
// which is the same trade the player list already makes.
//
// IT FETCHES ITSELF ON LOAD, so the page render does not have to reach into the
// hub -- and so that a room whose actor is not loaded yet is not booted by
// drawing its chrome.
type RoomLayerNameData struct {
	RoomID string

	// Name is empty for a room with one layer, which is nearly every room, and
	// the span then renders as nothing at all.
	Name string

	// Fetched marks the render that IS the answer to that fetch, and it is
	// here to stop this element replacing itself for the rest of the session.
	//
	// hx-trigger="load" fires the moment htmx processes an element, and htmx
	// processes whatever it swaps in. Every other live panel gets away with
	// answering itself an outerHTML swap because none of them listens for
	// load; this one does, so its own answer re-armed it the instant it
	// landed. The symptom is a GET per round trip for ever, the loading bar up
	// for good, and -- because html[state="loading"] * sets it -- the whole
	// page under a wait cursor. The page render asks once. The answer listens
	// for the socket and nothing else.
	Fetched bool
}

func (d RoomLayerNameData) Path() string {
	return "/fragment/room/layer?room=" + d.RoomID
}

// Trigger is what the span listens for, which is one event more on the page
// render than it is in the answer. See Fetched.
func (d RoomLayerNameData) Trigger() string {
	const live = "room:tabletop from:window"

	if d.Fetched {
		return live
	}

	return "load, " + live
}
