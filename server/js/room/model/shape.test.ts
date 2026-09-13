import assert from "node:assert/strict";
import { test } from "node:test";
import type { Grid } from "../protocol.ts";
import { boundsOf, footprintOf, pawnExtents, snapPawn, snapsToGrid } from "./shape.ts";
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
test("snapPawn reads the footprint off the pawn", () => {
	const large = { kind: "monster" as const, size: "large" as const, width: 0, height: 0, rotation: 0 };
	assert.deepEqual(snapPawn(grid(), large, 100, 100), [128, 128]);
	const medium = { kind: "monster" as const, size: "medium" as const, width: 0, height: 0, rotation: 0 };
	assert.deepEqual(snapPawn(grid(), medium, 100, 100), [96, 96]);
});
test("an object keeps the pixel it was given, whatever the grid says", () => {
	const wagon = { kind: "object" as const, size: "medium" as const, width: 128, height: 192, rotation: 0 };
	assert.deepEqual(snapPawn(grid(), wagon, 100, 100), [100, 100]);
	assert.deepEqual(snapPawn(grid(), wagon, 7, -3), [7, -3]);
	assert.equal(snapsToGrid(wagon, grid()), false);
	assert.equal(snapsToGrid({ ...wagon, kind: "monster" }, grid()), true);
	assert.equal(snapsToGrid({ ...wagon, kind: "monster" }, { ...grid(), snap: "off" }), false);
});
test("a creature's footprint is its size category", () => {
	assert.equal(footprintOf("tiny"), 1);
	assert.equal(footprintOf("medium"), 1);
	assert.equal(footprintOf("large"), 2);
	assert.equal(footprintOf("huge"), 3);
	assert.equal(footprintOf("gargantuan"), 4);
	assert.equal(footprintOf("colossal" as never), 1);
});
test("an object is drawn at its picture's size and a creature at its cell's", () => {
	assert.deepEqual(
		pawnExtents({ kind: "object", size: "medium", width: 100, height: 40, rotation: 0 }, 64),
		[50, 20],
	);
	assert.deepEqual(
		pawnExtents({ kind: "monster", size: "large", width: 0, height: 0, rotation: 0 }, 64),
		[64, 64],
	);
	assert.deepEqual(
		pawnExtents({ kind: "monster", size: "tiny", width: 0, height: 0, rotation: 0 }, 64),
		[16, 16],
	);
	assert.deepEqual(
		pawnExtents({ kind: "object", size: "medium", width: 0, height: 0, rotation: 0 }, 64),
		[0.5, 0.5],
	);
});
test("a turned token's screen box is not the box it is drawn in", () => {
	const beam = { kind: "object" as const, size: "medium" as const, width: 200, height: 40, rotation: 0 };
	assert.deepEqual(boundsOf(beam, 64), [100, 20]);
	assert.deepEqual(pawnExtents({ ...beam, rotation: 90 }, 64), [100, 20], "the quad grew as it spun");
	const [halfW, halfH] = boundsOf({ ...beam, rotation: 90 }, 64);
	assert.ok(Math.abs(halfW - 20) < 1e-9 && Math.abs(halfH - 100) < 1e-9, `${halfW} by ${halfH}`);
	const [diagW, diagH] = boundsOf({ ...beam, rotation: 45 }, 64);
	assert.ok(Math.abs(diagW - diagH) < 1e-9, `${diagW} by ${diagH}`);
	assert.ok(Math.abs(diagW - 120 / Math.SQRT2) < 1e-9, `${diagW}`);
	assert.deepEqual(boundsOf({ kind: "monster", size: "large", width: 0, height: 0, rotation: 90 }, 64), [64, 64]);
});
