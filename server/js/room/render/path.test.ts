// The ruler, checked against the rules it implements and against the Go it is a
// port of.
//
// THE SNAPPING NUMBERS ARE internal/room/snap_test.go's OWN. The server snaps
// every committed move, so a port that disagreed would show a pawn under the
// pointer and then jump it when the echo arrived -- and it would only do so on
// some parts of the table, which is the kind of bug that gets reported as "it
// sometimes moves wrong".

import assert from "node:assert/strict";
import { test } from "node:test";

import type { Grid } from "../protocol.ts";
import {
	boundsOf,
	cellAt,
	cellCentre,
	cellsMoved,
	distanceLabel,
	feetMoved,
	footprintOf,
	pawnExtents,
	snapAxis,
	snapPawn,
	snapPoint,
	snapsToGrid,
	supercover,
} from "./path.ts";

function grid(over: Partial<Grid> = {}): Grid {
	return {
		lines: "solid",
		cellSize: 64,
		offsetX: 0,
		offsetY: 0,
		color: "#000000FF",
		snap: "cells",
		feetPerCell: 5,
		diagonals: "equal",
		...over,
	};
}

// EVERY PARITY AGAINST EVERY MODE, which is the whole of the snapping rule.
// With a zero offset the cell centres are at 32, 96, 160 and the vertices at 0,
// 64, 128. A raw 100 is nearer 96 than 160, and nearer 128 than 64.
test("snapAxis takes every parity and mode", () => {
	const cases: [string, number, Grid["snap"], number, number][] = [
		["odd footprint under cells lands on a centre", 1, "cells", 100, 96],
		["even footprint under cells lands on a vertex", 2, "cells", 100, 128],

		// 100 is 4 from the centre at 96 and 28 from the vertex at 128, so
		// half-cells takes the centre for both parities.
		["odd footprint under half-cells takes whichever is nearer", 1, "halfCells", 100, 96],
		["even footprint under half-cells takes the same one", 2, "halfCells", 100, 96],

		// And 120 is nearer the vertex at 128, so the same mode answers a
		// vertex without being told to.
		["half-cells reaches a vertex when the vertex is nearer", 1, "halfCells", 120, 128],
		["half-cells reaches a vertex for an even footprint too", 2, "halfCells", 120, 128],

		["a gargantuan creature is even and straddles", 4, "cells", 100, 128],
		["a huge creature is odd and centres", 3, "cells", 100, 96],

		["off leaves the coordinate alone", 1, "off", 100, 100],
		["off leaves an even footprint alone too", 2, "off", 100, 100],
	];

	for (const [name, footprint, mode, input, want] of cases) {
		assert.equal(snapAxis(64, 0, footprint, mode, input), want, name);
	}
});

// A point exactly between two legal positions has to go somewhere, and the
// somewhere has to be the same as the server's. Go's math.Round is half AWAY
// FROM ZERO and JavaScript's Math.round is half toward positive infinity, so
// this is the assertion that catches the port having used the wrong one: 64 is
// the midpoint between the centres at 32 and 96 and goes up, and 0 is the
// midpoint between -32 and 32 and goes DOWN.
test("snapAxis breaks ties away from zero, as Go does", () => {
	assert.equal(snapAxis(64, 0, 1, "cells", 64), 96);
	assert.equal(snapAxis(64, 0, 1, "cells", 0), -32);
});

// The half-cell lattice is exactly the centres and the vertices, alternating.
// A mode that stepped by a whole cell from the vertex would reach half of these
// and a GM would report that corners "sometimes" work.
test("half-cells reaches every centre and every vertex", () => {
	const seen: number[] = [];
	for (let v = 0; v <= 128; v++) {
		const got = snapAxis(64, 0, 1, "halfCells", v);
		if (seen.length === 0 || seen[seen.length - 1] !== got) {
			seen.push(got);
		}
	}

	assert.deepEqual(seen, [0, 32, 64, 96, 128]);
});

// A negative offset is an ordinary offset: a GM who nudged the grid left by a
// quarter cell gets a grid a quarter cell left, not one that stopped at zero.
test("snapAxis takes a negative offset", () => {
	assert.equal(snapAxis(64, -16, 1, "cells", 0), 16);
	assert.equal(snapAxis(64, -16, 2, "cells", 0), -16);
});

// The case objects exist for. A two by three wagon is even across and odd down,
// so it straddles horizontally and centres vertically -- one call, two answers.
test("snapPoint takes each axis separately", () => {
	assert.deepEqual(snapPoint(grid(), 2, 3, 100, 100), [128, 96]);
});

test("snapPawn reads the footprint off the pawn", () => {
	const large = { kind: "monster" as const, size: "large" as const, width: 0, height: 0, rotation: 0 };
	assert.deepEqual(snapPawn(grid(), large, 100, 100), [128, 128]);

	const medium = { kind: "monster" as const, size: "medium" as const, width: 0, height: 0, rotation: 0 };
	assert.deepEqual(snapPawn(grid(), medium, 100, 100), [96, 96]);
});

// AN OBJECT IS NOT ON THE LATTICE AT ALL, which is internal/room.snapPawn
// returning early: a picture laid on a floor is placed against what the
// cartographer drew rather than against the grid over it.
test("an object keeps the pixel it was given, whatever the grid says", () => {
	const wagon = { kind: "object" as const, size: "medium" as const, width: 128, height: 192, rotation: 0 };

	assert.deepEqual(snapPawn(grid(), wagon, 100, 100), [100, 100]);
	assert.deepEqual(snapPawn(grid(), wagon, 7, -3), [7, -3]);

	// And the drag's clock follows the same rule: a token that never crosses a
	// cell boundary has to report itself on a timer instead.
	assert.equal(snapsToGrid(wagon, grid()), false);
	assert.equal(snapsToGrid({ ...wagon, kind: "monster" }, grid()), true);
	assert.equal(snapsToGrid({ ...wagon, kind: "monster" }, { ...grid(), snap: "off" }), false);
});

// A cell size of zero would divide by nothing. It cannot arrive through
// table.setGrid, which validates, but this is called from wherever the client
// feels like calling it.
test("an impossible grid leaves the coordinate alone", () => {
	assert.equal(snapAxis(0, 0, 1, "cells", 100), 100);
});

test("a creature's footprint is its size category", () => {
	assert.equal(footprintOf("tiny"), 1);
	assert.equal(footprintOf("medium"), 1);
	assert.equal(footprintOf("large"), 2);
	assert.equal(footprintOf("huge"), 3);
	assert.equal(footprintOf("gargantuan"), 4);

	// A size the client has never heard of is one cell rather than none: a zero
	// footprint divides by nothing in the snapper.
	assert.equal(footprintOf("colossal" as never), 1);
});

// pawnExtents is the DRAWN size, and it is where a creature and an object part
// company: a goblin is its cell whatever picture is on it, and a wagon is its
// picture whatever the grid is set to.
test("an object is drawn at its picture's size and a creature at its cell's", () => {
	assert.deepEqual(
		pawnExtents({ kind: "object", size: "medium", width: 100, height: 40, rotation: 0 }, 64),
		[50, 20],
	);

	assert.deepEqual(
		pawnExtents({ kind: "monster", size: "large", width: 0, height: 0, rotation: 0 }, 64),
		[64, 64],
	);

	// A tiny creature OCCUPIES a whole cell and is drawn at half of one.
	assert.deepEqual(
		pawnExtents({ kind: "monster", size: "tiny", width: 0, height: 0, rotation: 0 }, 64),
		[16, 16],
	);

	// An object with no picture size is not drawn at nothing.
	assert.deepEqual(
		pawnExtents({ kind: "object", size: "medium", width: 0, height: 0, rotation: 0 }, 64),
		[0.5, 0.5],
	);
});

// THE PATH IS EVERY CELL THE LINE PASSES THROUGH, which is what a creature
// would actually have to walk. Bresenham picks one cell per column and skips
// the one a line clips the corner of.
test("a horizontal path is the row of cells between the two", () => {
	const cells: number[] = [];
	supercover(0, 0, 3, 0, cells);

	assert.deepEqual(cells, [0, 0, 1, 0, 2, 0, 3, 0]);
});

// THE ONE PLACE THE TEXTBOOK SUPERCOVER IS WRONG FOR A TABLE. A line at exactly
// forty-five degrees passes through the corner where four cells meet, and the
// classic algorithm adds all four; a GM dragging diagonally wants a diagonal of
// single cells.
test("an exact diagonal is a diagonal of single cells", () => {
	const cells: number[] = [];
	supercover(0, 0, 3, 3, cells);

	assert.deepEqual(cells, [0, 0, 1, 1, 2, 2, 3, 3]);
});

test("a diagonal walked backwards is the same cells in reverse", () => {
	const cells: number[] = [];
	supercover(2, 5, -1, 2, cells);

	assert.deepEqual(cells, [2, 5, 1, 4, 0, 3, -1, 2]);
});

// A knight's move is the staircase it looks like: two across and one down
// crosses one horizontal boundary between two vertical ones.
test("a knight's move is a staircase", () => {
	const cells: number[] = [];
	supercover(0, 0, 2, 1, cells);

	assert.deepEqual(cells, [0, 0, 1, 0, 1, 1, 2, 1]);
});

test("a path to the cell it started in is that one cell", () => {
	const cells: number[] = [];
	supercover(4, 7, 4, 7, cells);

	assert.deepEqual(cells, [4, 7]);
});

// Every cell in a path is adjacent to the one before it, which is the property
// that makes it a path rather than a list of squares. Without it a highlight
// would have gaps in it that a creature could not have stepped over.
test("every step of a path is one cell from the last", () => {
	const cells: number[] = [];

	for (const [x, y] of [[7, 3], [-5, 9], [1, -12], [13, 13], [0, 6]]) {
		supercover(0, 0, x, y, cells);

		for (let i = 2; i < cells.length; i += 2) {
			const dx = Math.abs(cells[i] - cells[i - 2]);
			const dy = Math.abs(cells[i + 1] - cells[i - 1]);

			assert.ok(dx <= 1 && dy <= 1 && dx + dy > 0, `step ${i / 2} to (${x}, ${y}) jumped`);
		}

		assert.deepEqual([cells[cells.length - 2], cells[cells.length - 1]], [x, y]);
	}
});

// 5e's default: a diagonal costs the same as a straight step, so three across
// and two down is three squares and fifteen feet.
test("equal diagonals count the Chebyshev distance", () => {
	assert.equal(cellsMoved(3, 2, "equal"), 3);
	assert.equal(cellsMoved(0, 4, "equal"), 4);
	assert.equal(cellsMoved(-3, 3, "equal"), 3);
	assert.equal(feetMoved(3, 2, grid()), 15);
});

// THE 5-10-5 SEQUENCE, which is the whole reason the alternative exists: the
// first diagonal costs one square, the second two, the third one, the fourth
// two. One through four diagonals are therefore five, fifteen, twenty and
// thirty feet.
test("alternating diagonals run 5, 15, 20, 30", () => {
	const feet = [1, 2, 3, 4].map((n) => feetMoved(n, n, grid({ diagonals: "alternating" })));

	assert.deepEqual(feet, [5, 15, 20, 30]);
});

test("alternating charges the straights at face value", () => {
	// Four across and one down is one diagonal and three straights: four
	// squares, twenty feet, the same as under the default.
	assert.equal(cellsMoved(4, 1, "alternating"), 4);
	assert.equal(cellsMoved(4, 1, "equal"), 4);
});

test("a table with ten foot cells doubles every answer", () => {
	assert.equal(feetMoved(3, 3, grid({ feetPerCell: 10 })), 30);
});

test("cells and their centres are inverses under an offset", () => {
	const g = grid({ offsetX: -16, offsetY: 24 });

	for (const [cx, cy] of [[0, 0], [3, -2], [-7, 5]]) {
		const [x, y] = cellCentre(g, cx, cy);
		assert.deepEqual(cellAt(g, x, y), [cx, cy]);
	}
});

// The atlas holds the ten digits, a space, an f, a t and a full stop, and
// nothing else. A label with a character it has no quad for renders a hole.
test("a distance label uses only what the glyph atlas holds", () => {
	for (const feet of [0, 5, 15, 120, 1005]) {
		assert.match(distanceLabel(feet), /^[0-9]+ ft\.$/);
	}

	assert.equal(distanceLabel(14.6), "15 ft.");
});

// boundsOf is the SCREEN-ALIGNED box, which is what something square-on has to
// be placed against -- the DOM overlay that floats above a selected pawn. For a
// long token turned on its side it is pawnExtents the other way round.
test("a turned token's screen box is not the box it is drawn in", () => {
	const beam = { kind: "object" as const, size: "medium" as const, width: 200, height: 40, rotation: 0 };

	assert.deepEqual(boundsOf(beam, 64), [100, 20]);
	assert.deepEqual(pawnExtents({ ...beam, rotation: 90 }, 64), [100, 20], "the quad grew as it spun");

	const [halfW, halfH] = boundsOf({ ...beam, rotation: 90 }, 64);
	assert.ok(Math.abs(halfW - 20) < 1e-9 && Math.abs(halfH - 100) < 1e-9, `${halfW} by ${halfH}`);

	// At forty-five degrees both sides project the same, so the box is square:
	// (100 + 20) divided by root two.
	const [diagW, diagH] = boundsOf({ ...beam, rotation: 45 }, 64);
	assert.ok(Math.abs(diagW - diagH) < 1e-9, `${diagW} by ${diagH}`);
	assert.ok(Math.abs(diagW - 120 / Math.SQRT2) < 1e-9, `${diagW}`);

	// A creature has no facing, so its box is its disc whatever it carries.
	assert.deepEqual(boundsOf({ kind: "monster", size: "large", width: 0, height: 0, rotation: 90 }, 64), [64, 64]);
});
