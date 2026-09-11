package pages

const NewRoomPanel = "new-room"
const RoomNameLimit = 128

type RoomsPageData struct {
	Rooms []RoomSummary
}
type RoomSummary struct {
	ID     string
	Name   string
	Code   string
	Locked bool
	Closed bool
}
