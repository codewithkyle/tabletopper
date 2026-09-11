import assert from "node:assert/strict";
import { test } from "node:test";
import type { Pawn } from "../protocol.ts";
import { SPRITE_SIZE } from "./sprites.ts";
import { fitFactors } from "./pawn-pass.ts";
import { pawnExtents } from "../model/shape.ts";
test("a pawn covers as much floor as its size says", () => {
	const creature = (size: Pawn["size"]) => pawnExtents({ kind: "monster", size, width: 0, height: 0, rotation: 0 }, 64);
	assert.deepEqual(creature("medium"), [32, 32]);
	assert.deepEqual(creature("large"), [64, 64]);
	assert.deepEqual(creature("gargantuan"), [128, 128]);
	assert.deepEqual(creature("tiny"), [16, 16]);
	assert.deepEqual(
		pawnExtents({ kind: "object", size: "medium", width: 128, height: 256, rotation: 0 }, 64),
		[64, 128],
	);
	assert.deepEqual(
		pawnExtents({ kind: "object", size: "medium", width: 128, height: 256, rotation: 0 }, 100),
		[64, 128],
	);
});
test("the cell size comes from the grid and never from nothing", () => {
	assert.deepEqual(pawnExtents({ kind: "monster", size: "medium", width: 0, height: 0, rotation: 0 }, 0), [0.5, 0.5]);
	assert.deepEqual(pawnExtents({ kind: "monster", size: "medium", width: 0, height: 0, rotation: 0 }, 100), [50, 50]);
});
test("an object's picture is contained and a creature's covers", () => {
	const [cx, cy] = fitFactors(300, 100, 64, 128, false);
	assert.equal(cx, 1);
	assert.ok(Math.abs(cy - 1 / 6) < 1e-9, `cy = ${cy}`);
	const [dx, dy] = fitFactors(300, 100, 64, 64, true);
	assert.equal(dx, 3);
	assert.equal(dy, 1);
});
test("a square picture fits a square quad exactly, either way", () => {
	assert.deepEqual(fitFactors(256, 256, 32, 32, true), [1, 1]);
	assert.deepEqual(fitFactors(256, 256, 32, 32, false), [1, 1]);
});
test("a picture with no size fits as though it were square", () => {
	assert.deepEqual(fitFactors(0, 0, 32, 32, true), [1, 1]);
	assert.deepEqual(fitFactors(256, 256, 0, 0, false), [1, 1]);
});
test("the pass and the cache agree on the sprite layer's size", () => {
	assert.equal(SPRITE_SIZE, 256);
});
