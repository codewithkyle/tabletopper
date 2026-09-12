package pages

import "tabletopper/internal/uievents"

const RoomInitiativeRoundID = "room-initiative-round"

type RoomInitiativeRoundData struct {
	RoomID  string
	Round   string
	Fetched bool
}

func (d RoomInitiativeRoundData) Path() string {
	return "/fragment/room/initiative/round?room=" + d.RoomID
}
func (d RoomInitiativeRoundData) Trigger() string {
	const live = uievents.Initiative + " from:window"
	if d.Fetched {
		return live
	}
	return "load, " + live
}
