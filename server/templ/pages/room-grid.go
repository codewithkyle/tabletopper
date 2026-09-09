package pages

import (
	"strconv"

	"tabletopper/internal/room"
)

// THE GRID AND THE TWO ROOM-WIDE OPTIONS, which are one window because they are
// one question: how does this table behave. The grid decides what is drawn and
// what a pawn snaps to; the options decide what a pawn is labelled with and
// whether players may draw. Neither is worth a window of its own and both are
// the GM's.
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

	PawnLabels     string
	PlayersCanDraw bool

	InitiativeGrouping string

	Errors []string
}

func (d RoomGridData) SavePath() string {
	return "/rooms/" + d.RoomID + "/grid"
}

func (d RoomGridData) CellSizeText() string { return strconv.Itoa(d.CellSize) }

func (d RoomGridData) OffsetXText() string { return strconv.Itoa(d.OffsetX) }

func (d RoomGridData) OffsetYText() string { return strconv.Itoa(d.OffsetY) }

func (d RoomGridData) FeetText() string { return strconv.Itoa(d.FeetPerCell) }

// THE COLOUR CONTROL IS A CUSTOM ELEMENT AND NOT <input type="color">, and the
// reason is the alpha. The grid is drawn over somebody's map and is almost
// never wanted opaque, so the alpha pair is the half of the value that
// actually gets tuned -- and a native colour input has no opinion about
// opacity, which left the two digits to be typed and guessed at. The `alpha`
// attribute would answer that on Chromium and Safari; a browser without it
// ignores the attribute and says nothing, which is the same guessing with a
// worse explanation.
//
// SO IT IS vanilla-colorful's <hex-alpha-color-picker>, pinned in package.json
// and bundled into room.js. It was chosen over the popup pickers for one
// reason that matters here more than anywhere: every style it has is inside
// its own shadow root. There is no stylesheet to add to public/css, nothing
// for Tailwind to scan, and no second theme to keep honest against caramellatte
// and coffee.
//
// THE TEXT FIELD IS STILL THE FIELD. It keeps name="color", its pattern and
// its value; the picker writes into it and the form posts what it always
// posted. Nothing on the server has heard of the picker, and a browser running
// no scripts still edits the colour by hand.
//
// AND THE PICKER IS FOLDED AWAY UNTIL IT IS ASKED FOR. This window is 300 by
// 420 and its contents already scroll; a permanent 176-pixel picker would push
// half the settings below the fold for a setting a GM changes once a campaign.
// The swatch beside the field is the disclosure, and it is the first thing on
// the row because it is what tells the GM what the eight digits mean.

// GridColorPickerID is the picker's element id, which exists only so the
// disclosure button can name it in aria-controls. There is one grid window per
// page -- a window's id is its identity -- so a constant is safe here in a way
// it would not be inside a pawn panel.
const GridColorPickerID = "grid-color-picker"

// PickerColor is what the picker opens on. It is the stored colour, except for
// a table that somehow has none: the picker parses whatever it is given and
// would read an empty string as a colour made of NaN, so the default the core
// would have written stands in.
func (d RoomGridData) PickerColor() string {
	if d.Color == "" {
		return room.DefaultGridColor
	}

	return d.Color
}

// SwatchStyle paints the chip on the disclosure button, and it is the FIRST
// paint rather than the only one -- color.ts sets the same property as the
// picker moves. Doing it here is what keeps the chip right on a panel that has
// just been swapped in, which is every time the window is opened: the fragment
// arrives with no script of its own to run.
//
// The template puts white behind the chip, so a colour at a fifth opacity
// looks like a fifth of itself rather than like a darker panel.
func (d RoomGridData) SwatchStyle() map[string]string {
	return map[string]string{"background-color": d.PickerColor()}
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
// rule and what a pawn is labelled with -- each carry the wording a GM reads
// rather than the value the protocol stores. The values are the protocol's own
// and are validated there.
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

// PawnLabelChoices is what the panel over a pawn says, and it is written from
// the players' side because the GM's side only ever changes at one of the three
// -- none, where the GM's own label goes away too.
//
// THE HINTS SPELL OUT THE WORDS BECAUSE THE WORDS ARE THE FEATURE. A GM
// choosing between "words" and "numbers" is choosing whether a fight is
// described or calculated, and "Healthy, bruised, bloody" is what that reads
// like on the table. The full list is six long and the hint gives the ends of
// it rather than all of it; internal/room's HPBand has them all.
// InitiativeGroupingChoices is how a sync turns monsters into lines of the
// turn order, and it is here rather than in the markup for the reason every
// other hint in this file is: Tailwind reads a .templ file as text and takes a
// class-name candidate from every word in it, so a sentence about a card or a
// list in a template is a DaisyUI component family in the stylesheet.
//
// GROUPED IS FIRST AND IS THE DEFAULT because it is how most tables run most
// fights. The hints are written as the fight rather than as the feature: what a
// GM is choosing between is nine turns of bookkeeping and one, and the goblins
// are what makes that concrete.
//
// IT SAYS MONSTERS AND MEANS MONSTERS. An NPC is a named individual and is
// never grouped with another; players never are either. See
// room.InitiativeGrouping.
func InitiativeGroupingChoices() []Choice {
	return []Choice{
		{Value: "grouped", Label: "Grouped", Hint: "Nine goblins take one turn together, on one line, with a dot each for how hurt they are."},
		{Value: "individual", Label: "One at a time", Hint: "Every monster gets a line of its own. Switch to this for a fight where each of them matters."},
	}
}

func PawnLabelChoices() []Choice {
	return []Choice{
		{Value: "none", Label: "None", Hint: "No panel over any pawn, for anybody. Every pawn's window is still yours."},
		{Value: "default", Label: "Default", Hint: "You read the numbers. Players read a name and a word: healthy, bruised, bloody, and so on down to near death."},
		{Value: "full", Label: "Full", Hint: "Everybody reads the name, the armour class and the hit points, the same as you do."},
	}
}
