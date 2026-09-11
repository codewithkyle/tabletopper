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
	ID       string
	Name     string
	Index    int
	Active   bool
	Bottom   bool
	Top      bool
	Pawns    int
	MapID    string
	MapName  string
	Width    int
	Height   int
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
func (d RoomLayersData) ChooseMapPath(l RoomLayer) string {
	return "/fragment/room/maps?room=" + d.RoomID + "&layer=" + l.ID
}
func (d RoomLayersData) MoveVals(l RoomLayer, by int) string {
	return `{"index": "` + strconv.Itoa(l.Index+by) + `"}`
}
func (d RoomLayersData) PreviewURL(l RoomLayer) string {
	return "/assets/images/" + l.MapID + "/preview"
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
func (l RoomLayer) HasMap() bool     { return l.MapID != "" }
func (l RoomLayer) MapMissing() bool { return l.MapID != "" && l.MapName == "" }
func (l RoomLayer) Size() string {
	return strconv.Itoa(l.Width) + " × " + strconv.Itoa(l.Height)
}
func (l RoomLayer) PawnLabel() string {
	if l.Pawns == 0 {
		return ""
	}
	return pawnCount(l.Pawns)
}

const LayerNameLimit = 60

func SafeLayerName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "Untitled layer"
	}
	return name
}
