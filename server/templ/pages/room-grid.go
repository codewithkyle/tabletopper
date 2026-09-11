package pages

import (
	"strconv"

	"tabletopper/internal/room"
)






























const RoomGridPanel = "room-grid"



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

	
	
	
	
	
	FogPrefill bool

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
