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

import type { Grid, Pawn, Size } from "../protocol.ts";
import type { Point } from "./camera.ts";

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

// snapPoint snaps a centre, each axis by its own footprint. The axes are
// genuinely independent even though nothing passes two different numbers today:
// a creature's square is the same across and down, and an object does not snap
// at all.
export function snapPoint(grid: Grid, footprintW: number, footprintH: number, x: number, y: number): [number, number] {
	return [
		snapAxis(grid.cellSize, grid.offsetX, footprintW, grid.snap, x),
		snapAxis(grid.cellSize, grid.offsetY, footprintH, grid.snap, y),
	];
}

// Sized is the part of a pawn that answers "how big is it, and which way round":
// a creature by its size category, an object by the pixel size of the picture on
// it and the angle it has been turned to.
export type Sized = Pick<Pawn, "kind" | "size" | "width" | "height" | "rotation">;

// Placed is that plus where it is, which is what a hit test needs.
export type Placed = Sized & Pick<Pawn, "x" | "y">;

// TINY_SCALE is how much of its cell a tiny creature is drawn at. It OCCUPIES
// a whole cell, because half a cell is not a position any grid rule can
// express, and a rat drawn the size of an ogre is not a rat.
export const TINY_SCALE = 0.5;

// footprintOf is how many CELLS a creature stands on, on a side. It is Go's
// Size.Footprint, and it is the parity snapping needs: an odd footprint has a
// middle cell to stand in, an even one straddles the vertex where four meet.
//
// AN OBJECT HAS NO FOOTPRINT AND IS NOT ASKED FOR ONE. It is not on the lattice
// -- see snapPawn -- so "how many cells is this wagon" has no caller left. What
// an object is, is width by height pixels at an angle.
//
// AN UNKNOWN SIZE IS ONE CELL rather than zero, because a zero footprint would
// divide by nothing in the snapper and would make a pawn with a corrupt size
// unplaceable rather than merely medium.
export function footprintOf(size: Size): number {
	switch (size) {
		case "large":
			return 2;
		case "huge":
			return 3;
		case "gargantuan":
			return 4;
		default:
			return 1;
	}
}

// pawnExtents is how much floor a pawn covers, in map pixels, as half extents
// IN ITS OWN FRAME. It is what the renderer sizes a quad with, what the hit test
// measures against and what the selection outline is drawn round.
//
// AN OBJECT IS ITS PICTURE AND A CREATURE IS ITS CELLS. A wagon is as wide as
// the wagon in the file, whatever the grid is set to; a goblin is one cell and
// an ogre is two, whatever picture is on them.
//
// THE ROTATION IS NOT IN HERE. These are the half extents of the unrotated
// rectangle, and every consumer applies the angle itself -- the shader by
// turning the quad, the hit test by turning the POINT the other way. Baking a
// rotated bounding box in would give the renderer a quad that grew as it
// spun.
export function pawnExtents(pawn: Sized, cellSize: number): [number, number] {
	if (pawn.kind === "object") {
		return [Math.max(1, pawn.width) / 2, Math.max(1, pawn.height) / 2];
	}

	const cell = Math.max(1, cellSize);
	const f = footprintOf(pawn.size);
	const scale = pawn.size === "tiny" ? TINY_SCALE : 1;

	return [(f * cell * scale) / 2, (f * cell * scale) / 2];
}

// boundsOf is the SCREEN-ALIGNED box a pawn occupies, as half extents: the
// rotated rectangle's own extents projected back onto the map's axes.
//
// IT IS NOT WHAT THE QUAD IS DRAWN AT, which is pawnExtents. This is the box
// something square-on has to be placed against -- the DOM overlay that floats
// above a selected pawn -- and for a long token turned on its side the two are
// each other's opposite. Baking it into pawnExtents instead would give the
// renderer a quad that grew as it spun.
export function boundsOf(pawn: Sized, cellSize: number): [number, number] {
	const [halfW, halfH] = pawnExtents(pawn, cellSize);
	if (pawn.rotation === 0 || pawn.kind !== "object") {
		return [halfW, halfH];
	}

	const a = radians(pawn.rotation);
	const cos = Math.abs(Math.cos(a));
	const sin = Math.abs(Math.sin(a));

	return [halfW * cos + halfH * sin, halfW * sin + halfH * cos];
}

// radians turns the wire's whole degrees into what the trigonometry wants. The
// protocol carries degrees because they are exact over JSON and are what a
// person types into a field; nothing below this line is in them.
export function radians(degrees: number): number {
	return (degrees * Math.PI) / 180;
}

// unrotate takes an offset from a pawn's centre into the PAWN'S own frame, and
// spin puts one back out into the map's.
//
// THEY WRITE INTO A CALLER'S OBJECT rather than returning a pair, because the
// hit test calls unrotate once per pawn on every pointer move and a tuple per
// pawn per move is the allocation the second performance rule is about.
//
// ZERO IS A FAST PATH AND ALSO AN EXACT ONE. Almost every pawn on a table is at
// zero, and Math.cos(0) is 1 but Math.sin(radians(180)) is 1.2e-16 -- so a
// short-circuit here keeps an unturned pawn's arithmetic to the bit rather than
// to a rounding error's worth of it.
export function unrotate(degrees: number, dx: number, dy: number, out: Point): Point {
	if (degrees === 0) {
		out.x = dx;
		out.y = dy;

		return out;
	}

	const a = radians(degrees);
	const cos = Math.cos(a);
	const sin = Math.sin(a);

	out.x = dx * cos + dy * sin;
	out.y = dy * cos - dx * sin;

	return out;
}

export function spin(degrees: number, lx: number, ly: number, out: Point): Point {
	if (degrees === 0) {
		out.x = lx;
		out.y = ly;

		return out;
	}

	const a = radians(degrees);
	const cos = Math.cos(a);
	const sin = Math.sin(a);

	out.x = lx * cos - ly * sin;
	out.y = lx * sin + ly * cos;

	return out;
}

// local is this module's scratch, so containsPoint allocates nothing.
const local: Point = { x: 0, y: 0 };

// containsPoint is whether a map point is on a pawn, and it is the one place
// the shapes are decided.
//
// A DISC FOR A CREATURE AND A RECTANGLE FOR AN OBJECT, matching exactly what the
// pawn pass draws: a click on the corner of a wagon's box hits the wagon, and a
// click on the corner of a goblin's does not hit the goblin.
//
// THE POINT IS TURNED AND THE PAWN IS NOT. A rotated rectangle is an axis-
// aligned one seen from an angle, so the cheap and exact test is to take the
// pointer into the pawn's frame and compare against the half extents there.
// Testing a rotated box in the map's frame would need four edges and would give
// a different answer at the corners.
export function containsPoint(pawn: Placed, x: number, y: number, cellSize: number): boolean {
	const [halfW, halfH] = pawnExtents(pawn, cellSize);

	if (pawn.kind !== "object") {
		const dx = x - pawn.x;
		const dy = y - pawn.y;

		return dx * dx + dy * dy <= halfW * halfW;
	}

	unrotate(pawn.rotation, x - pawn.x, y - pawn.y, local);

	return Math.abs(local.x) <= halfW && Math.abs(local.y) <= halfH;
}

// snapsToGrid is whether a pawn's position is quantised at all: the grid has to
// be snapping AND the pawn has to be the kind that answers to a lattice.
//
// IT IS ASKED BY THE DRAG'S CLOCK as well as by the snapper. With snapping on,
// the cell under the pointer is what decides when to report a drag -- the hot
// path quantises itself -- and a pawn that moves freely never crosses a
// boundary, so it needs the timer instead.
export function snapsToGrid(pawn: Sized, grid: Grid): boolean {
	return grid.snap !== "off" && pawn.kind !== "object";
}

// snapPawn is what a drag calls: it reads the footprint off the pawn so no
// caller has to remember which axis takes which number.
//
// AN OBJECT IS NEVER SNAPPED, which is internal/room.snapPawn returning early
// for the same reason. A picture laid on a floor is not a creature standing in
// a square: a rug, a door, a bloodstain and a road sign are placed against what
// the cartographer drew rather than against the lattice laid over it, and a
// table that could not be nudged the last twenty pixels into its doorway is a
// table that cannot be dressed.
export function snapPawn(grid: Grid, pawn: Sized, x: number, y: number): [number, number] {
	if (pawn.kind === "object") {
		return [x, y];
	}

	const f = footprintOf(pawn.size);

	return snapPoint(grid, f, f, x, y);
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

// feetBetween is the straight-line distance between two map points in the
// table's own units, with the lattice ignored entirely.
//
// IT IS THE ONE MEASUREMENT ON THIS TABLE THAT DOES NOT COUNT SQUARES, and it
// exists for the ruler in the pill rather than for a move. A move is a creature
// walking through cells and is scored by the rule the table plays under -- that
// is cellsMoved, and it is why two squares diagonally can be ten feet or
// fifteen. A GM asking how far the dragon's breath reaches is asking about a
// line on a map, and a line does not care which squares it crosses.
//
// SO IT IS PYTHAGORAS AND NOTHING ELSE. The cell size is what turns pixels into
// feet, and the diagonal rule has no say: a hypotenuse that came back as three
// squares would be the grid answering a question nobody asked it.
export function feetBetween(dx: number, dy: number, grid: Grid): number {
	const cell = Math.max(1, grid.cellSize);

	return (Math.hypot(dx, dy) / cell) * Math.max(0, grid.feetPerCell);
}

// distanceLabel is the string the canvas draws under a drag. It is built here
// rather than in the pass because the glyph atlas holds exactly the characters
// this can produce -- the ten digits, a space, an f, a t and a full stop -- and
// a label with a character the atlas has no quad for would render a hole.
export function distanceLabel(feet: number): string {
	return `${Math.round(feet)} ft.`;
}
