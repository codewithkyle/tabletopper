package pages

import (
	"math"
	"strconv"
)

const (
	TableMenuParty = "party"
	TableMenuErase = "erase"
	tableMenuRing  = 96
	tableMenuSize  = 48
	tableMenuEdge  = 6
)

type RoomTableMenuItem struct {
	Action string
	Label  string
	Index  int
	Count  int
}
type RoomTableMenuArt struct {
	ID    string
	Name  string
	Image string
	Index int
	Count int
}
type RoomTableMenuData struct {
	RoomID string
	Items  []RoomTableMenuItem
	Ring   []RoomTableMenuArt
}

func NewTableMenu(roomID string, isGM bool, ring []RoomTableMenuArt) RoomTableMenuData {
	var items []RoomTableMenuItem
	if len(ring) > 0 {
		items = append(items, RoomTableMenuItem{Action: TableMenuErase, Label: tableMenuEraseLabel})
	}
	if isGM {
		items = append(items, RoomTableMenuItem{Action: TableMenuParty, Label: tableMenuPartyLabel})
	}
	count := len(items) + len(ring)
	for i := range items {
		items[i].Index = i
		items[i].Count = count
	}
	for i := range ring {
		ring[i].Index = len(items) + i
		ring[i].Count = count
	}
	return RoomTableMenuData{RoomID: roomID, Items: items, Ring: ring}
}
func (d RoomTableMenuData) PartyStartPath() string {
	return "/rooms/" + d.RoomID + "/party-start"
}
func (d RoomTableMenuData) Reach() string {
	return strconv.Itoa(ringRadius(len(d.Items)+len(d.Ring)) + tableMenuSize/2 + tableMenuEdge)
}
func (d RoomTableMenuData) Empty() bool {
	return len(d.Items) == 0 && len(d.Ring) == 0
}
func (i RoomTableMenuItem) Placement() map[string]string {
	return placement(spokeDegree(i.Index, i.Count), ringRadius(i.Count))
}
func (i RoomTableMenuItem) IsParty() bool {
	return i.Action == TableMenuParty
}
func (i RoomTableMenuItem) IsErase() bool {
	return i.Action == TableMenuErase
}
func (a RoomTableMenuArt) Placement() map[string]string {
	return placement(spokeDegree(a.Index, a.Count), ringRadius(a.Count))
}
func ringRadius(n int) int {
	return max(tableMenuRing, int(math.Round(float64(n)*tableMenuSize/(2*math.Pi))))
}
func spokeDegree(index int, count int) float64 {
	if count < 1 {
		return 90
	}
	return 90 - float64(index)*360/float64(count)
}
func placement(degree float64, radius int) map[string]string {
	return map[string]string{
		"--degree": strconv.FormatFloat(math.Round(degree*1000)/1000, 'f', -1, 64) + "deg",
		"--radius": strconv.Itoa(radius) + "px",
	}
}
func (d RoomTableMenuData) ClearPartyLabel() string {
	return tableMenuClearPartyLabel
}

const (
	tableMenuPartyLabel      = "Party starts here"
	tableMenuClearPartyLabel = "The party starts here: click to take it back"
	tableMenuEraseLabel      = "Erase this cell"
)
