




import assert from "node:assert/strict";
import { test } from "node:test";

import { evaluate } from "./hp.ts";

test("a plain number is itself, whatever is in the box", () => {
	assert.equal(evaluate("7", "23"), 7);
	assert.equal(evaluate("0", "23"), 0);
});

test("a leading sign counts from what the pawn has", () => {
	assert.equal(evaluate("-7", "23"), 16);
	assert.equal(evaluate("+5", "23"), 28);
});



test("a sum typed onto the end of the value is worked out", () => {
	assert.equal(evaluate("23-7", "23"), 16);
	assert.equal(evaluate("23-7-4", "23"), 12);
	assert.equal(evaluate("23+5-8+2", "23"), 22);
});

test("a chain behind a leading sign is added to the current value", () => {
	assert.equal(evaluate("-7-4", "23"), 12);
	assert.equal(evaluate("+5+5", "0"), 10);
});



test("the terms are added left to right", () => {
	assert.equal(evaluate("1-2-3", ""), -4);
});

test("spaces are how people type and are not an error", () => {
	assert.equal(evaluate(" 23 - 7 ", "23"), 16);
});



test("an empty entry resolves to nothing", () => {
	assert.equal(evaluate("", "23"), null);
	assert.equal(evaluate("   ", "23"), null);
});




test("anything that is not a sum is left alone", () => {
	for (const bad of ["ten", "2d6", "7.5", "1/2", "23-", "-", "+-4", "5**2", "23 7"]) {
		assert.equal(evaluate(bad, "23"), null, bad);
	}
});




test("an entry longer than a hit-point total could be is refused", () => {
	assert.equal(evaluate("1".repeat(25), "23"), null);
	assert.equal(evaluate("1".repeat(24), "23"), Number("1".repeat(24)));
});



test("a relative entry against an empty box counts from zero", () => {
	assert.equal(evaluate("+5", ""), 5);
	assert.equal(evaluate("-5", ""), -5);
});
