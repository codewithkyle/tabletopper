package pages

import (
	"strconv"

	"tabletopper/internal/room"
)

const RoomGridPanel = "room-grid"

type RoomGridData struct {
	RoomID      string
	Lines       string
	CellSize    int
	OffsetX     int
	OffsetY     int
	Color       string
	Snap        string
	FeetPerCell int
	Diagonals   string
	Errors      []string
}

func (d RoomGridData) SavePath() string {
	return "/rooms/" + d.RoomID + "/grid"
}
func (d RoomGridData) Path() string {
	return "/fragment/room/grid?room=" + d.RoomID
}
func (d RoomGridData) CellSizeText() string { return strconv.Itoa(d.CellSize) }
func (d RoomGridData) OffsetXText() string  { return strconv.Itoa(d.OffsetX) }
func (d RoomGridData) OffsetYText() string  { return strconv.Itoa(d.OffsetY) }
func (d RoomGridData) FeetText() string     { return strconv.Itoa(d.FeetPerCell) }

const GridColorPickerID = "grid-color-picker"

func (d RoomGridData) PickerColor() string {
	if d.Color == "" {
		return room.DefaultGridColor
	}
	return d.Color
}
func (d RoomGridData) SwatchStyle() map[string]string {
	return map[string]string{"background-color": d.PickerColor()}
}

const (
	GridCellMin  = "8"
	GridCellMax  = "512"
	GridFeetMin  = "1"
	GridFeetMax  = "1000"
	GridColorPat = "#?[0-9a-fA-F]{6}([0-9a-fA-F]{2})?"
)

type Choice struct {
	Value string
	Label string
	Hint  string
}

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
