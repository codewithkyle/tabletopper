package room

import "math"

// SNAPPING, which is one function applied twice.
//
// The 5e rule the table already plays by is that a medium creature stands in a
// cell and a large one straddles four, so the two cases are not "snap to cells"
// and "snap to corners" -- they are one rule read against the creature's size.
// A footprint of odd width has a middle cell, so its centre belongs at a cell
// centre; a footprint of even width has no middle cell, so its centre belongs
// on the vertex where four cells meet. The GM's "corners" mode is the same
// function with the parity inverted, for the tables that place everything on
// intersections.
//
// IT IS PER AXIS, which is what makes an object work. A two by four wagon is
// even on both axes and lands on vertices; a three by four one is odd across
// and even down, so its centre sits at a cell centre horizontally and on a
// vertex vertically. Nothing special is written for objects: they pass a
// different pair of numbers into the same call.
//
// THE SERVER SNAPS TOO, not just the client. The stored position is the
// server's answer, so a client that does not snap, or snaps differently,
// converges on the same pixel as everyone else the moment its echo comes back.

// SnapAxis moves one coordinate to the nearest legal position on one axis.
// cell is the grid's cell size, offset that axis's grid offset, footprint the
// pawn's width in cells along this axis, and v the raw coordinate.
//
// It is exported because the TypeScript renderer is a port of it and the test
// that pins the four parity-and-mode combinations is the specification that
// port is written against.
func SnapAxis(cell, offset, footprint int, mode Snap, v int) int {
	if mode == SnapOff || cell < 1 {
		return v
	}

	// half is the whole difference between the two cases: a centred footprint
	// measures from the middle of a cell, a straddling one from its edge.
	half := 0.0
	if centred(footprint, mode) {
		half = float64(cell) / 2
	}

	c := float64(cell)
	o := float64(offset)

	// math.Round is half away from zero, which is what keeps a point exactly
	// between two cells from drifting toward the origin on one side of the map
	// and away from it on the other.
	k := math.Round((float64(v) - o - half) / c)

	return int(math.Round(k*c + o + half))
}

// SnapPoint snaps a pawn's centre, each axis by its own footprint.
func SnapPoint(g Grid, footprintW, footprintH, x, y int) (int, int) {
	return SnapAxis(g.CellSize, g.OffsetX, footprintW, g.Snap, x),
		SnapAxis(g.CellSize, g.OffsetY, footprintH, g.Snap, y)
}

// centred answers which of the two positions this footprint takes under this
// mode. Odd footprints centre on cells and even ones on vertices; corners mode
// swaps the two, which is exactly what the equality below says.
func centred(footprint int, mode Snap) bool {
	odd := footprint%2 == 1

	return odd == (mode == SnapCells)
}

// snapPawn is what the commands call: it reads the footprint off the pawn so
// that no caller has to remember which axis takes which number.
func snapPawn(g Grid, p Pawn, x, y int) (int, int) {
	w, h := p.Footprint()

	return SnapPoint(g, w, h, x, y)
}
