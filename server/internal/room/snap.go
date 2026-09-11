package room

import "math"

func SnapAxis(cell, offset, footprint int, mode Snap, v int) int {
	if mode == SnapOff || cell < 1 {
		return v
	}
	c := float64(cell)
	o := float64(offset)
	step := c
	half := 0.0
	if mode == SnapHalfCells {
		step = c / 2
	} else if centred(footprint) {
		half = c / 2
	}
	k := math.Round((float64(v) - o - half) / step)
	return int(math.Round(k*step + o + half))
}
func SnapPoint(g Grid, footprintW, footprintH, x, y int) (int, int) {
	return SnapAxis(g.CellSize, g.OffsetX, footprintW, g.Snap, x),
		SnapAxis(g.CellSize, g.OffsetY, footprintH, g.Snap, y)
}
func centred(footprint int) bool {
	return footprint%2 == 1
}
func snapPawn(g Grid, p Pawn, x, y int) (int, int) {
	if p.Kind == PawnObject {
		return x, y
	}
	f := p.Size.Footprint()
	return SnapPoint(g, f, f, x, y)
}
