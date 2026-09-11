package pages
const JoinRoomPanel = "join-room"
const NoCharacterValue = ""
type JoinRoomPageData struct {
	Code string
	Characters []JoinCharacterOption
}
type JoinCharacterOption struct {
	ID   string
	Name string
}
