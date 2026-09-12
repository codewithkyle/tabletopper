package pages

import "strconv"

const (
	RoomMusicNowID    = "room-music-now"
	RoomMusicListID   = "room-music-list"
	roomMusicUploadID = "room-music-upload"
)

type RoomMusicData struct {
	RoomID     string
	CanControl bool
	Loaded     bool
	Name       string
	Playing    bool
	Loop       bool
	Started    bool
	Query      string
	Tracks     []MusicTrack
}

func (d RoomMusicData) Path() string {
	return "/fragment/room/music?room=" + d.RoomID
}
func (d RoomMusicData) LibraryPath() string {
	return "/fragment/room/music/library?room=" + d.RoomID
}
func (d RoomMusicData) LoadPath() string {
	return "/rooms/" + d.RoomID + "/music"
}
func (d RoomMusicData) PlayPath() string  { return d.LoadPath() + "/play" }
func (d RoomMusicData) PausePath() string { return d.LoadPath() + "/pause" }
func (d RoomMusicData) StopPath() string  { return d.LoadPath() + "/stop" }
func (d RoomMusicData) LoopPath() string  { return d.LoadPath() + "/loop" }
func (d RoomMusicData) NowSelect() string { return "#" + RoomMusicNowID }
func (d RoomMusicData) ListID() string    { return RoomMusicListID }
func (d RoomMusicData) ListTarget() string {
	return "#" + RoomMusicListID
}
func (d RoomMusicData) UploadID() string    { return roomMusicUploadID }
func (d RoomMusicData) SearchLimit() string { return strconv.Itoa(AssetNameLimit) }
func (d RoomMusicData) Title() string {
	if !d.Loaded {
		return "Nothing loaded"
	}
	return d.Name
}
func (d RoomMusicData) StateLabel() string {
	switch {
	case d.Playing:
		return "Playing"
	case d.Started:
		return "Paused"
	default:
		return "Ready"
	}
}
func (d RoomMusicData) TrackVals(t MusicTrack) string {
	return `{"track":"` + t.ID + `"}`
}
func (d RoomMusicData) EmptyText() string {
	if d.Query != "" {
		return "No track of yours is called that."
	}
	return "No music in your library yet. Upload a track to get started."
}
