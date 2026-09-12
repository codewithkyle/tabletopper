package pages

import "strconv"

type RoomDebugRow struct {
	Label string
	Value string
	Warn  bool
}
type RoomDebugGroup struct {
	Label string
	Rows  []RoomDebugRow
}
type RoomDebugConn struct {
	User  string
	Count int
	Over  bool
}
type RoomDebugServerData struct {
	RoomID     string
	Live       bool
	Groups     []RoomDebugGroup
	PerUser    []RoomDebugConn
	Coalescing []string
}

func (d RoomDebugServerData) Path() string {
	return "/fragment/room/debug/server?room=" + d.RoomID
}

func (c RoomDebugConn) CountLabel() string {
	return strconv.Itoa(c.Count)
}
