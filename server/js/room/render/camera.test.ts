// The camera's arithmetic, which is the part of the renderer that can be wrong
// without looking wrong: a projection that is off by half a viewport still
// draws a map, and the symptom is that the pointer does not land where the
// cursor is.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { Camera, Point, Viewport } from "./camera.ts";
import {
	ZOOM_MAX,
	ZOOM_MIN,
	clampToMap,
	clipMatrix,
	fit,
	inverseClipMatrix,
	newCamera,
	panBy,
	screenToWorld,
	visibleRect,
	worldToScreen,
	zoomAt,
	zoomTo,
} from "./camera.ts";

const point = (): Point => ({ x: 0, y: 0 });
const viewport = (width: number, height: number): Viewport => ({ width, height });
const camera = (x: number, y: number, zoom: number): Camera => ({ x, y, zoom });

test("a point survives the round trip through the screen at every zoom", () => {
	const vp = viewport(1280, 720);
	const out = point();

	for (const zoom of [ZOOM_MIN, 0.3, 1, 1.75, ZOOM_MAX]) {
		const cam = camera(4000, 3000, zoom);

		for (const [wx, wy] of [[0, 0], [4000, 3000], [11999, 8999], [-500, 12000]]) {
			worldToScreen(cam, vp, wx, wy, out);
			const screenX = out.x;
			const screenY = out.y;

			screenToWorld(cam, vp, screenX, screenY, out);

			assert.ok(Math.abs(out.x - wx) < 1e-6, `x ${out.x} != ${wx} at zoom ${zoom}`);
			assert.ok(Math.abs(out.y - wy) < 1e-6, `y ${out.y} != ${wy} at zoom ${zoom}`);
		}
	}
});

test("the camera position is the map pixel in the middle of the viewport", () => {
	const cam = camera(4000, 3000, 0.5);
	const vp = viewport(1280, 720);
	const out = point();

	worldToScreen(cam, vp, 4000, 3000, out);

	assert.equal(out.x, 640);
	assert.equal(out.y, 360);
});

test("zooming at a point leaves that point exactly where it was", () => {
	const vp = viewport(1280, 720);
	const before = point();
	const after = point();

	for (const [sx, sy] of [[0, 0], [17, 640], [640, 360], [1280, 720]]) {
		const cam = camera(4000, 3000, 0.8);
		screenToWorld(cam, vp, sx, sy, before);

		zoomAt(cam, vp, sx, sy, 1.25);
		screenToWorld(cam, vp, sx, sy, after);

		assert.ok(Math.abs(after.x - before.x) < 1e-6, `x drifted by ${after.x - before.x}`);
		assert.ok(Math.abs(after.y - before.y) < 1e-6, `y drifted by ${after.y - before.y}`);
	}
});

// The clamp is applied before the anchor is re-measured, so a zoom refused by
// the limit must not move the camera at all -- a wheel held down at the maximum
// would otherwise walk the map away from under the cursor.
test("a zoom refused by the limit moves nothing", () => {
	const vp = viewport(1280, 720);
	const cam = camera(4000, 3000, ZOOM_MAX);

	zoomAt(cam, vp, 100, 100, 4);

	assert.deepEqual(cam, camera(4000, 3000, ZOOM_MAX));
});

test("zoom is clamped at both ends", () => {
	const vp = viewport(1280, 720);

	const out = camera(0, 0, 1);
	zoomAt(out, vp, 0, 0, 1000);
	assert.equal(out.zoom, ZOOM_MAX);

	const inward = camera(0, 0, 1);
	zoomAt(inward, vp, 0, 0, 0.0001);
	assert.equal(inward.zoom, ZOOM_MIN);
});

test("zoomTo sets an absolute zoom about the middle", () => {
	const vp = viewport(1280, 720);
	const cam = camera(4000, 3000, 0.37);

	zoomTo(cam, vp, 2);

	assert.equal(cam.zoom, 2);
	assert.equal(cam.x, 4000);
	assert.equal(cam.y, 3000);
});

test("panning is measured in screen pixels and divided by the zoom", () => {
	const cam = camera(1000, 1000, 0.5);

	panBy(cam, 100, -50);

	assert.equal(cam.x, 800);
	assert.equal(cam.y, 1100);
});

test("the visible rectangle is the viewport in map pixels", () => {
	const cam = camera(4000, 3000, 0.5);
	const rect = { x1: 0, y1: 0, x2: 0, y2: 0 };

	visibleRect(cam, viewport(1280, 720), rect);

	assert.deepEqual(rect, { x1: 2720, y1: 2280, x2: 5280, y2: 3720 });
});

test("fit centres the map and shows all of it with a margin", () => {
	const cam = newCamera();
	const vp = viewport(1280, 720);

	fit(cam, vp, 12000, 9000);

	assert.equal(cam.x, 6000);
	assert.equal(cam.y, 4500);
	assert.ok(cam.zoom * 12000 <= vp.width, "the map is wider than the viewport");
	assert.ok(cam.zoom * 9000 <= vp.height, "the map is taller than the viewport");
	assert.ok(cam.zoom * 9000 > vp.height * 0.85, "the margin swallowed the map");
});

test("fit on a viewport or a map with no size does nothing", () => {
	const cam = camera(1, 2, 3);

	fit(cam, viewport(1280, 720), 0, 9000);
	fit(cam, viewport(0, 0), 12000, 9000);

	assert.deepEqual(cam, camera(1, 2, 3));
});

test("the middle of the screen cannot leave the map", () => {
	const vp = viewport(1280, 720);

	const past = camera(20000, -400, 1);
	clampToMap(past, vp, 12000, 9000);
	assert.equal(past.x, 12000);
	assert.equal(past.y, 0);

	const inside = camera(6000, 4500, 1);
	clampToMap(inside, vp, 12000, 9000);
	assert.deepEqual(inside, camera(6000, 4500, 1));
});

test("a map smaller than the viewport is locked centred", () => {
	const cam = camera(9000, 200, 0.05);

	clampToMap(cam, viewport(1280, 720), 12000, 9000);

	assert.equal(cam.x, 6000);
	assert.equal(cam.y, 4500);
});

test("clampToMap does nothing when there is no map", () => {
	const cam = camera(9000, 200, 0.5);

	clampToMap(cam, viewport(1280, 720), 0, 0);

	assert.deepEqual(cam, camera(9000, 200, 0.5));
});

// The two matrices are what the shaders actually receive, and they are each
// other's inverse: the tile pass goes map to clip, the grid pass goes clip to
// map, and a sign wrong in either draws a mirrored table.
test("the clip matrix agrees with worldToScreen", () => {
	const cam = camera(4000, 3000, 0.5);
	const vp = viewport(1280, 720);
	const dpr = 2;
	const out = point();

	const m = clipMatrix(cam, vp.width * dpr, vp.height * dpr, dpr, new Float32Array(9));

	for (const [wx, wy] of [[0, 0], [4000, 3000], [12000, 9000]]) {
		const clipX = m[0] * wx + m[6];
		const clipY = m[4] * wy + m[7];

		worldToScreen(cam, vp, wx, wy, out);

		assert.ok(Math.abs(clipX - ((out.x / vp.width) * 2 - 1)) < 1e-5, "x disagrees");
		assert.ok(Math.abs(clipY - (1 - (out.y / vp.height) * 2)) < 1e-5, "y disagrees");
	}
});

test("the inverse clip matrix undoes the clip matrix", () => {
	const cam = camera(4000, 3000, 0.37);
	const forward = clipMatrix(cam, 2560, 1440, 2, new Float32Array(9));
	const back = inverseClipMatrix(cam, 2560, 1440, 2, new Float32Array(9));

	for (const [wx, wy] of [[0, 0], [1234, 5678], [12000, 9000]]) {
		const clipX = forward[0] * wx + forward[6];
		const clipY = forward[4] * wy + forward[7];

		assert.ok(Math.abs(back[0] * clipX + back[6] - wx) < 1e-3, "x did not come back");
		assert.ok(Math.abs(back[4] * clipY + back[7] - wy) < 1e-3, "y did not come back");
	}
});
