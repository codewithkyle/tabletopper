import assert from "node:assert/strict";
import { test } from "node:test";
import type { Drawn } from "./pawn-pass.ts";
import type { Pawn } from "../protocol.ts";
import { CONDITION_RINGS_MAX, RING_GAP, RING_WIDTH, ringRadius, visiblePawns } from "./scene.ts";
import { SPRITE_SIZE } from "./sprites.ts";
import { fitFactors } from "./pawn-pass.ts";
import { pawnExtents } from "../model/shape.ts";
const GROUND = "01LAYERGROUND";
const CELLAR = "01LAYERCELLAR";
function pawn(over: Partial<Pawn> = {}): Pawn {
	return {
		id: "01PAWN",
		kind: "monster",
		layerId: GROUND,
		name: "Goblin",
		image: "",
		x: 0,
		y: 0,
		z: 1,
		size: "medium",
		width: 0,
		height: 0,
		rotation: 0,
		visible: true,
		hp: 7,
		maxHp: 7,
		hpBand: null,
		ac: 15,
		conditions: [],
		ownerId: null,
		monsterId: null,
		characterId: null,
		...over,
	};
}
test("only the viewed floor's pawns are drawn", () => {
	const pawns = [
		pawn({ id: "a", layerId: GROUND }),
		pawn({ id: "b", layerId: CELLAR }),
		pawn({ id: "c", layerId: GROUND }),
	];
	const out: Drawn[] = [];
	visiblePawns(pawns, GROUND, out);
	assert.deepEqual(out.map((p) => p.id), ["a", "c"]);
	visiblePawns(pawns, CELLAR, out);
	assert.deepEqual(out.map((p) => p.id), ["b"]);
	visiblePawns(pawns, "01LAYERATTIC", out);
	assert.deepEqual(out, []);
});
test("rebuilding writes into the same objects", () => {
	const out: Drawn[] = [];
	visiblePawns([pawn({ id: "a", x: 10 })], GROUND, out);
	const first = out[0];
	visiblePawns([pawn({ id: "a", x: 99 })], GROUND, out);
	assert.equal(out[0], first, "a rebuild allocated a fresh object");
	assert.equal(out[0].x, 99);
});
test("a pawn players cannot see is marked hidden", () => {
	const out: Drawn[] = [];
	visiblePawns([pawn({ visible: false })], GROUND, out);
	assert.equal(out[0].hidden, true);
});
test("health is whichever the viewer was told, and the number wins", () => {
	const out: Drawn[] = [];
	visiblePawns([pawn({ hp: 0 })], GROUND, out);
	assert.equal(out[0].health, "dead");
	visiblePawns([pawn({ hp: 1, maxHp: 7 })], GROUND, out);
	assert.equal(out[0].health, "veryBloody");
	visiblePawns([pawn({ hp: null, maxHp: null, hpBand: "dead" })], GROUND, out);
	assert.equal(out[0].health, "dead", "a player was not shown the band they were sent");
	visiblePawns([pawn({ hp: null, maxHp: null, hpBand: "nearDeath" })], GROUND, out);
	assert.equal(out[0].health, "nearDeath");
	visiblePawns([pawn({ hp: null, maxHp: null, hpBand: null })], GROUND, out);
	assert.equal(out[0].health, null, "health was invented out of nothing at all");
	visiblePawns([pawn({ hp: 4, maxHp: 7, hpBand: "dead" })], GROUND, out);
	assert.equal(out[0].health, "bruised");
});
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
test("condition rings step outwards by a fixed screen distance", () => {
	const half = 32;
	const worldPerDevicePixel = 2;
	const first = ringRadius(half, 0, worldPerDevicePixel);
	const second = ringRadius(half, 1, worldPerDevicePixel);
	assert.equal(first, half + RING_GAP * worldPerDevicePixel);
	assert.equal(second - first, (RING_WIDTH + RING_GAP) * worldPerDevicePixel);
	assert.ok(ringRadius(half, 0, 0.5) < first);
});
test("the ring cap is the protocol's own", () => {
	assert.equal(CONDITION_RINGS_MAX, 16);
});
test("the pass and the cache agree on the sprite layer's size", () => {
	assert.equal(SPRITE_SIZE, 256);
});
