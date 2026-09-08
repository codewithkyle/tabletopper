package pages

import "strconv"

// THE GRID AND THE TWO ROOM-WIDE OPTIONS, which are one window because they are
// one question: how does this table behave. The grid decides what is drawn and
// what a pawn snaps to; the options decide what players are told about a
// monster's health and whether they may draw. Neither is worth a window of its
// own and both are the GM's.
//
// IT IS A WINDOW, SO IT DOES NOT CLOSE ON SAVE. A GM setting a cell size is
// matching it against a map they can see, which takes three tries; a dialog
// that dismissed itself after each one would be reopened twice. Every field
// saves on change and the table updates underneath.
//
// AND IT IS THE ONE TABLE WINDOW THAT DOES NOT REFETCH ITSELF ON room:tabletop,
// which every other one does. Its own save raises that event, so a form
// listening for it would replace itself a few milliseconds after every change
// -- and a field replaced while somebody is tabbing through the form takes the
// focus with it. The layer manager can afford the refetch because what changes
// there is structural; a form's fields are only ever changed by the person
// looking at them, and reopening the window is the way to reread the table.
//
// THE ERRORS GO WHERE THE CHARACTER PANELS' DO. A refusal here is "Cell size
// must be between 8 and 512 pixels", which belongs above the field that says 4
// and not in the alert modal -- so the form carries the panel trio, answers
// 422, and swaps PanelFormErrors. That is also why a save that works answers an
// EMPTY error block rather than 204: something has to clear the message the
// last attempt left on screen.

// RoomGridPanel is the id the error block and the form's target share. The
// panel helpers build "errors-" + this, and a target that has drifted from its
// block fails silently -- so it is written once.
const RoomGridPanel = "room-grid"

// RoomGridData is the form, flattened out of room.Table so the template reads
// fields rather than reaching through two structs.
type RoomGridData struct {
	RoomID string

	Lines       string
	CellSize    int
	OffsetX     int
	OffsetY     int
	Color       string
	Snap        string
	FeetPerCell int
	Diagonals   string

	MonsterHP      string
	PlayersCanDraw bool

	Errors []string
}

func (d RoomGridData) SavePath() string {
	return "/rooms/" + d.RoomID + "/grid"
}

func (d RoomGridData) CellSizeText() string { return strconv.Itoa(d.CellSize) }

func (d RoomGridData) OffsetXText() string { return strconv.Itoa(d.OffsetX) }

func (d RoomGridData) OffsetYText() string { return strconv.Itoa(d.OffsetY) }

func (d RoomGridData) FeetText() string { return strconv.Itoa(d.FeetPerCell) }

// ColorRGB is the colour without its alpha, because a native colour input takes
// six digits and has no opinion about opacity. The text field beside it is the
// one that holds all eight, and the picker writes its choice into that field's
// first six.
func (d RoomGridData) ColorRGB() string {
	if len(d.Color) >= 7 {
		return d.Color[:7]
	}

	return "#000000"
}

// The bounds the core enforces, as attributes. A browser that refuses 4 before
// the request leaves is a browser that saved a round trip; the server refuses
// it again because a browser is not where rules live.
const (
	GridCellMin  = "8"
	GridCellMax  = "512"
	GridFeetMin  = "1"
	GridFeetMax  = "1000"
	GridColorPat = "#?[0-9a-fA-F]{6}([0-9a-fA-F]{2})?"
)

// The four closed sets below -- the line style, the snapping mode, the diagonal
// rule and what players are told about a monster's health -- each carry the
// wording a GM reads rather than the value the protocol stores. The values are
// the protocol's own and are validated there.
type Choice struct {
	Value string
	Label string
	Hint  string
}

// GridLineChoices is the grid's own control, and off is one of its three
// answers rather than a switch beside them. A GM looking at a map asks one
// question -- what do I want over this -- and a checkbox plus a style select
// would be two controls for it, with a fourth state (off, but dashed) that
// means nothing.
func GridLineChoices() []Choice {
	return []Choice{
		{Value: "off", Label: "Off", Hint: "No lines at all. Pawns still snap and distances are still counted."},
		{Value: "solid", Label: "Solid"},
		{Value: "dashed", Label: "Dashed", Hint: "Easier to read over a map that has its own floor drawn on it."},
	}
}

func GridSnapChoices() []Choice {
	return []Choice{
		{Value: "cells", Label: "Centre only", Hint: "A creature stands in the middle of the squares it fills."},
		{Value: "halfCells", Label: "Centre and corners", Hint: "Half a square at a time, so a pawn may also stand where four squares meet."},
		{Value: "off", Label: "No snapping", Hint: "Pawns go exactly where they are dropped."},
	}
}

func GridDiagonalChoices() []Choice {
	return []Choice{
		{Value: "equal", Label: "Every diagonal counts one square"},
		{Value: "alternating", Label: "Every second diagonal counts two"},
	}
}

func GridHPChoices() []Choice {
	return []Choice{
		{Value: "hidden", Label: "Nothing"},
		{Value: "band", Label: "A health bar"},
		{Value: "exact", Label: "The exact numbers"},
	}
}
