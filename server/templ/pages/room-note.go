package pages

import (
	"net/url"
	"strconv"

	"tabletopper/internal/room"
)

type RoomNoteData struct {
	RoomID   string
	Layer    string
	Q        int
	R        int
	Title    string
	Body     string
	Revealed bool
	Exists   bool
	IsGM     bool
}

func (d RoomNoteData) SavePath() string {
	return "/rooms/" + d.RoomID + "/notes"
}
func (d RoomNoteData) RevealPath() string {
	return d.SavePath() + "/reveal"
}
func (d RoomNoteData) RemovePath() string {
	return d.SavePath() + "?" + d.cell().Encode()
}
func (d RoomNoteData) cell() url.Values {
	return url.Values{
		"layer": {d.Layer},
		"q":     {strconv.Itoa(d.Q)},
		"r":     {strconv.Itoa(d.R)},
	}
}
func (d RoomNoteData) Across() string {
	return strconv.Itoa(d.Q)
}
func (d RoomNoteData) Down() string {
	return strconv.Itoa(d.R)
}
func (d RoomNoteData) BodyLimit() string {
	return strconv.Itoa(room.NoteBodyLimit)
}
func (d RoomNoteData) TitleLimit() string {
	return strconv.Itoa(room.NameLimit)
}
func (d RoomNoteData) RemoveConfirm() string {
	if d.Title == "" {
		return "The note on this hex goes, for good."
	}
	return "The note on this hex — " + d.Title + " — goes, for good."
}

const noteActions = "room-note-actions"
