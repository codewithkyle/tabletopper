// The viewed layer, which is the one piece of room state that is not the
// room's: players see what the GM made active, the GM may look somewhere else,
// and the difference has to be invisible to everybody but them.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { Layer, MapRef, Table } from "../protocol.ts";
import { CROSSFADE_MS, newLayerView } from "./layers.ts";

function mapRef(asset: string, gen = "g1"): MapRef {
	return { assetId: asset, gen, width: 4000, height: 3000, tileSize: 512, maxZoom: 3 };
}

function layer(id: string, map: MapRef | null): Layer {
	return { id, name: id, map, fogEnabled: false, fogPrefill: true };
}

function table(active: string, layers: Layer[]): Table {
	return { layers, activeLayer: active, grid: grid(), pawnLabels: "default", playersCanDraw: true };
}

function grid(): Table["grid"] {
	return {
		lines: "solid", cellSize: 64, offsetX: 0, offsetY: 0,
		color: "#000000FF", snap: "cells", feetPerCell: 5, diagonals: "equal",
	};
}

const ground = layer("ground", mapRef("m-ground"));
const cellar = layer("cellar", mapRef("m-cellar"));
const twoFloors = table("ground", [ground, cellar]);

test("a fresh view is on the active layer", () => {
	const view = newLayerView(true);
	view.update(twoFloors, 0);

	assert.equal(view.viewed()?.id, "ground");
	assert.ok(view.following());
});

test("the GM can look at another floor and is told they are", () => {
	const view = newLayerView(true);
	view.update(twoFloors, 0);

	view.choose("cellar");
	view.update(twoFloors, 1000);

	assert.equal(view.viewed()?.id, "cellar");
	assert.equal(view.following(), false);
});

// THE PROJECTION IS WHY. A player's state contains only the active layer's
// pawns -- the rest were removed server-side before the event was encoded -- so
// a player "viewing" another floor would be looking at an empty room.
test("a player cannot look anywhere but the active layer", () => {
	const view = newLayerView(false);
	view.update(twoFloors, 0);

	view.choose("cellar");
	view.update(twoFloors, 1000);

	assert.equal(view.viewed()?.id, "ground");
	assert.ok(view.following());
});

// A GM who has wandered off to the cellar and then makes the first floor active
// means to be looking at the first floor. Leaving them where they were would
// put them one command behind their own table.
test("the view snaps back when the active layer moves", () => {
	const view = newLayerView(true);
	view.update(twoFloors, 0);
	view.choose("cellar");
	view.update(twoFloors, 100);

	assert.equal(view.viewed()?.id, "cellar");

	view.update(table("cellar", [ground, cellar]), 200);
	assert.equal(view.viewed()?.id, "cellar", "the cellar became active");
	assert.ok(view.following());

	// And the override is gone, so the next change is followed too.
	view.update(table("ground", [ground, cellar]), 300);
	assert.equal(view.viewed()?.id, "ground");
	assert.ok(view.following());
});

test("a GM viewing a floor that is deleted falls back to the active one", () => {
	const view = newLayerView(true);
	view.update(twoFloors, 0);
	view.choose("cellar");
	view.update(twoFloors, 100);

	view.update(table("ground", [ground]), 200);

	assert.equal(view.viewed()?.id, "ground");
});

// Choosing the layer that is already active is the same as following it, so a
// GM who picks their way back is following again rather than pinned to a value
// that happens to match.
test("choosing the active layer is following it", () => {
	const view = newLayerView(true);
	view.update(twoFloors, 0);
	view.choose("cellar");
	view.update(twoFloors, 100);
	view.choose("ground");
	view.update(twoFloors, 200);

	assert.ok(view.following());

	view.update(table("cellar", [ground, cellar]), 300);
	assert.equal(view.viewed()?.id, "cellar");
});

// THE FIRST MAP OF THE SESSION ARRIVES OVER AN EMPTY TABLE. Fading that in from
// nothing is a quarter second of blank screen for no reason, so a fade needs
// two things to cross.
test("the first map appears at once rather than fading in", () => {
	const view = newLayerView(true);
	view.update(twoFloors, 0);

	const drawn = view.draws();
	assert.equal(drawn.length, 1);
	assert.equal(drawn[0].alpha, 1);
	assert.equal(view.fading(), false);
});

// THE OUTGOING MAP STAYS OPAQUE. Complementary alphas over a cleared buffer
// leave a quarter of the table colour showing at the midpoint, so the
// transition dips through the empty desk and back; painting the old one solid
// and dissolving the new one over it is exactly lerp(old, new, t).
test("switching floors dissolves the new map over the old one", () => {
	const view = newLayerView(true);
	view.update(twoFloors, 0);

	view.choose("cellar");
	view.update(twoFloors, 1000);

	const half = 1000 + CROSSFADE_MS / 2;
	view.update(twoFloors, half);

	const drawn = view.draws();
	assert.equal(drawn.length, 2, "both maps should be painted mid-fade");
	assert.equal(drawn[0].map.assetId, "m-ground", "the outgoing map is painted first, underneath");
	assert.equal(drawn[0].alpha, 1, "the outgoing map must not let the background through");
	assert.equal(drawn[1].map.assetId, "m-cellar");
	assert.ok(Math.abs(drawn[1].alpha - 0.5) < 1e-9, "the incoming map should be halfway in");
	assert.ok(view.fading());
});

test("the fade ends and leaves only the new map", () => {
	const view = newLayerView(true);
	view.update(twoFloors, 0);
	view.choose("cellar");
	view.update(twoFloors, 1000);
	view.update(twoFloors, 1000 + CROSSFADE_MS);

	const drawn = view.draws();
	assert.equal(drawn.length, 1);
	assert.equal(drawn[0].map.assetId, "m-cellar");
	assert.equal(drawn[0].alpha, 1);

	// fading() going false is what lets the frame loop go idle again.
	assert.equal(view.fading(), false);
});

// Two floors sharing one map is not a change anybody should see. What is
// compared is the asset and its generation, which is exactly what decides the
// tile URLs.
test("switching to a floor with the same map does not fade", () => {
	const attic = layer("attic", mapRef("m-ground"));
	const shared = table("ground", [ground, attic]);

	const view = newLayerView(true);
	view.update(shared, 0);
	view.choose("attic");
	view.update(shared, 1000);

	assert.equal(view.fading(), false);
	assert.equal(view.draws().length, 1);
});

// Re-tiling a map completes into a NEW generation at new URLs, so the picture
// underneath really is changing even though the layer and the asset have not.
test("a re-tiled map fades even though the layer did not change", () => {
	const view = newLayerView(true);
	view.update(twoFloors, 0);

	const retiled = table("ground", [layer("ground", mapRef("m-ground", "g2")), cellar]);
	view.update(retiled, 1000);

	assert.ok(view.fading());
	assert.equal(view.draws().length, 2);
});

test("a layer with no map paints nothing at all", () => {
	const bare = table("bare", [layer("bare", null)]);

	const view = newLayerView(true);
	view.update(bare, 0);

	assert.equal(view.draws().length, 0);
	assert.equal(view.viewed()?.id, "bare");
});

// The array draws() hands back is reused, which is what keeps a frame free of
// allocation. A caller that kept it would be holding last frame's answer.
test("draws reuses its array", () => {
	const view = newLayerView(true);
	view.update(twoFloors, 0);

	assert.equal(view.draws(), view.draws());
});
