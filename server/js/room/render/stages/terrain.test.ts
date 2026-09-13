import assert from "node:assert/strict";
import { test } from "node:test";
import type { Grid, Tile, TileArt } from "../../protocol.ts";
import type { Laid } from "./terrain.ts";
import type { FrameContext } from "../frame-context.ts";
import type { Resources } from "../resources.ts";
import type { SpriteCache } from "../sprites.ts";
import { cellExtents } from "../../model/grid.ts";
import { createPawnProgram } from "../pawn-pass.ts";
import { empty } from "../../store.ts";
import { fakeDOM, recordingGL } from "./testing.ts";
import { newFrame } from "../frame-context.ts";
import { newOverlay } from "../../model/overlay.ts";
import { placedTiles, terrainStage } from "./terrain.ts";
import { revisions } from "../../model/revisions.ts";
function spriteShelf() {
	const here = new Set<string>();
	const wanted: string[] = [];
	return {
		wanted,
		arrive(url: string) {
			here.add(url);
		},
		cache: {
			sprite(url: string) {
				wanted.push(url);
				return here.has(url) ? { layer: 0, w: 256, h: 256 } : null;
			},
			texture: () => ({}),
		} as unknown as SpriteCache,
	};
}
function stageResources(gl: WebGL2RenderingContext, shelf: ReturnType<typeof spriteShelf>): Resources {
	return { sprites: shelf.cache, pawnProgram: createPawnProgram(gl) } as unknown as Resources;
}
function terrainFrame(gl: WebGL2RenderingContext, shelf: ReturnType<typeof spriteShelf>): FrameContext {
	const frame = newFrame(
		gl, { x: 0, y: 0, zoom: 1 }, { width: 100, height: 100 },
		"gm", "01USER", empty(), revisions(), newOverlay(),
		{} as unknown as Resources,
	);
	frame.resources = stageResources(gl, shelf);
	frame.viewedID = GROUND;
	frame.state.table.grid = grid();
	return frame;
}
const GROUND = "01LAYERGROUND";
const CELLAR = "01LAYERCELLAR";
const PINE = "01ARTPINE";
const HILL = "01ARTHILL";
function grid(over: Partial<Grid> = {}): Grid {
	return {
		type: "square", lines: "solid", cellSize: 64, offsetX: 0, offsetY: 0,
		color: "#000000FF", snap: "cells", feetPerCell: 5, units: "feet", diagonals: "equal",
		...over,
	};
}
function art(id: string, image: string): TileArt {
	return { id, assetId: "01ASSET" + id, name: id, image };
}
function tile(over: Partial<Tile> = {}): Tile {
	return { layerId: GROUND, art: PINE, q: 0, r: 0, rotation: 0, by: "01GM", ...over };
}
const PALETTE = [art(PINE, "/pine.webp"), art(HILL, "/hill.webp")];
test("only the viewed floor's tiles are placed", () => {
	const out: Laid[] = [];
	const tiles = [tile({ q: 1 }), tile({ layerId: CELLAR, q: 2 }), tile({ q: 3 })];
	placedTiles(tiles, PALETTE, GROUND, grid(), out);
	assert.deepEqual(out.map((p) => p.x), [96, 224]);
	placedTiles(tiles, PALETTE, CELLAR, grid(), out);
	assert.equal(out.length, 1);
	placedTiles(tiles, PALETTE, "01LAYERATTIC", grid(), out);
	assert.deepEqual(out, []);
});
test("a tile whose art left the bag is not drawn at all", () => {
	const out: Laid[] = [];
	placedTiles([tile(), tile({ q: 1, art: "01ARTGONE" })], PALETTE, GROUND, grid(), out);
	assert.deepEqual(out.map((p) => p.image), ["/pine.webp"]);
});
test("a tile sits at the centre of its own cell and keeps its turn", () => {
	const out: Laid[] = [];
	placedTiles([tile({ q: 2, r: 1, art: HILL, rotation: 90 })], PALETTE, GROUND, grid(), out);
	assert.deepEqual(out[0], { image: "/hill.webp", x: 160, y: 96, rotation: 90 });
});
test("the same cell on a hex grid lands where the hex does", () => {
	const out: Laid[] = [];
	placedTiles([tile({ q: 1, r: 0 })], PALETTE, GROUND, grid({ type: "hexPointy" }), out);
	assert.equal(out[0].x, 96, "a hex column is one cell across the flats");
	assert.equal(out[0].y, 32, "the first row does not step down");
});
test("rebuilding writes into the same objects", () => {
	const out: Laid[] = [];
	placedTiles([tile()], PALETTE, GROUND, grid(), out);
	const first = out[0];
	placedTiles([tile({ rotation: 180 })], PALETTE, GROUND, grid(), out);
	assert.equal(out[0], first, "a rebuild allocated a fresh object");
	assert.equal(out[0].rotation, 180);
});
test("a cell is as wide as the grid says and as tall as its shape needs", () => {
	assert.deepEqual(cellExtents(grid()), [32, 32]);
	const pointy = cellExtents(grid({ type: "hexPointy" }));
	assert.equal(pointy[0], 32, "a pointy-top hex is one cell across its flats");
	assert.ok(Math.abs(pointy[1] - 64 / Math.sqrt(3)) < 1e-9, "and two circumradii tall");
	const flat = cellExtents(grid({ type: "hexFlat" }));
	assert.deepEqual(flat, [pointy[1], pointy[0]], "a flat-top hex is a pointy-top one on its side");
});
test("a grid with no size still gives a cell something to be", () => {
	assert.deepEqual(cellExtents(grid({ cellSize: 0 })), [0.5, 0.5]);
});
test("a tile is drawn as soon as its picture arrives, not only when the table changes", () => {
	const dom = fakeDOM();
	try {
		const gl = recordingGL();
		const shelf = spriteShelf();
		const stage = terrainStage(gl.gl, stageResources(gl.gl, shelf));
		const frame = terrainFrame(gl.gl, shelf);
		frame.state.table.palette = [art(PINE, "/pine.webp")];
		frame.state.tiles = [tile()];
		stage.build?.(frame);
		gl.reset();
		stage.draw(frame);
		assert.deepEqual(gl.draws(), [], "a tile drew before its picture had loaded");
		shelf.arrive("/pine.webp");
		frame.rebuild = true;
		stage.build?.(frame);
		gl.reset();
		stage.draw(frame);
		assert.deepEqual(gl.draws(), ["drawArraysInstanced"], "the tile never appeared once its picture had loaded");
	} finally {
		dom.restore();
	}
});
test("every tile asks for its picture on a rebuild, so the cache keeps it resident", () => {
	const dom = fakeDOM();
	try {
		const gl = recordingGL();
		const shelf = spriteShelf();
		shelf.arrive("/pine.webp");
		const stage = terrainStage(gl.gl, stageResources(gl.gl, shelf));
		const frame = terrainFrame(gl.gl, shelf);
		frame.state.table.palette = [art(PINE, "/pine.webp")];
		frame.state.tiles = [tile()];
		stage.build?.(frame);
		const asked = shelf.wanted.length;
		assert.ok(asked > 0, "the first build asked for nothing");
		shelf.wanted.length = 0;
		frame.rebuild = true;
		stage.build?.(frame);
		assert.ok(shelf.wanted.length > 0, "a rebuild did not touch the picture, so it can be evicted");
	} finally {
		dom.restore();
	}
});
