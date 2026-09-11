import assert from "node:assert/strict";
import { test } from "node:test";
import { auraTurn } from "./aura-pass.ts";
test("the phase is always somewhere in one revolution", () => {
	for (const now of [0, 1, 17.5, 999, 6000, 123456, 4.2e9]) {
		const turn = auraTurn(now);
		assert.ok(turn >= 0 && turn < 1, `${now} gave ${turn}`);
	}
});
test("it starts where it ends and goes round in between", () => {
	assert.equal(auraTurn(0), 0);
	const period = 1 / auraTurn(1);
	const marks = [0.25, 0.5, 0.75].map((at) => auraTurn(at * period));
	assert.deepEqual(marks, [0.25, 0.5, 0.75]);
});
test("one revolution later is the same place", () => {
	const period = 1 / auraTurn(1);
	for (const now of [0, 250, 3333.5]) {
		assert.ok(Math.abs(auraTurn(now + period) - auraTurn(now)) < 1e-9, `${now}`);
	}
});
