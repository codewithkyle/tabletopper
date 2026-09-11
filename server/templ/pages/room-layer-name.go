package pages

import "tabletopper/internal/events"




















type RoomLayerNameData struct {
	RoomID string

	
	
	Name string

	
	
	
	
	
	
	
	
	
	
	
	Fetched bool
}

func (d RoomLayerNameData) Path() string {
	return "/fragment/room/layer?room=" + d.RoomID
}



func (d RoomLayerNameData) Trigger() string {
	const live = events.Tabletop + " from:window"

	if d.Fetched {
		return live
	}

	return "load, " + live
}
