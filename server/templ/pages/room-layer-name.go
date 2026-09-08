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
}

func (d RoomLayerNameData) Path() string {
	return "/fragment/room/layer?room=" + d.RoomID
}
