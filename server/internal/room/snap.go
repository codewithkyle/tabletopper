package room

import "math"

// SNAPPING, which is one lattice and a choice of how fine it is.
//
// The rule the table already plays by is that a medium creature stands in a
// cell and a large one straddles four. That is not two modes, it is one rule
// read against the creature's size: a footprint of odd width has a middle cell,
// so its centre belongs at a cell centre, and a footprint of even width has no
// middle cell, so its centre belongs on the vertex where four cells meet. Cells
// mode is that rule, and it is the only thing the parity term below is for.
//
// Half-cells mode drops the question instead of answering it. Stepping by half
// a cell reaches every cell centre AND every vertex -- the odd multiples are the
// centres and the even ones are the vertices -- so the nearest legal position
// is simply the nearest of the two and the footprint stops mattering. It is the
// mode for a table that wants a creature between squares as readily as in one.
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
// that pins the modes against both parities is the specification that port is
// written against.
func SnapAxis(cell, offset, footprint int, mode Snap, v int) int {
	if mode == SnapOff || cell < 1 {
		return v
	}

	c := float64(cell)
	o := float64(offset)

	// step is the lattice spacing and half is where that lattice starts
	// relative to a grid line. Half-cells needs no phase at all: a step of
	// c/2 from the line already lands on the centres and the vertices alike.
	step := c
	half := 0.0
	if mode == SnapHalfCells {
		step = c / 2
	} else if centred(footprint) {
		half = c / 2
	}

	// math.Round is half away from zero, which is what keeps a point exactly
	// between two cells from drifting toward the origin on one side of the map
	// and away from it on the other.
	k := math.Round((float64(v) - o - half) / step)

	return int(math.Round(k*step + o + half))
}

// SnapPoint snaps a pawn's centre, each axis by its own footprint.
func SnapPoint(g Grid, footprintW, footprintH, x, y int) (int, int) {
	return SnapAxis(g.CellSize, g.OffsetX, footprintW, g.Snap, x),
		SnapAxis(g.CellSize, g.OffsetY, footprintH, g.Snap, y)
}

// centred answers whether this footprint sits on a cell centre or on a vertex
// under whole-cell snapping. Odd footprints have a middle cell to stand in;
// even ones do not.
func centred(footprint int) bool {
	return footprint%2 == 1
}

// snapPawn is what the commands call: it reads the footprint off the pawn so
// that no caller has to remember which axis takes which number.
func snapPawn(g Grid, p Pawn, x, y int) (int, int) {
	w, h := p.Footprint(g.CellSize)

	return SnapPoint(g, w, h, x, y)
}
