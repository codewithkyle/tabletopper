import assert from "node:assert/strict";
import { test } from "node:test";
import type { Grid } from "../protocol.ts";
import {
	cellAt,
	cellCentre,
	cellPath,
	cellsBetween,
	cellsMoved,
	distanceLabel,
	feetBetween,
	feetMoved,
	snapAxis,
	snapPoint,
	supercover,
} from "./grid.ts";
function grid(over: Partial<Grid> = {}): Grid {
	return {
		type: "square",
		lines: "solid",
		cellSize: 64,
		offsetX: 0,
		offsetY: 0,
		color: "#000000FF",
		snap: "cells",
		feetPerCell: 5,
		units: "feet",
		diagonals: "equal",
		...over,
	};
}
test("snapAxis takes every parity and mode", () => {
	const cases: [string, number, Grid["snap"], number, number][] = [
		["odd footprint under cells lands on a centre", 1, "cells", 100, 96],
		["even footprint under cells lands on a vertex", 2, "cells", 100, 128],
		["odd footprint under half-cells takes whichever is nearer", 1, "halfCells", 100, 96],
		["even footprint under half-cells takes the same one", 2, "halfCells", 100, 96],
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
test("snapAxis breaks ties away from zero, as Go does", () => {
	assert.equal(snapAxis(64, 0, 1, "cells", 64), 96);
	assert.equal(snapAxis(64, 0, 1, "cells", 0), -32);
});
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
test("snapAxis takes a negative offset", () => {
	assert.equal(snapAxis(64, -16, 1, "cells", 0), 16);
	assert.equal(snapAxis(64, -16, 2, "cells", 0), -16);
});
test("snapPoint takes each axis separately", () => {
	assert.deepEqual(snapPoint(grid(), 2, 3, 100, 100), [128, 96]);
});
test("an impossible grid leaves the coordinate alone", () => {
	assert.equal(snapAxis(0, 0, 1, "cells", 100), 100);
});
test("a horizontal path is the row of cells between the two", () => {
	const cells: number[] = [];
	supercover(0, 0, 3, 0, cells);
	assert.deepEqual(cells, [0, 0, 1, 0, 2, 0, 3, 0]);
});
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
test("equal diagonals count the Chebyshev distance", () => {
	assert.equal(cellsMoved(3, 2, "equal"), 3);
	assert.equal(cellsMoved(0, 4, "equal"), 4);
	assert.equal(cellsMoved(-3, 3, "equal"), 3);
	assert.equal(feetMoved(3, 2, grid()), 15);
});
test("alternating diagonals run 5, 15, 20, 30", () => {
	const feet = [1, 2, 3, 4].map((n) => feetMoved(n, n, grid({ diagonals: "alternating" })));
	assert.deepEqual(feet, [5, 15, 20, 30]);
});
test("alternating charges the straights at face value", () => {
	assert.equal(cellsMoved(4, 1, "alternating"), 4);
	assert.equal(cellsMoved(4, 1, "equal"), 4);
});
test("a table with ten foot cells doubles every answer", () => {
	assert.equal(feetMoved(3, 3, grid({ feetPerCell: 10 })), 30);
});
test("a free measurement is the hypotenuse and not the square count", () => {
	const g = grid();
	assert.equal(feetMoved(3, 3, g), 15);
	assert.equal(Math.round(feetBetween(3 * 64, 3 * 64, g)), 21);
});
test("a free measurement is cells of pixels turned into feet", () => {
	assert.equal(feetBetween(64, 0, grid()), 5);
	assert.equal(feetBetween(0, -128, grid()), 10);
	assert.equal(feetBetween(64, 0, grid({ feetPerCell: 10 })), 10);
	assert.equal(feetBetween(64, 0, grid({ offsetX: -17, offsetY: 5 })), 5);
});
test("a free measurement does not know which diagonal rule the table uses", () => {
	const equal = feetBetween(64, 64, grid({ diagonals: "equal" }));
	const alternating = feetBetween(64, 64, grid({ diagonals: "alternating" }));
	assert.equal(equal, alternating);
});
test("a free measurement of no distance is no distance", () => {
	assert.equal(feetBetween(0, 0, grid()), 0);
	assert.equal(feetBetween(0, 0, grid({ cellSize: 0 })), 0);
	assert.equal(Number.isFinite(feetBetween(64, 64, grid({ cellSize: 0 }))), true);
});
test("cells and their centres are inverses under an offset", () => {
	const g = grid({ offsetX: -16, offsetY: 24 });
	for (const [cx, cy] of [[0, 0], [3, -2], [-7, 5]]) {
		const [x, y] = cellCentre(g, cx, cy);
		assert.deepEqual(cellAt(g, x, y), [cx, cy]);
	}
});
test("a distance label uses only what the glyph atlas holds", () => {
	for (const feet of [0, 5, 15, 120, 1005]) {
		assert.match(distanceLabel(feet, grid()), /^[0-9]+ ft\.$/);
	}
	assert.equal(distanceLabel(14.6, grid()), "15 ft.");
});
test("a distance label names the unit the table is measured in", () => {
	assert.equal(distanceLabel(12, grid({ units: "miles" })), "12 mi");
	assert.equal(distanceLabel(12, grid({ units: "kilometres" })), "12 km");
	assert.equal(distanceLabel(12, grid({ units: "cells" })), "12 sq.");
	assert.equal(distanceLabel(12, grid({ type: "hexPointy", units: "cells" })), "12 hex");
	assert.equal(distanceLabel(12, grid({ type: "hexFlat", units: "cells" })), "12 hex");
});
test("a table measured in cells counts cells and not feet", () => {
	assert.equal(feetMoved(3, 2, grid({ units: "cells", feetPerCell: 5 })), 3);
	assert.equal(feetBetween(3 * 64, 0, grid({ units: "cells", feetPerCell: 5 })), 3);
});
test("cells and their centres are inverses on a hex grid too", () => {
	for (const type of ["hexPointy", "hexFlat"] as const) {
		const g = grid({ type, offsetX: -16, offsetY: 24 });
		for (const [q, r] of [[0, 0], [3, -2], [-7, 5]]) {
			const [x, y] = cellCentre(g, q, r);
			assert.deepEqual(cellAt(g, x, y), [q, r], `${type} (${q}, ${r})`);
		}
	}
});
test("a hex path is walked in hexes and a square one in squares", () => {
	const cells: number[] = [];
	assert.deepEqual(cellPath(grid({ type: "hexPointy" }), 0, 0, 3, -1, cells), [0, 0, 1, 0, 2, -1, 3, -1]);
	assert.deepEqual(cellPath(grid(), 0, 0, 3, 0, cells), [0, 0, 1, 0, 2, 0, 3, 0]);
});
test("a hex grid counts cube distance and ignores the diagonal rule", () => {
	const g = grid({ type: "hexPointy" });
	assert.equal(cellsBetween(3, -3, g), 3);
	assert.equal(cellsBetween(-2, 5, g), 5);
	assert.equal(cellsBetween(3, 3, g), cellsBetween(3, 3, grid({ type: "hexPointy", diagonals: "alternating" })));
	assert.equal(feetMoved(-2, 5, g), 25);
});
