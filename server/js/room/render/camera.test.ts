import assert from "node:assert/strict";
import { test } from "node:test";
import type { Camera, Point, Viewport } from "./camera.ts";
import {
	ZOOM_MAX,
	ZOOM_MIN,
	clampToMap,
	clipMatrix,
	fit,
	focusTarget,
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
test("a followed box is centred whatever shape it is", () => {
	const vp = viewport(1280, 720);
	const out = newCamera();
	focusTarget(camera(0, 0, 1), vp, { x1: 100, y1: 200, x2: 200, y2: 260 }, out);
	assert.equal(out.x, 150);
	assert.equal(out.y, 230);
});
test("a box that already fits keeps the viewer's zoom", () => {
	const vp = viewport(1280, 720);
	const out = newCamera();
	for (const zoom of [ZOOM_MIN, 0.4, 1, 2.5, ZOOM_MAX]) {
		focusTarget(camera(0, 0, zoom), vp, { x1: 0, y1: 0, x2: 70, y2: 70 }, out);
		assert.equal(out.zoom, zoom, `a single token moved the zoom at ${zoom}`);
	}
});
test("a box too big for the screen pulls the zoom out until it fits", () => {
	const vp = viewport(1280, 720);
	const out = newCamera();
	focusTarget(camera(0, 0, 1), vp, { x1: 0, y1: 0, x2: 2400, y2: 1800 }, out);
	assert.ok(out.zoom < 1, "the zoom did not give");
	assert.ok(out.zoom * 2400 <= vp.width, "the group is wider than the viewport");
	assert.ok(out.zoom * 1800 <= vp.height, "the group is taller than the viewport");
});
test("a box pulled out to fit is not pulled to the very edges", () => {
	const vp = viewport(1280, 720);
	const out = newCamera();
	focusTarget(camera(0, 0, 1), vp, { x1: 0, y1: 0, x2: 2400, y2: 1800 }, out);
	assert.ok(out.zoom * 1800 < vp.height * 0.9, "the group touches the top and bottom of the screen");
});
test("following never zooms in", () => {
	const vp = viewport(1280, 720);
	const out = newCamera();
	focusTarget(camera(0, 0, 0.2), vp, { x1: 4000, y1: 4000, x2: 4070, y2: 4070 }, out);
	assert.equal(out.zoom, 0.2);
	assert.equal(out.x, 4035);
	assert.equal(out.y, 4035);
});
test("a box with no size still gives a camera", () => {
	const vp = viewport(1280, 720);
	const out = newCamera();
	focusTarget(camera(0, 0, 1), vp, { x1: 500, y1: 500, x2: 500, y2: 500 }, out);
	assert.deepEqual(out, camera(500, 500, 1));
});
test("a box bigger than the zoom range stops at the limit", () => {
	const vp = viewport(1280, 720);
	const out = newCamera();
	focusTarget(camera(0, 0, 1), vp, { x1: 0, y1: 0, x2: 1_000_000, y2: 1_000_000 }, out);
	assert.equal(out.zoom, ZOOM_MIN);
});
test("the middle of the screen cannot leave the map while the map is the bigger one", () => {
	const vp = viewport(1280, 720);
	const past = camera(20000, -400, 1);
	clampToMap(past, vp, 12000, 9000);
	assert.equal(past.x, 12000);
	assert.equal(past.y, 0);
	const inside = camera(6000, 4500, 1);
	clampToMap(inside, vp, 12000, 9000);
	assert.deepEqual(inside, camera(6000, 4500, 1));
});
test("a map smaller than the viewport can still be panned around", () => {
	const vp = viewport(1280, 720);
	const cam = camera(6000 + 4000, 4500 - 2000, 0.05);
	clampToMap(cam, vp, 12000, 9000);
	assert.deepEqual(cam, camera(10000, 2500, 0.05), "a pan well inside the bound was refused");
});
test("a map smaller than the viewport stops before it leaves the screen", () => {
	const vp = viewport(1280, 720);
	const far = camera(1e6, -1e6, 0.05);
	clampToMap(far, vp, 12000, 9000);
	assert.equal(far.x, 6000 + 12800);
	assert.equal(far.y, 4500 - 7200);
});
test("clampToMap does nothing when there is no map", () => {
	const cam = camera(9000, 200, 0.5);
	clampToMap(cam, viewport(1280, 720), 0, 0);
	assert.deepEqual(cam, camera(9000, 200, 0.5));
});
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
