package pages

import (
	"strconv"
	"strings"
)

type RoomLayersData struct {
	RoomID string
	Layers []RoomLayer
	Full   bool
}
type RoomLayer struct {
	ID     string
	Name   string
	Index  int
	Active bool
	Bottom bool
	Top    bool
	Pawns  int
	Map    RoomLayerMap
	GMMap  RoomLayerMap
}
type RoomLayerMap struct {
	ID       string
	Name     string
	Width    int
	Height   int
	Mismatch bool
}
type RoomLayerSlot struct {
	Label  string
	Empty  string
	Map    RoomLayerMap
	Path   string
	Picker string
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
func (d RoomLayersData) SlotPath(l RoomLayer, gm bool) string {
	if gm {
		return d.LayerPath(l) + "/gm-map"
	}
	return d.LayerPath(l) + "/map"
}
func (d RoomLayersData) ChooseMapPath(l RoomLayer, gm bool) string {
	picker := "/fragment/room/maps?room=" + d.RoomID + "&layer=" + l.ID
	if gm {
		return picker + "&gm=1"
	}
	return picker
}
func (d RoomLayersData) Slots(l RoomLayer) []RoomLayerSlot {
	return []RoomLayerSlot{
		{
			Label:  roomLayerPlayersSlot,
			Empty:  roomLayerNoMap,
			Map:    l.Map,
			Path:   d.SlotPath(l, false),
			Picker: d.ChooseMapPath(l, false),
		},
		{
			Label:  roomLayerGMSlot,
			Empty:  roomLayerSameAsPlayers,
			Map:    l.GMMap,
			Path:   d.SlotPath(l, true),
			Picker: d.ChooseMapPath(l, true),
		},
	}
}
func (d RoomLayersData) MoveVals(l RoomLayer, by int) string {
	return `{"index": "` + strconv.Itoa(l.Index+by) + `"}`
}
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
func (m RoomLayerMap) Set() bool     { return m.ID != "" }
func (m RoomLayerMap) Missing() bool { return m.ID != "" && m.Name == "" }
func (m RoomLayerMap) PreviewURL() string {
	return "/assets/images/" + m.ID + "/preview"
}
func (m RoomLayerMap) Size() string {
	return strconv.Itoa(m.Width) + " × " + strconv.Itoa(m.Height)
}
func (l RoomLayer) PawnLabel() string {
	if l.Pawns == 0 {
		return ""
	}
	return pawnCount(l.Pawns)
}

const (
	roomLayerPlayersSlot   = "Players see"
	roomLayerGMSlot        = "You see"
	roomLayerNoMap         = "No map"
	roomLayerSameAsPlayers = "Same as the players"
	roomLayerMapGone       = "That map is no longer in your library."
	roomLayerMismatch      = "A different size from the other maps, so the grid will not line up."
)

const LayerNameLimit = 60

func SafeLayerName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "Untitled layer"
	}
	return name
}
