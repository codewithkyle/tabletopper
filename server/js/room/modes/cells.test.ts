import assert from "node:assert/strict";
import { test } from "node:test";
import type { Grid } from "../protocol.ts";
import { newCellWalker } from "./cells.ts";
import { grid } from "./testing.ts";
function hexGrid(): Grid {
	return grid({ type: "hexPointy" });
}
test("the first sample is the cell under the pointer", () => {
	const walker = newCellWalker();
	assert.deepEqual(walker.enter(grid(), 32, 32), [0, 0]);
});
test("staying inside one cell hands back nothing more", () => {
	const walker = newCellWalker();
	walker.enter(grid(), 32, 32);
	assert.deepEqual(walker.enter(grid(), 40, 40), []);
	assert.deepEqual(walker.enter(grid(), 63, 1), []);
});
test("a jump hands back every cell the line crossed", () => {
	const walker = newCellWalker();
	walker.enter(grid(), 32, 32);
	assert.deepEqual(walker.enter(grid(), 224, 32), [1, 0, 2, 0, 3, 0]);
});
test("coming back over a cell it already handed back hands it back once", () => {
	const walker = newCellWalker();
	walker.enter(grid(), 32, 32);
	walker.enter(grid(), 96, 32);
	assert.deepEqual(walker.enter(grid(), 32, 32), []);
});
test("a reset starts the gesture again", () => {
	const walker = newCellWalker();
	walker.enter(grid(), 32, 32);
	walker.reset();
	assert.deepEqual(walker.enter(grid(), 32, 32), [0, 0]);
});
test("on a hex grid it walks in axial coordinates", () => {
	const walker = newCellWalker();
	const g = hexGrid();
	assert.deepEqual(walker.enter(g, 32, 32), [0, 0]);
	const crossed = walker.enter(g, 32 + 64 * 2, 32);
	assert.equal(crossed.length % 2, 0);
	assert.ok(crossed.length >= 2, "a jump of two hexes crossed nothing");
	for (let i = 0; i < crossed.length; i += 2) {
		assert.ok(crossed[i] !== 0 || crossed[i + 1] !== 0, "the hex it started on came back a second time");
	}
});
test("the walker reuses its array", () => {
	const walker = newCellWalker();
	const first = walker.enter(grid(), 32, 32);
	assert.deepEqual(first, [0, 0]);
	walker.enter(grid(), 96, 32);
	assert.deepEqual(first, [1, 0]);
});
