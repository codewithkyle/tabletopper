import assert from "node:assert/strict";
import { test } from "node:test";
import type { MapRef } from "../protocol.ts";
import type { Rect } from "../model/types.ts";
import { levelFor, levelScale, newRange, rangeCount, tileKey, tileRect, tileURL, uvFor, visibleRange } from "./tiles.ts";
const map: MapRef = { assetId: "a", gen: "g", width: 12000, height: 9000, tileSize: 512, maxZoom: 5 };
const rect = (): Rect => ({ x1: 0, y1: 0, x2: 0, y2: 0 });
const view = (x1: number, y1: number, x2: number, y2: number): Rect => ({ x1, y1, x2, y2 });
test("the level follows the zoom and the pixel ratio together", () => {
	for (const c of [
		{ zoom: 1, dpr: 1, want: 0 },
		{ zoom: 0.5, dpr: 1, want: 1 },
		{ zoom: 0.3, dpr: 1, want: 2 },
		{ zoom: 0.1, dpr: 1, want: 3 },
		{ zoom: 1, dpr: 2, want: 0 },
		{ zoom: 0.5, dpr: 2, want: 0 },
		{ zoom: 0.3, dpr: 2, want: 1 },
		{ zoom: 0.1, dpr: 2, want: 2 },
	]) {
		assert.equal(levelFor(c.zoom, c.dpr, 5), c.want, `zoom ${c.zoom} at ${c.dpr}x`);
	}
});
test("the level is clamped to the pyramid at both ends", () => {
	assert.equal(levelFor(4, 1, 5), 0, "zoomed past native");
	assert.equal(levelFor(0.001, 1, 5), 5, "zoomed past the top level");
	assert.equal(levelFor(0, 1, 5), 5, "a zoom of zero");
});
test("a level's scale comes from its pixel count and not from a shift", () => {
	assert.equal(levelScale(12000, 0), 1);
	assert.equal(levelScale(12000, 5), 32, "12000 divides evenly");
	assert.equal(levelScale(9000, 5), 9000 / 282);
	assert.notEqual(levelScale(9000, 5), 32);
});
test("the top level covers the map exactly and does not overshoot", () => {
	const out = tileRect(map, 5, 0, 0, rect());
	assert.equal(out.x1, 0);
	assert.equal(out.y1, 0);
	assert.equal(out.x2, 12000);
	assert.equal(out.y2, 9000, "a shift would give 9024 here");
});
test("tiles of a level meet with no gap and the last ends at the edge", () => {
	const a = tileRect(map, 0, 22, 0, rect());
	const b = tileRect(map, 0, 23, 0, rect());
	assert.equal(a.x2, b.x1, "adjacent tiles must share an edge");
	assert.equal(b.x2, 12000, "the last tile ends at the map's edge");
	assert.equal(b.x2 - b.x1, 12000 - 23 * 512, "the last tile is the remainder");
});
test("the visible range covers the whole map when the viewport does", () => {
	const r = visibleRange(map, 0, view(-5000, -5000, 20000, 20000), newRange());
	assert.deepEqual(r, { x0: 0, y0: 0, x1: 23, y1: 17 });
	assert.equal(rangeCount(r), 24 * 18);
});
test("a viewport inside one tile asks for one tile", () => {
	const r = visibleRange(map, 0, view(10, 10, 500, 500), newRange());
	assert.deepEqual(r, { x0: 0, y0: 0, x1: 0, y1: 0 });
	assert.equal(rangeCount(r), 1);
});
test("a viewport ending on a boundary stops at that tile", () => {
	assert.equal(visibleRange(map, 0, view(0, 0, 512, 512), newRange()).x1, 0);
	assert.equal(visibleRange(map, 0, view(0, 0, 513, 513), newRange()).x1, 1);
});
test("the range is clipped to the map at the right and bottom edges", () => {
	const r = visibleRange(map, 0, view(11900, 8900, 13000, 10000), newRange());
	assert.deepEqual(r, { x0: 23, y0: 17, x1: 23, y1: 17 });
});
test("a viewport off the map is an empty range", () => {
	for (const off of [view(-9000, 0, -100, 9000), view(13000, 0, 20000, 9000), view(0, -500, 12000, -1)]) {
		assert.equal(rangeCount(visibleRange(map, 0, off, newRange())), 0);
	}
});
test("a tile's own texture coordinates run the full layer, or short at an edge", () => {
	const middle = tileRect(map, 0, 5, 5, rect());
	const full = uvFor(map, 0, 5, 5, middle, rect());
	assert.deepEqual(full, { x1: 0, y1: 0, x2: 1, y2: 1 });
	const edge = tileRect(map, 0, 23, 0, rect());
	const short = uvFor(map, 0, 23, 0, edge, rect());
	assert.equal(short.x1, 0);
	assert.equal(short.x2, (12000 - 23 * 512) / 512, "the edge tile's UV maximum is its real width");
});
test("a tile drawn from its parent takes the right quarter of it", () => {
	const child = tileRect(map, 0, 3, 0, rect());
	const uv = uvFor(map, 1, 1, 0, child, rect());
	assert.deepEqual(uv, { x1: 0.5, y1: 0, x2: 1, y2: 0.5 });
	const below = tileRect(map, 0, 3, 1, rect());
	assert.deepEqual(uvFor(map, 1, 1, 0, below, rect()), { x1: 0.5, y1: 0.5, x2: 1, y2: 1 });
});
test("a tile drawn from two levels up takes a quarter of it", () => {
	const child = tileRect(map, 0, 3, 3, rect());
	const uv = uvFor(map, 2, 0, 0, child, rect());
	assert.deepEqual(uv, { x1: 0.75, y1: 0.75, x2: 1, y2: 1 });
});
test("a tile key names the generation, so a re-tiled map shares nothing", () => {
	const retiled: MapRef = { ...map, gen: "g2" };
	assert.notEqual(tileKey(map, 0, 1, 2), tileKey(retiled, 0, 1, 2));
	assert.equal(tileURL(map, 3, 4, 5), "/assets/maps/a/tiles/g/3/4_5.webp");
});
