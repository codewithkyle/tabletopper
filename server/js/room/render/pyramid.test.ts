import assert from "node:assert/strict";
import { test } from "node:test";
import { levelPixels, levelTileSize, levelTiles, maxZoom, tileOrigin, tileSpan } from "./pyramid.ts";
test("a 12000 by 9000 map at 512 is six levels and 584 tiles", () => {
	const width = 12000;
	const height = 9000;
	const tile = 512;
	const levels = [
		{ pixelsX: 12000, pixelsY: 9000, tilesX: 24, tilesY: 18 },
		{ pixelsX: 6000, pixelsY: 4500, tilesX: 12, tilesY: 9 },
		{ pixelsX: 3000, pixelsY: 2250, tilesX: 6, tilesY: 5 },
		{ pixelsX: 1500, pixelsY: 1125, tilesX: 3, tilesY: 3 },
		{ pixelsX: 750, pixelsY: 563, tilesX: 2, tilesY: 2 },
		{ pixelsX: 375, pixelsY: 282, tilesX: 1, tilesY: 1 },
	];
	assert.equal(maxZoom(width, height, tile), levels.length - 1);
	let total = 0;
	levels.forEach((want, z) => {
		assert.equal(levelPixels(width, z), want.pixelsX, `pixels across at ${z}`);
		assert.equal(levelPixels(height, z), want.pixelsY, `pixels down at ${z}`);
		assert.equal(levelTiles(width, tile, z), want.tilesX, `tiles across at ${z}`);
		assert.equal(levelTiles(height, tile, z), want.tilesY, `tiles down at ${z}`);
		total += want.tilesX * want.tilesY;
	});
	assert.equal(total, 584);
});
test("levels ceil rather than floor", () => {
	assert.equal(levelPixels(9000, 5), 282);
	assert.equal(9000 >> 5, 281);
});
test("the last tile of a row is the remainder", () => {
	assert.equal(levelTileSize(12000, 512, 0, 22), 512);
	assert.equal(levelTileSize(12000, 512, 0, 23), 12000 - 23 * 512);
	assert.equal(levelTileSize(9000, 512, 5, 0), 282);
});
test("a tile index off the end of a level has no size", () => {
	assert.equal(levelTileSize(12000, 512, 0, 24), 0);
	assert.equal(levelTileSize(12000, 512, 0, -1), 0);
	assert.equal(levelTiles(12000, 0, 0), 0);
});
test("a tile's rectangle is native pixels at every level", () => {
	assert.equal(tileOrigin(512, 0, 3), 1536);
	assert.equal(tileSpan(512, 0), 512);
	assert.equal(tileOrigin(512, 3, 1), 4096);
	assert.equal(tileSpan(512, 3), 4096);
});
test("a map smaller than one tile is a single level", () => {
	assert.equal(maxZoom(300, 200, 512), 0);
	assert.equal(levelTiles(300, 512, 0), 1);
	assert.equal(levelTileSize(300, 512, 0, 0), 300);
});
