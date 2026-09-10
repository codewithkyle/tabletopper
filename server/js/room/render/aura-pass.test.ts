// The two questions the aura asks that are not a shader: where the tails have
// got to, and what colour they are.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { HPBand } from "../protocol.ts";
import { AURA_GOLD, auraColor, auraTurn } from "./aura-pass.ts";
import { BLOOD_FRESH } from "./wounds.ts";

// The phase is a fraction of one revolution, so anything outside it is an angle
// the shader would read as several turns at once.
test("the phase is always somewhere in one revolution", () => {
	for (const now of [0, 1, 17.5, 999, 6000, 123456, 4.2e9]) {
		const turn = auraTurn(now);

		assert.ok(turn >= 0 && turn < 1, `${now} gave ${turn}`);
	}
});

test("it starts where it ends and goes round in between", () => {
	assert.equal(auraTurn(0), 0);

	// Quarter, half and three quarters of a revolution, in that order, without
	// this test having to know how long a revolution is.
	const period = 1 / auraTurn(1);
	const marks = [0.25, 0.5, 0.75].map((at) => auraTurn(at * period));

	assert.deepEqual(marks, [0.25, 0.5, 0.75]);
});

// IT LOOKS THE SAME AN HOUR IN AS IT DID IN THE FIRST MINUTE, which is what
// makes it a function of the wall clock rather than of when a turn began: two
// creatures on one grouped line turn in step because both read this, and a tab
// left open all evening is still turning at the same rate.
test("one revolution later is the same place", () => {
	const period = 1 / auraTurn(1);

	for (const now of [0, 250, 3333.5]) {
		assert.ok(Math.abs(auraTurn(now + period) - auraTurn(now)) < 1e-9, `${now}`);
	}
});

// NOTHING ABOVE THE HALFWAY LINE CHANGES COLOUR. It is hurt()'s rule and the
// reason a table full of scratched creatures is a table full of gold: an aura
// that went red early would be a warning nobody could act on.
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

// AND A CORPSE IS NOT MARKED EITHER, which is the other end of hurt()'s table.
// It is already grey and already wearing a skull; a red ring would be saying
// something the card has said twice, in a red so dark it cannot be seen on a
// dark map. What a dead line's turn keeps is the gold that means nothing but
// "you are up".
test("a corpse still on the count is gold like anybody else", () => {
	assert.equal(auraColor("dead"), AURA_GOLD);
});

// The one colour in here that is nobody else's: DaisyUI supplies the mechanic
// and this supplies the currentColor it reads. Pinned because it is the sort of
// number that gets nudged and never noticed.
test("the gold is the one the tails are painted in", () => {
	assert.deepEqual(Array.from(AURA_GOLD), [1, 0.78, 0.35]);
});
