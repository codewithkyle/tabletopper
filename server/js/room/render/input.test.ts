



import assert from "node:assert/strict";
import { test } from "node:test";

import type { Camera, Viewport } from "./camera.ts";
import { apply, newPending, wheelMultiplier } from "./input.ts";

const vp: Viewport = { width: 1280, height: 720 };
const camera = (x: number, y: number, zoom: number): Camera => ({ x, y, zoom });

test("a wheel notch zooms in when it scrolls up and out when it scrolls down", () => {
	assert.ok(wheelMultiplier(-100, 0, false) > 1);
	assert.ok(wheelMultiplier(100, 0, false) < 1);
});




test("a notch out is exactly the reciprocal of a notch in", () => {
	for (const delta of [1, 40, 100, 3000]) {
		const inward = wheelMultiplier(-delta, 0, false);
		const outward = wheelMultiplier(delta, 0, false);

		assert.ok(Math.abs(inward * outward - 1) < 1e-12, `${delta} is not symmetric`);
	}
});

test("no wheel event may move the zoom by more than a quarter", () => {
	for (const delta of [1e3, 1e6]) {
		assert.ok(wheelMultiplier(-delta, 0, false) <= 1.25 + 1e-12);
		assert.ok(wheelMultiplier(delta, 0, false) >= 1 / 1.25 - 1e-12);
	}
});

test("line and page deltas are larger than the same number of pixels", () => {
	const pixels = wheelMultiplier(3, 0, false);
	const lines = wheelMultiplier(3, 1, false);
	const pages = wheelMultiplier(3, 2, false);

	assert.ok(lines < pixels, "a line should move more than a pixel");
	assert.ok(pages < lines, "a page should move more than a line");
});



test("a trackpad pinch moves further than the same delta on a wheel", () => {
	assert.ok(wheelMultiplier(-4, 0, true) > wheelMultiplier(-4, 0, false));
});

test("an empty accumulator does nothing and says so", () => {
	const pending = newPending();
	const cam = camera(100, 200, 1);

	assert.equal(apply(pending, cam, vp), false);
	assert.deepEqual(cam, camera(100, 200, 1));
});

test("applying empties the accumulator so the next frame starts clean", () => {
	const pending = newPending();
	pending.panX = 40;
	pending.zoom = 1.1;

	const cam = camera(100, 200, 1);
	assert.equal(apply(pending, cam, vp), true);

	assert.equal(pending.panX, 0);
	assert.equal(pending.panY, 0);
	assert.equal(pending.zoom, 1);
	assert.equal(apply(pending, cam, vp), false);
});

test("a hundred small drags cost the same as one big one", () => {
	const many = newPending();
	for (let i = 0; i < 100; i++) {
		many.panX += 1;
		many.panY -= 0.5;
	}

	const once = newPending();
	once.panX = 100;
	once.panY = -50;

	const a = camera(0, 0, 0.5);
	const b = camera(0, 0, 0.5);
	apply(many, a, vp);
	apply(once, b, vp);

	assert.deepEqual(a, b);
});




test("a pinch pans at the old zoom and then zooms at the new anchor", () => {
	const pending = newPending();
	pending.panX = 64;
	pending.zoom = 2;
	pending.zoomX = 640;
	pending.zoomY = 360;

	const cam = camera(1000, 1000, 1);
	apply(pending, cam, vp);

	assert.equal(cam.zoom, 2);
	assert.equal(cam.x, 936);
	assert.equal(cam.y, 1000);
});
