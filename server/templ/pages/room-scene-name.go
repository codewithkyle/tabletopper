package pages

import "tabletopper/internal/uievents"

type RoomSceneNameData struct {
	RoomID  string
	Name    string
	Fetched bool
}

func (d RoomSceneNameData) Path() string {
	return "/fragment/room/scene-name?room=" + d.RoomID
}
func (d RoomSceneNameData) Trigger() string {
	const live = uievents.Tabletop + " from:window, " + uievents.Scenes + " from:window"
	if d.Fetched {
		return live
	}
	return "load, " + live
}
