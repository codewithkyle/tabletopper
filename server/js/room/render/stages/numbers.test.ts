import assert from "node:assert/strict";
import { test } from "node:test";
import { inkFor } from "./numbers.ts";
test("a cell number is the colour the GM gave the grid", () => {
	const ink = inkFor("#3355FFCC");
	assert.deepEqual(
		ink.ink.map((channel) => Math.round(channel * 255)),
		[0x33, 0x55, 0xFF],
	);
});
test("a number is as faint as the grid it belongs to", () => {
	assert.equal(Math.round(inkFor("#3355FFCC").alpha * 255), 0xCC);
	assert.equal(inkFor("#000000FF").alpha, 1);
});
test("a grid colour written without an alpha channel is opaque", () => {
	assert.equal(inkFor("#3355FF").alpha, 1);
});
