// THE RULER: where a dragged pawn lands, which cells it passes through, and how
// far that is in feet.
//
// EVERY FUNCTION HERE IS PURE AND EVERY ONE OF THEM HAS A COUNTERPART ON THE
// SERVER OR IN THE RULES. snapPoint is a port of internal/room/snap.go, line
// for line, and it has to stay one: the server snaps every committed move
// itself, so a client that snapped differently would show a pawn under the
// pointer and then watch it jump when its own echo came back. The Go tests are
// the specification and path.test.ts reproduces their numbers.
//
// THE PATH IS COUNTED IN CELLS AND NOT MEASURED IN PIXELS. Two squares diagonally
// is ten feet under 5e's default and fifteen under the 5-10-5 variant, and
// neither is the 14.1 feet a ruler would report. What the table plays by is a
// count of squares, so that is what this counts.

import type { Grid, Pawn } from "../protocol.ts";

// PATH_CELLS_MAX bounds one path. A drag across a 12000 pixel map at 64 pixel
// cells is under two hundred cells; this is generous enough never to be reached
// by a hand and small enough that a client which somehow asked for a path from
// one corner of the coordinate space to the other allocates a bounded array
// rather than fifteen thousand of them.
export const PATH_CELLS_MAX = 512;

// round is HALF AWAY FROM ZERO, which JavaScript's Math.round is not.
//
// THIS IS THE ONE LINE THAT WOULD SILENTLY DESYNC THE PORT. Go's math.Round
// takes -0.5 to -1; Math.round takes it to -0, because it rounds half toward
// positive infinity. Every snap on the negative side of the grid origin would
// land one lattice step away from where the server put it, on a map whose
// origin the GM had offset -- which is a pawn that jumps when its echo arrives,
// on some parts of the table and not others.
function round(value: number): number {
	return value < 0 ? -Math.round(-value) : Math.round(value);
}

// snapAxis moves one coordinate to the nearest legal position on one axis. It
// is internal/room.SnapAxis.
//
// ONE LATTICE AND A CHOICE OF HOW FINE. Cells mode answers the parity question
// -- an odd footprint has a middle cell so its centre belongs at a cell centre,
// an even one has none so its centre belongs on a vertex -- and half-cells mode
// drops the question, because stepping by half a cell reaches every centre AND
// every vertex and the nearer one simply wins.
export function snapAxis(cell: number, offset: number, footprint: number, mode: Grid["snap"], value: number): number {
	if (mode === "off" || cell < 1) {
		return value;
	}

	let step = cell;
	let half = 0;

	if (mode === "halfCells") {
		step = cell / 2;
	} else if (footprint % 2 === 1) {
		half = cell / 2;
	}

	const k = round((value - offset - half) / step);

	return round(k * step + offset + half);
}

// snapPoint snaps a centre, each axis by its own footprint. A two by three
// wagon is even across and odd down, so it straddles horizontally and centres
// vertically -- which is the whole reason snapping is per axis.
export function snapPoint(grid: Grid, footprintW: number, footprintH: number, x: number, y: number): [number, number] {
	return [
		snapAxis(grid.cellSize, grid.offsetX, footprintW, grid.snap, x),
		snapAxis(grid.cellSize, grid.offsetY, footprintH, grid.snap, y),
	];
}

// Sized is the part of a pawn that answers "how big is it": a creature by its
// size category, an object by the pixel size of the picture on it.
export type Sized = Pick<Pawn, "kind" | "size" | "width" | "height">;

// TINY_SCALE is how much of its cell a tiny creature is drawn at. It OCCUPIES
// a whole cell, because half a cell is not a position any grid rule can
// express, and a rat drawn the size of an ogre is not a rat.
export const TINY_SCALE = 0.5;

// footprintOf is how many CELLS a pawn stands on, per axis. It is
// Pawn.Footprint in Go: a creature reads its size category, and an object
// divides the picture's pixels by the cell size.
//
// IT IS THE SNAPPING LATTICE AND NOT THE DRAWN SIZE, which is the whole reason
// it and pawnExtents are two functions. An object is DRAWN at the picture's own
// pixels -- that is what makes a token look like the thing it is a picture of
// -- and there is no such thing as half a cell of snapping parity, so the
// lattice it lands on is the nearest whole number of cells to that.
//
// THE ROUNDING IS GO'S INTEGER ARITHMETIC AND NOT Math.round, for the reason
// the note at the top of this file gives about snapPoint: the server does this
// sum too, and the two must not disagree. (w + cell/2) / cell with both
// divisions truncated is not Math.round(w / cell) once the cell size is odd.
//
// AN UNKNOWN SIZE IS ONE CELL rather than zero, because a zero footprint would
// divide by nothing in the snapper and would make a pawn with a corrupt size
// unplaceable rather than merely medium.
export function footprintOf(pawn: Sized, cellSize: number): [number, number] {
	if (pawn.kind === "object") {
		const cell = Math.max(1, cellSize);
		const half = Math.trunc(cell / 2);

		return [
			Math.max(1, Math.trunc((pawn.width + half) / cell)),
			Math.max(1, Math.trunc((pawn.height + half) / cell)),
		];
	}

	switch (pawn.size) {
		case "large":
			return [2, 2];
		case "huge":
			return [3, 3];
		case "gargantuan":
			return [4, 4];
		default:
			return [1, 1];
	}
}

// pawnExtents is how much floor a pawn covers, in map pixels, as half extents.
// It is what the renderer sizes a quad with, what the hit test measures against
// and what the selection ring is drawn round.
//
// AN OBJECT IS ITS PICTURE AND A CREATURE IS ITS CELLS. A wagon is as wide as
// the wagon in the file, whatever the grid is set to; a goblin is one cell and
// an ogre is two, whatever picture is on them.
export function pawnExtents(pawn: Sized, cellSize: number): [number, number] {
	if (pawn.kind === "object") {
		return [Math.max(1, pawn.width) / 2, Math.max(1, pawn.height) / 2];
	}

	const cell = Math.max(1, cellSize);
	const [w, h] = footprintOf(pawn, cell);
	const scale = pawn.size === "tiny" ? TINY_SCALE : 1;

	return [(w * cell * scale) / 2, (h * cell * scale) / 2];
}

// snapPawn is what a drag calls: it reads the footprint off the pawn so no
// caller has to remember which axis takes which number.
export function snapPawn(grid: Grid, pawn: Sized, x: number, y: number): [number, number] {
	const [w, h] = footprintOf(pawn, grid.cellSize);

	return snapPoint(grid, w, h, x, y);
}

// cellAt is which cell a map point falls in, as integer cell coordinates. The
// grid's own offset is subtracted first, so a GM who nudged the grid gets cells
// that line up with what they can see.
export function cellAt(grid: Grid, x: number, y: number): [number, number] {
	const cell = Math.max(1, grid.cellSize);

	return [Math.floor((x - grid.offsetX) / cell), Math.floor((y - grid.offsetY) / cell)];
}

// cellCentre is the inverse: the map point at the middle of a cell, which is
// where a highlight is drawn from.
export function cellCentre(grid: Grid, cx: number, cy: number): [number, number] {
	const cell = Math.max(1, grid.cellSize);

	return [grid.offsetX + (cx + 0.5) * cell, grid.offsetY + (cy + 0.5) * cell];
}

// supercover is the cells a straight line from one cell's centre to another's
// passes through, the first and the last included.
//
// IT IS A VOXEL WALK AND NOT A BRESENHAM LINE. Bresenham picks ONE cell per
// column and skips the one a line clips the corner of, which is a path that
// misses squares a creature would actually have to walk through. This crosses
// every boundary in the order the line crosses it, so a knight's move comes out
// as the staircase it is.
//
// AN EXACT CORNER TAKES THE DIAGONAL STEP AND NOTHING ELSE, which is the one
// place the classic supercover disagrees with what a table wants. A line at
// exactly forty-five degrees passes through the corner where four cells meet;
// the textbook answer adds all four, and what a GM wants to see for a diagonal
// move is a diagonal of single cells. So the tie steps diagonally.
export function supercover(x0: number, y0: number, x1: number, y1: number, out: number[]): number[] {
	out.length = 0;
	out.push(x0, y0);

	if (x0 === x1 && y0 === y1) {
		return out;
	}

	const dx = x1 - x0;
	const dy = y1 - y0;
	const stepX = Math.sign(dx);
	const stepY = Math.sign(dy);

	// The walk runs in cell units from the centre of the first cell, so the
	// first boundary on each axis is half a cell away and every one after it is
	// a whole cell. An axis with no movement never crosses a boundary at all,
	// which Infinity says exactly.
	const deltaX = dx === 0 ? Infinity : Math.abs(1 / dx);
	const deltaY = dy === 0 ? Infinity : Math.abs(1 / dy);

	let tX = dx === 0 ? Infinity : deltaX / 2;
	let tY = dy === 0 ? Infinity : deltaY / 2;

	let x = x0;
	let y = y0;

	// The epsilon is what makes the diagonal case exact. tX and tY are built
	// from the same arithmetic when |dx| equals |dy|, so they are bit-identical
	// there -- but a comparison that had to be exact would be a comparison
	// waiting to be broken by a refactor, and a tolerance this far below a cell
	// cannot change any other answer.
	const epsilon = 1e-9;

	while ((x !== x1 || y !== y1) && out.length < PATH_CELLS_MAX * 2) {
		if (tX < tY - epsilon) {
			x += stepX;
			tX += deltaX;
		} else if (tY < tX - epsilon) {
			y += stepY;
			tY += deltaY;
		} else {
			x += stepX;
			y += stepY;
			tX += deltaX;
			tY += deltaY;
		}

		out.push(x, y);
	}

	return out;
}

// cellsMoved is the distance in CELLS between two cells under the table's
// diagonal rule.
//
// equal: the Chebyshev distance, which is 5e's default -- a diagonal costs the
// same as a straight step, so three across and two down is three squares.
//
// alternating: the 5-10-5 variant, where the first diagonal costs one square,
// the second two, the third one, and so on. n diagonals therefore cost
// n + floor(n / 2), which gives 1, 3, 4, 6 for one through four -- five, fifteen,
// twenty and thirty feet at a five foot cell.
export function cellsMoved(dx: number, dy: number, diagonals: Grid["diagonals"]): number {
	const across = Math.abs(dx);
	const down = Math.abs(dy);

	const straight = Math.max(across, down) - Math.min(across, down);
	const diagonal = Math.min(across, down);

	if (diagonals === "alternating") {
		return straight + diagonal + Math.floor(diagonal / 2);
	}

	return straight + diagonal;
}

// feetMoved is that count in the table's own units.
export function feetMoved(dx: number, dy: number, grid: Grid): number {
	return cellsMoved(dx, dy, grid.diagonals) * Math.max(0, grid.feetPerCell);
}

// distanceLabel is the string the canvas draws under a drag. It is built here
// rather than in the pass because the glyph atlas holds exactly the characters
// this can produce -- the ten digits, a space, an f, a t and a full stop -- and
// a label with a character the atlas has no quad for would render a hole.
export function distanceLabel(feet: number): string {
	return `${Math.round(feet)} ft.`;
}
