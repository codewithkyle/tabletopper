// What reaches the canvas out of what the store holds, and how big it is drawn.
//
// THE LAYER FILTER IS THE SECURITY-ADJACENT ONE. A player's store never holds a
// pawn from another floor, because the projection removed it before the event
// was encoded -- but the GM's does, and the GM may be looking at the cellar
// while the party is upstairs. A filter that leaked would put the party's pawns
// on the cellar's map, which reads as the renderer being broken rather than as
// a floor being wrong.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { Drawn } from "./pawn-pass.ts";
import type { Pawn } from "../protocol.ts";
import { CONDITION_RINGS_MAX, RING_GAP, RING_WIDTH, ringRadius, visiblePawns } from "./scene.ts";
import { SPRITE_SIZE } from "./sprites.ts";
import { fitFactors } from "./pawn-pass.ts";
import { pawnExtents } from "./path.ts";

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

	// A floor with nothing on it is an empty list rather than a stale one. The
	// array is reused, so an implementation that forgot to shorten it would
	// leave the last floor's pawns hanging off the end of this one.
	visiblePawns(pawns, "01LAYERATTIC", out);
	assert.deepEqual(out, []);
});

// The array and the objects in it are reused, which is the second performance
// rule applied to a rebuild that happens on every move.
test("rebuilding writes into the same objects", () => {
	const out: Drawn[] = [];

	visiblePawns([pawn({ id: "a", x: 10 })], GROUND, out);
	const first = out[0];

	visiblePawns([pawn({ id: "a", x: 99 })], GROUND, out);

	assert.equal(out[0], first, "a rebuild allocated a fresh object");
	assert.equal(out[0].x, 99);
});

// A pawn a player cannot see never reaches their store, so this is the GM's
// marker and nobody else's.
test("a pawn players cannot see is marked hidden", () => {
	const out: Drawn[] = [];
	visiblePawns([pawn({ visible: false })], GROUND, out);

	assert.equal(out[0].hidden, true);
});

// DEAD IS A NUMBER THE VIEWER WAS ACTUALLY GIVEN. A monster in a room that
// hides its hit points arrives with hp null, and inferring death from a band
// would leak exactly what the setting exists to withhold.
test("dead is only what the viewer was told", () => {
	const out: Drawn[] = [];

	visiblePawns([pawn({ hp: 0 })], GROUND, out);
	assert.equal(out[0].dead, true);

	visiblePawns([pawn({ hp: 1 })], GROUND, out);
	assert.equal(out[0].dead, false);

	visiblePawns([pawn({ hp: null, maxHp: null, hpBand: "dead" })], GROUND, out);
	assert.equal(out[0].dead, false, "a band was read as a hit-point count");
});

// A creature's footprint is its size category and an object's is its picture.
// A tiny creature OCCUPIES a whole cell -- half a cell is not a position any
// grid rule can express -- and is DRAWN at half of one, so a rat and an ogre
// are not the same size on the table.
test("a pawn covers as much floor as its size says", () => {
	const creature = (size: Pawn["size"]) => pawnExtents({ kind: "monster", size, width: 0, height: 0 }, 64);

	assert.deepEqual(creature("medium"), [32, 32]);
	assert.deepEqual(creature("large"), [64, 64]);
	assert.deepEqual(creature("gargantuan"), [128, 128]);
	assert.deepEqual(creature("tiny"), [16, 16]);

	assert.deepEqual(
		pawnExtents({ kind: "object", size: "medium", width: 128, height: 256 }, 64),
		[64, 128],
	);

	// AND AN OBJECT DOES NOT CARE WHAT THE CELL SIZE IS. The same wagon on a
	// hundred-pixel grid is the same wagon.
	assert.deepEqual(
		pawnExtents({ kind: "object", size: "medium", width: 128, height: 256 }, 100),
		[64, 128],
	);
});

// The grid is where the cell size comes from, including when no map is set --
// which is a blank floor with an infinite grid on it, and pawns on that floor
// still have to be a sensible size.
test("the cell size comes from the grid and never from nothing", () => {
	assert.deepEqual(pawnExtents({ kind: "monster", size: "medium", width: 0, height: 0 }, 0), [0.5, 0.5]);
	assert.deepEqual(pawnExtents({ kind: "monster", size: "medium", width: 0, height: 0 }, 100), [50, 50]);
});

// CONTAIN LETTERBOXES AND COVER CROPS, and which one applies is the whole
// difference between an object and a creature. A wagon must be its own shape on
// the floor; a portrait in a circle must not have bars down the side.
test("an object's picture is contained and a creature's covers", () => {
	// A 3:1 wagon in a 2-by-4-cell footprint: half extents 64 by 128, so the
	// quad is 128 across and 256 down. Contained, the image is limited by
	// width -- 128 across and 42.7 down -- which is a sixth of the quad's
	// height, and the rest of the footprint is empty floor.
	const [cx, cy] = fitFactors(300, 100, 64, 128, false);
	assert.equal(cx, 1);
	assert.ok(Math.abs(cy - 1 / 6) < 1e-9, `cy = ${cy}`);

	// The same picture on a creature's square disc: covering it means running
	// off the sides by three to one and filling it top to bottom.
	const [dx, dy] = fitFactors(300, 100, 64, 64, true);
	assert.equal(dx, 3);
	assert.equal(dy, 1);
});

test("a square picture fits a square quad exactly, either way", () => {
	assert.deepEqual(fitFactors(256, 256, 32, 32, true), [1, 1]);
	assert.deepEqual(fitFactors(256, 256, 32, 32, false), [1, 1]);
});

// A sprite that has not arrived has no dimensions, and the fit has to have an
// answer rather than a division by zero that renders the whole table blank.
test("a picture with no size fits as though it were square", () => {
	assert.deepEqual(fitFactors(0, 0, 32, 32, true), [1, 1]);
	assert.deepEqual(fitFactors(256, 256, 0, 0, false), [1, 1]);
});

// The rings are concentric, outside the pawn, one gap apart, and they are
// spaced in screen pixels so a GM zoomed out to the whole map can still count
// them.
test("condition rings step outwards by a fixed screen distance", () => {
	const half = 32;
	const worldPerDevicePixel = 2;

	const first = ringRadius(half, 0, worldPerDevicePixel);
	const second = ringRadius(half, 1, worldPerDevicePixel);

	assert.equal(first, half + RING_GAP * worldPerDevicePixel);
	assert.equal(second - first, (RING_WIDTH + RING_GAP) * worldPerDevicePixel);

	// Zoomed in, one CSS pixel is less of the table, so the rings sit closer to
	// the pawn in map units and the same distance from it on screen.
	assert.ok(ringRadius(half, 0, 0.5) < first);
});

// The protocol caps a pawn's conditions at sixteen, and the drawing matches it:
// a seventeenth ring would be one the server would not have accepted.
test("the ring cap is the protocol's own", () => {
	assert.equal(CONDITION_RINGS_MAX, 16);
});

// pawn-pass does its own arithmetic against the layer's edge rather than
// importing it, so that the pass does not depend on how the cache was built.
// This is the pin that keeps the two honest.
test("the pass and the cache agree on the sprite layer's size", () => {
	// A picture that fills the layer exactly has a UV maximum of one, which is
	// the arithmetic the pass performs with its own copy of the number.
	assert.equal(SPRITE_SIZE, 256);
});
