package pages

import (
	"strconv"
	"strings"
)

// THE LAYER MANAGER, which is a window and not a modal. It is where a GM builds
// a dungeon out of floors: add one, name it, give it a map, put the players on
// it, drag it up or down the stack, delete it.
//
// IT IS A WINDOW BECAUSE THE WORK IS NOT ONE ACT. Somebody preparing a tower
// adds three floors, picks three maps and checks each one against the table
// behind it; a dialog that covered the table between every step would be shut
// and reopened six times. The map picker it opens IS a modal, because choosing
// one map is a single act with an end.
//
// IT REDRAWS ITSELF FROM THE SOCKET AND NOT FROM ITS OWN REPLIES. Every button
// below answers 204, and the list refetches on room:tabletop -- which the client
// raises for table.updated, the event every one of those commands ends in. So
// the manager is correct in a second tab, and correct when the change came from
// somewhere else entirely.

// RoomLayersData is the whole window.
type RoomLayersData struct {
	RoomID string
	Layers []RoomLayer

	// Full is a room already at the layer limit. The add form stays on screen
	// and says why rather than disappearing, because a control that vanishes
	// reads as a bug and a control that explains itself reads as a limit.
	Full bool
}

// RoomLayer is one floor as the manager draws it.
//
// EVERY DERIVED FACT IS COMPUTED IN THE CONTROLLER AND CARRIED HERE. Whether a
// layer is at the bottom of the stack, how many pawns are standing on it,
// whether its map is a different size from the rest -- each is a decision about
// the room, and the template's job is to print them.
type RoomLayer struct {
	ID    string
	Name  string
	Index int

	Active bool
	Bottom bool
	Top    bool

	Pawns int

	// MapID is empty when the layer has no map at all. MapName is empty when it
	// has one whose asset is no longer in the library -- a map that was deleted
	// or is being re-tiled -- which is worth saying out loud, because the
	// symptom otherwise is a floor that renders nothing.
	MapID   string
	MapName string
	Width   int
	Height  int

	// Mismatch is a map whose dimensions differ from the first mapped layer's.
	// The grid is room-wide on the assumption that a building's floors were
	// exported at one scale, and this is where a room that breaks the
	// assumption gets told so instead of quietly misaligning.
	Mismatch bool
}

func (d RoomLayersData) Path() string {
	return "/fragment/room/layers?room=" + d.RoomID
}

func (d RoomLayersData) AddPath() string {
	return "/rooms/" + d.RoomID + "/layers"
}

func (d RoomLayersData) LayerPath(l RoomLayer) string {
	return "/rooms/" + d.RoomID + "/layers/" + l.ID
}

func (d RoomLayersData) NamePath(l RoomLayer) string {
	return d.LayerPath(l) + "/name"
}

func (d RoomLayersData) MovePath(l RoomLayer) string {
	return d.LayerPath(l) + "/move"
}

func (d RoomLayersData) ActivatePath(l RoomLayer) string {
	return d.LayerPath(l) + "/activate"
}

func (d RoomLayersData) MapPath(l RoomLayer) string {
	return d.LayerPath(l) + "/map"
}

// ChooseMapPath is the picker, which is a fragment because it opens in the
// content modal and every modal fragment lives under /fragment/.
func (d RoomLayersData) ChooseMapPath(l RoomLayer) string {
	return "/fragment/room/maps?room=" + d.RoomID + "&layer=" + l.ID
}

// MoveVals is the body of one reorder, as hx-vals. The index is where the layer
// ends up, counted from the bottom, so Up is one more and Down is one less.
func (d RoomLayersData) MoveVals(l RoomLayer, by int) string {
	return `{"index": "` + strconv.Itoa(l.Index+by) + `"}`
}

// PreviewURL is the same thumbnail the map manager shows, so a GM recognises
// the map in the layer list as the one they uploaded.
func (d RoomLayersData) PreviewURL(l RoomLayer) string {
	return "/assets/images/" + l.MapID + "/preview"
}

// RemovePrompt is the confirm modal's message, composed here rather than in the
// template because it has two forms and a number in it.
//
// IT NAMES THE PAWN COUNT BECAUSE THE COMMAND DELETES THEM. internal/room drops
// every pawn, fog shape and stroke on a removed layer, and a GM who has parked
// an encounter on the second floor needs that in front of them before they
// press the button, not after. A layer with nothing on it says so by leaving
// the clause out, which is the difference between a warning and a formality.
func (d RoomLayersData) RemovePrompt(l RoomLayer) string {
	if l.Pawns == 0 {
		return "Delete " + l.Name + "? Its fog and drawings go with it. This cannot be undone."
	}

	return "Delete " + l.Name + " and the " + pawnCount(l.Pawns) + " on it? Their fog and drawings go with them. This cannot be undone."
}

func pawnCount(n int) string {
	if n == 1 {
		return "1 pawn"
	}

	return strconv.Itoa(n) + " pawns"
}

// HasMap and MapMissing are the three states of a layer's map, which the
// template switches on: no map, a map, and a map whose asset has gone.
func (l RoomLayer) HasMap() bool { return l.MapID != "" }

func (l RoomLayer) MapMissing() bool { return l.MapID != "" && l.MapName == "" }

// Size is the map's dimensions as a person reads them.
func (l RoomLayer) Size() string {
	return strconv.Itoa(l.Width) + " × " + strconv.Itoa(l.Height)
}

// PawnLabel is the count as the row prints it, and it is empty for a layer with
// nothing on it -- a row that said "0 pawns" would be drawing attention to the
// absence of something nobody asked about.
func (l RoomLayer) PawnLabel() string {
	if l.Pawns == 0 {
		return ""
	}

	return pawnCount(l.Pawns)
}

// LayerNameLimit is the maxlength on the name field. The core refuses a longer
// one with a message; the attribute is what stops it being typed.
const LayerNameLimit = 60

// SafeLayerName is the name with anything that would break out of an attribute
// removed. templ escapes every interpolation already, so this exists for the
// one thing escaping cannot fix: a name that is nothing but whitespace renders
// as a row with no label at all.
func SafeLayerName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "Untitled layer"
	}

	return name
}
