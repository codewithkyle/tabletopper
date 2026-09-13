package pages

import (
	"strconv"

	"tabletopper/internal/queries"
)

const RoomMapListID = "room-map-list"

type RoomMapsData struct {
	RoomID  string
	LayerID string
	GM      bool
	Query   string
	Maps    []RoomMapChoice
}
type RoomMapChoice struct {
	RoomID     string
	LayerID    string
	GM         bool
	ID         string
	Name       string
	FileName   string
	Width      int
	Height     int
	Generation string
	State      queries.AssetsTileState
	AutoRetry  bool
}

func slotQuery(gm bool) string {
	if gm {
		return "&gm=1"
	}
	return ""
}
func slotParam(gm bool) string {
	if gm {
		return "?gm=1"
	}
	return ""
}
func (d RoomMapsData) ListPath() string {
	return "/fragment/room/map-list?room=" + d.RoomID + "&layer=" + d.LayerID + slotQuery(d.GM)
}
func (d RoomMapsData) UploadPath() string {
	return "/rooms/" + d.RoomID + "/layers/" + d.LayerID + "/maps" + slotParam(d.GM)
}
func (m RoomMapChoice) SetPath() string {
	if m.GM {
		return "/rooms/" + m.RoomID + "/layers/" + m.LayerID + "/gm-map"
	}
	return "/rooms/" + m.RoomID + "/layers/" + m.LayerID + "/map"
}
func (m RoomMapChoice) CardURL() string {
	return "/fragment/room/map-card?room=" + m.RoomID + "&layer=" + m.LayerID + "&asset=" + m.ID + slotQuery(m.GM)
}
func (m RoomMapChoice) RetryPath() string {
	return "/rooms/" + m.RoomID + "/layers/" + m.LayerID + "/maps/" + m.ID + slotParam(m.GM)
}
func (m RoomMapChoice) PreviewURL() string {
	return "/assets/images/" + m.ID + "/preview"
}
func (m RoomMapChoice) Usable() bool {
	return m.Generation != ""
}
func (m RoomMapChoice) Polling() bool {
	return m.State == queries.AssetsTileStatePending || m.State == queries.AssetsTileStateWorking
}
func (m RoomMapChoice) Retryable() bool {
	return m.State == queries.AssetsTileStateFailed
}
func (m RoomMapChoice) TileFailure() string { return tileFailureText(m.AutoRetry) }
func (m RoomMapChoice) RetryLabel() string  { return retryLabelText(m.AutoRetry) }
func (m RoomMapChoice) CardClass() string {
	if m.Usable() {
		return roomMapPickableBox
	}
	return roomMapCardBox
}
func (m RoomMapChoice) Size() string {
	if m.Width < 1 || m.Height < 1 {
		return ""
	}
	return strconv.Itoa(m.Width) + " × " + strconv.Itoa(m.Height)
}
func (m RoomMapChoice) FileLabel() string {
	if m.FileName == m.Name {
		return ""
	}
	return m.FileName
}

const (
	roomMapPickerHeading   = "Choose the map the players see"
	roomGMMapPickerHeading = "Choose the map you see"
	roomMapsEmptyHeading   = "No maps yet."
	roomMapsEmptyBlurb     = "Upload one and it is cut into tiles, so it stays sharp however far in you zoom. It becomes choosable here the moment they are ready."
)

func (d RoomMapsData) Heading() string {
	if d.GM {
		return roomGMMapPickerHeading
	}
	return roomMapPickerHeading
}
func (d RoomMapsData) NoMatchHeading() string { return noMatchHeading("maps", d.Query) }
func (d RoomMapsData) EmptyHeading() string   { return roomMapsEmptyHeading }
func (d RoomMapsData) EmptyBlurb() string     { return roomMapsEmptyBlurb }
func (d RoomMapsData) SearchLabel() string    { return "Search your maps" }
func (d RoomMapsData) NameLimit() string      { return strconv.Itoa(AssetNameLimit) }
func (d RoomMapsData) UploadAccept() string   { return imageAccept }
func (d RoomMapsData) SearchTriggers() string { return "input changed delay:250ms, search" }
