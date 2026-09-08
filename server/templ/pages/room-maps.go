package pages

import "strconv"

// THE MAP PICKER, which is a content modal and not a window.
//
// Choosing a map is one act with an end: it is opened from a row in the layer
// manager, it answers one question, and it closes. That is the whole of the
// difference between the two mechanisms, and it is why this is the only thing
// in the table family that sends HX-Trigger modal:close.
//
// IT SHOWS ONLY MAPS THAT HAVE FINISHED TILING. A map with no pyramid in the
// bucket would give every browser at the table a URL that 404s at every zoom,
// so the query filters on tile_gen rather than the card explaining itself. The
// empty state is where a GM with nothing ready is told what to do about it.

// RoomMapsData is the picker.
type RoomMapsData struct {
	RoomID  string
	LayerID string
	Maps    []RoomMapChoice
}

// RoomMapChoice is one card.
type RoomMapChoice struct {
	ID     string
	Name   string
	Width  int
	Height int
}

// SetPath is the layer's map resource, which every card posts to with its own
// asset id. It is the layer's URL and not the picker's, because what is being
// changed is the layer.
func (d RoomMapsData) SetPath() string {
	return "/rooms/" + d.RoomID + "/layers/" + d.LayerID + "/map"
}

func (d RoomMapsData) PreviewURL(m RoomMapChoice) string {
	return "/assets/images/" + m.ID + "/preview"
}

// Size is the map's dimensions, which is the one fact that decides whether a
// map belongs on the same table as the others: the grid is room-wide, so two
// floors exported at different scales will not line up.
func (m RoomMapChoice) Size() string {
	return strconv.Itoa(m.Width) + " × " + strconv.Itoa(m.Height)
}
