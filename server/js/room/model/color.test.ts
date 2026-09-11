import assert from "node:assert/strict";
import { test } from "node:test";
import type { HPBand } from "../protocol.ts";
import { AURA_GOLD, BLOOD_FRESH, auraColor, parseColor } from "./color.ts";
const read = (value: string): number[] => Array.from(parseColor(value, new Float32Array(4)));
test("six digits are opaque", () => {
	assert.deepEqual(read("#ffffff"), [1, 1, 1, 1]);
	assert.deepEqual(read("#000000"), [0, 0, 0, 1]);
});
test("eight digits carry their own alpha", () => {
	assert.deepEqual(read("#00000000"), [0, 0, 0, 0]);
	assert.deepEqual(read("#ff0000ff"), [1, 0, 0, 1]);
});
test("the leading hash is optional and case does not matter", () => {
	assert.deepEqual(read("FF8800"), read("#ff8800"));
});
test("anything unreadable falls back to opaque black rather than to nothing", () => {
	for (const bad of ["", "#fff", "#gggggg", "rgb(1,2,3)", "#1234567"]) {
		assert.deepEqual(read(bad), [0, 0, 0, 1], bad);
	}
});
test("a creature that is not bleeding is gold", () => {
	for (const band of [null, "healthy", "bruised"] as (HPBand | null)[]) {
		assert.equal(auraColor(band), AURA_GOLD, `${band}`);
	}
});
test("a bleeding creature is drawn in its own blood", () => {
	for (const band of ["bloody", "veryBloody", "nearDeath"] as HPBand[]) {
		assert.equal(auraColor(band), BLOOD_FRESH, band);
	}
});
test("a corpse still on the count is gold like anybody else", () => {
	assert.equal(auraColor("dead"), AURA_GOLD);
});
test("the gold is the one the tails are painted in", () => {
	assert.deepEqual(Array.from(AURA_GOLD), [1, 0.78, 0.35]);
});
