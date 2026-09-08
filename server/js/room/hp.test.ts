// The hit-point box's arithmetic. Every case here has a twin in
// TestTheHitPointBoxTakesASum, because the server evaluates the same strings
// and the two answering differently is a number that changes when the panel
// refetches.

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

// This is the entry the field is built for: the current value is already there
// and the damage is typed on the end of it.
test("a sum typed onto the end of the value is worked out", () => {
	assert.equal(evaluate("23-7", "23"), 16);
	assert.equal(evaluate("23-7-4", "23"), 12);
	assert.equal(evaluate("23+5-8+2", "23"), 22);
});

test("a chain behind a leading sign is added to the current value", () => {
	assert.equal(evaluate("-7-4", "23"), 12);
	assert.equal(evaluate("+5+5", "0"), 10);
});

// Left to right and nothing else. There is no precedence to get wrong when the
// only operators are plus and minus, which is the whole of what a table does.
test("the terms are added left to right", () => {
	assert.equal(evaluate("1-2-3", ""), -4);
});

test("spaces are how people type and are not an error", () => {
	assert.equal(evaluate(" 23 - 7 ", "23"), 16);
});

// An empty box is not a change. Blurring a field somebody has cleared must not
// set the pawn to zero.
test("an empty entry resolves to nothing", () => {
	assert.equal(evaluate("", "23"), null);
	assert.equal(evaluate("   ", "23"), null);
});

// Anything that is not a sum is left in the field as it was typed, so the
// server answers with a message about it rather than this discarding what
// somebody meant.
test("anything that is not a sum is left alone", () => {
	for (const bad of ["ten", "2d6", "7.5", "1/2", "23-", "-", "+-4", "5**2", "23 7"]) {
		assert.equal(evaluate(bad, "23"), null, bad);
	}
});

// A pasted essay is refused as an entry rather than summed a character at a
// time. The bound is on the arithmetic; what a legal hit-point total is remains
// the core's question.
test("an entry longer than a hit-point total could be is refused", () => {
	assert.equal(evaluate("1".repeat(25), "23"), null);
	assert.equal(evaluate("1".repeat(24), "23"), Number("1".repeat(24)));
});

// A box the viewer was given no number for still takes an entry, and a relative
// one counts from nothing rather than from NaN.
test("a relative entry against an empty box counts from zero", () => {
	assert.equal(evaluate("+5", ""), 5);
	assert.equal(evaluate("-5", ""), -5);
});
