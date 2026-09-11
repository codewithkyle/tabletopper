import assert from "node:assert/strict";
import { test } from "node:test";
import { parseColor } from "./grid-pass.ts";
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
