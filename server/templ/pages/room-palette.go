package pages

import (
	"net/url"
	"strconv"

	"tabletopper/internal/room"
)

const PaletteWindow = "palette"

type PaletteEntry struct {
	RoomID string
	ID     string
	Name   string
	Image  string
}
type PaletteChoice struct {
	RoomID string
	ID     string
	Name   string
	Image  string
	Held   bool
}
type RoomPaletteData struct {
	RoomID  string
	Query   string
	Entries []PaletteEntry
	Shelf   []PaletteChoice
}

func PaletteWindowPath(roomID string) string {
	return "/fragment/room/palette?room=" + roomID
}
func (d RoomPaletteData) Path() string {
	if d.Query == "" {
		return PaletteWindowPath(d.RoomID)
	}
	return PaletteWindowPath(d.RoomID) + "&q=" + url.QueryEscape(d.Query)
}
func (d RoomPaletteData) ShelfPath() string {
	return d.Path() + "&part=shelf"
}
func (d RoomPaletteData) BagPath() string {
	return PaletteWindowPath(d.RoomID) + "&part=bag"
}
func (d RoomPaletteData) AddPath() string {
	return "/rooms/" + d.RoomID + "/palette"
}
func (d RoomPaletteData) Full() bool {
	return len(d.Entries) >= room.PaletteMax
}
func (d RoomPaletteData) Count() string {
	return strconv.Itoa(len(d.Entries)) + " of " + strconv.Itoa(room.PaletteMax)
}
func (d RoomPaletteData) FullNotice() string {
	return roomPaletteFull
}
func (d RoomPaletteData) EmptyBag() string {
	return roomPaletteEmpty
}
func (d RoomPaletteData) EmptyShelf() string {
	if d.Query == "" {
		return roomPaletteShelfEmpty
	}
	return roomPaletteNoMatch
}
func (d RoomPaletteData) SearchLabel() string {
	return roomPaletteSearchLabel
}
func (d RoomPaletteData) NameLimit() string {
	return strconv.Itoa(AssetNameLimit)
}
func (e PaletteEntry) RemoveLabel() string {
	return "Remove " + e.Name + " from the palette"
}
func (e PaletteEntry) RemovePath() string {
	return "/rooms/" + e.RoomID + "/palette/" + e.ID
}
func (e PaletteEntry) RemovePrompt() string {
	return "Take " + e.Name + " out of the palette. Every cell stamped with it is erased, on every floor. This cannot be undone."
}
func (c PaletteChoice) AddVals() string {
	return `{"asset": "` + c.ID + `"}`
}
func (c PaletteChoice) AddLabel() string {
	return "Add " + c.Name + " to the palette"
}
func (c PaletteChoice) HeldLabel() string {
	return roomPaletteHeld
}

const (
	roomPaletteEmpty       = "Nothing here yet. Add a picture below and it joins the ring under a right-click."
	roomPaletteShelfEmpty  = "No terrain in your library yet. Upload some in the Asset Manager and it shows up here."
	roomPaletteNoMatch     = "No terrain matches that."
	roomPaletteFull        = "The palette is full. Remove a picture before adding another."
	roomPaletteHeld        = "In use"
	roomPaletteSearchLabel = "Search terrain"
)
