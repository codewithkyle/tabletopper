


import assert from "node:assert/strict";
import { test } from "node:test";

import type { HPBand } from "../protocol.ts";
import { AURA_GOLD, auraColor, auraTurn } from "./aura-pass.ts";
import { BLOOD_FRESH } from "./wounds.ts";



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
