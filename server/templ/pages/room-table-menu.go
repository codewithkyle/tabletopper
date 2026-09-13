package pages

import "strconv"

const (
	TableMenuParty  = "party"
	tableMenuRadius = 46
	tableMenuItem   = 32
	tableMenuEdge   = 6
)

type RoomTableMenuItem struct {
	Action string
	Label  string
	Degree int
}
type RoomTableMenuData struct {
	RoomID string
	Items  []RoomTableMenuItem
}

func TableMenuItems(isGM bool) []RoomTableMenuItem {
	if !isGM {
		return nil
	}
	return []RoomTableMenuItem{
		{Action: TableMenuParty, Label: tableMenuPartyLabel, Degree: 90},
	}
}
func (d RoomTableMenuData) PartyStartPath() string {
	return "/rooms/" + d.RoomID + "/party-start"
}
func (d RoomTableMenuData) Reach() string {
	return strconv.Itoa(tableMenuRadius + tableMenuItem/2 + tableMenuEdge)
}
func (d RoomTableMenuData) Empty() bool {
	return len(d.Items) == 0
}
func (i RoomTableMenuItem) Placement() map[string]string {
	return map[string]string{
		"--degree": strconv.Itoa(i.Degree) + "deg",
		"--radius": strconv.Itoa(tableMenuRadius) + "px",
	}
}
func (i RoomTableMenuItem) IsParty() bool {
	return i.Action == TableMenuParty
}

const tableMenuPartyLabel = "Party starts here"
