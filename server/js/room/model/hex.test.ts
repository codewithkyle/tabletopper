import assert from "node:assert/strict";
import { test } from "node:test";
import type { Grid } from "../protocol.ts";
import { PATH_CELLS_MAX, hexAt, hexCentre, hexCorners, hexDistance, hexLine } from "./hex.ts";
function grid(type: Grid["type"], over: Partial<Grid> = {}): Grid {
	return {
		type,
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
test("a hex centre is spaced by the across-flats size, as the server spaces it", () => {
	const cases: [Grid["type"], number, number, number, number][] = [
		["hexPointy", 0, 0, 32, 32],
		["hexPointy", 1, 0, 96, 32],
		["hexPointy", 0, 1, 64, 87],
		["hexPointy", -1, 2, 32, 143],
		["hexPointy", 2, -1, 128, -23],
		["hexPointy", 3, 3, 320, 198],
		["hexFlat", 0, 0, 32, 32],
		["hexFlat", 1, 0, 87, 64],
		["hexFlat", 0, 1, 32, 96],
		["hexFlat", -1, 2, -23, 128],
		["hexFlat", 2, -1, 143, 32],
		["hexFlat", 3, 3, 198, 320],
	];
	for (const [type, q, r, x, y] of cases) {
		assert.deepEqual(hexCentre(grid(type), q, r), [x, y], `${type} (${q}, ${r})`);
	}
});
test("a point reads back the hex it is inside, as the server reads it", () => {
	const cases: [Grid["type"], number, number, number, number][] = [
		["hexPointy", 32, 32, 0, 0],
		["hexPointy", 100, 40, 1, 0],
		["hexPointy", 64, 87, 0, 1],
		["hexPointy", 0, 0, 0, -1],
		["hexPointy", 200, 200, 1, 3],
		["hexPointy", -40, -40, -1, -1],
		["hexFlat", 32, 32, 0, 0],
		["hexFlat", 100, 70, 1, 0],
		["hexFlat", 32, 96, 0, 1],
		["hexFlat", 0, 0, -1, 0],
		["hexFlat", 200, 200, 3, 1],
		["hexFlat", -40, -40, -1, -1],
	];
	for (const [type, x, y, q, r] of cases) {
		assert.deepEqual(hexAt(grid(type), x, y), [q, r], `${type} (${x}, ${y})`);
	}
});
test("every hex centre lands back in its own hex", () => {
	const grids = [
		grid("hexPointy"),
		grid("hexFlat"),
		grid("hexPointy", { cellSize: 97, offsetX: 17, offsetY: -9 }),
		grid("hexFlat", { cellSize: 97, offsetX: 17, offsetY: -9 }),
	];
	for (const g of grids) {
		for (let q = -8; q <= 8; q++) {
			for (let r = -8; r <= 8; r++) {
				const [x, y] = hexCentre(g, q, r);
				assert.deepEqual(hexAt(g, x, y), [q, r], `${g.type} (${q}, ${r}) centres at (${x}, ${y})`);
			}
		}
	}
});
test("a point on a shared edge resolves the same way every time", () => {
	const g = grid("hexPointy");
	const first = hexAt(g, 64, 32);
	for (let i = 0; i < 8; i++) {
		assert.deepEqual(hexAt(g, 64, 32), first);
	}
	assert.deepEqual(first, [1, 0]);
});
test("hex distance counts steps", () => {
	const cases: [number, number, number, number, number][] = [
		[0, 0, 0, 0, 0],
		[0, 0, 3, 0, 3],
		[0, 0, 0, 3, 3],
		[0, 0, -2, 5, 5],
		[0, 0, 3, -3, 3],
		[2, -1, -1, 3, 4],
	];
	for (const [q0, r0, q1, r1, want] of cases) {
		assert.equal(hexDistance(q0, r0, q1, r1), want, `(${q0}, ${r0}) to (${q1}, ${r1})`);
	}
});
test("a hex line walks every cell between the ends, as the server walks it", () => {
	const out: number[] = [];
	assert.deepEqual(hexLine(0, 0, 0, 0, out), [0, 0]);
	assert.deepEqual(hexLine(0, 0, 3, -1, out), [0, 0, 1, 0, 2, -1, 3, -1]);
	assert.deepEqual(hexLine(0, 0, 0, 3, out), [0, 0, 0, 1, 0, 2, 0, 3]);
	assert.deepEqual(hexLine(0, 0, -2, 5, out), [0, 0, 0, 1, -1, 2, -1, 3, -2, 4, -2, 5]);
});
test("every step of a hex line is one hex from the last", () => {
	const out: number[] = [];
	for (const [q, r] of [[7, 3], [-5, 9], [1, -12], [13, 13], [0, 6]]) {
		hexLine(0, 0, q, r, out);
		for (let i = 2; i < out.length; i += 2) {
			const step = hexDistance(out[i - 2], out[i - 1], out[i], out[i + 1]);
			assert.equal(step, 1, `step ${i / 2} to (${q}, ${r}) jumped ${step}`);
		}
		assert.deepEqual([out[out.length - 2], out[out.length - 1]], [q, r]);
	}
});
test("a hex line stops at the path cap", () => {
	const out: number[] = [];
	assert.equal(hexLine(0, 0, 5000, 0, out).length, PATH_CELLS_MAX * 2);
});
test("hex corners ring the centre, as the server rings it", () => {
	const out: number[] = [];
	assert.deepEqual(hexCorners(grid("hexPointy"), 0, 0, out), [64, 14, 64, 50, 32, 69, 0, 50, 0, 14, 32, -5]);
	assert.deepEqual(hexCorners(grid("hexFlat"), 0, 0, out), [69, 32, 50, 64, 14, 64, -5, 32, 14, 0, 50, 0]);
});
test("a hex is as wide across its flats as a square cell is wide", () => {
	const out: number[] = [];
	for (const [type, wide, tall] of [["hexPointy", 64, 74], ["hexFlat", 74, 64]] as const) {
		hexCorners(grid(type), 0, 0, out);
		const xs: number[] = [];
		const ys: number[] = [];
		for (let i = 0; i < out.length; i += 2) {
			xs.push(out[i]);
			ys.push(out[i + 1]);
		}
		assert.equal(Math.max(...xs) - Math.min(...xs), wide, `${type} width`);
		assert.equal(Math.max(...ys) - Math.min(...ys), tall, `${type} height`);
	}
});
