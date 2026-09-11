import assert from "node:assert/strict";
import { test } from "node:test";
import { CONDITION_RINGS_MAX, RING_GAP, RING_WIDTH, ringRadius } from "./rings.ts";
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
