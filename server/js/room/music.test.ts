import assert from "node:assert/strict";
import { test } from "node:test";
import { clampVolume, clock } from "./music.ts";
test("a running time reads as minutes and seconds", () => {
	assert.equal(clock(0), "0:00");
	assert.equal(clock(9), "0:09");
	assert.equal(clock(61.7), "1:01");
	assert.equal(clock(605), "10:05");
	assert.equal(clock(3600), "60:00");
});
test("a time nobody can play is still a time somebody can read", () => {
	assert.equal(clock(Number.NaN), "0:00");
	assert.equal(clock(Number.POSITIVE_INFINITY), "0:00");
	assert.equal(clock(-4), "0:00");
});
test("a volume outside the slider is pulled back onto it", () => {
	assert.equal(clampVolume(50), 50);
	assert.equal(clampVolume(-10), 0);
	assert.equal(clampVolume(140), 100);
	assert.equal(clampVolume(37.6), 38);
});
test("a volume that was never stored is full volume", () => {
	assert.equal(clampVolume(Number.NaN), 100);
});
