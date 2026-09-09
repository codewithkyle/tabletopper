// THE BAND TABLE BELOW IS COPIED FROM THE GO, deliberately and line for line.
// It is TestTheHealthBandsSitWhereTheyAreDescribed in
// internal/room/projection_test.go, and the two exist as a pair: a GM is sent
// hit points and works the band out here, a player is sent the band already
// worked out there, and the moment those disagree the same creature is bloody
// to one of them and bruised to the other.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { HPBand } from "../protocol.ts";
import { BEAT_PERIOD, bandOf, echo, healthOf, heartbeat, splatters, woundRing, worsened } from "./wounds.ts";

test("the bands sit exactly where the server puts them", () => {
	const cases: [number, number, HPBand][] = [
		// A maximum of 100, where every boundary is a whole number and every one
		// of them is ON the lower band: three quarters of a hundred is bruised,
		// not healthy.
		[100, 100, "healthy"],
		[76, 100, "healthy"],
		[75, 100, "bruised"],
		[51, 100, "bruised"],
		[50, 100, "bloody"],
		[26, 100, "bloody"],
		[25, 100, "veryBloody"],
		[6, 100, "veryBloody"],
		[5, 100, "nearDeath"],
		[1, 100, "nearDeath"],
		[0, 100, "dead"],

		// A maximum of 20, where a twentieth is one hit point: a goblin is near
		// death at exactly 1 and very bloody at 2.
		[20, 20, "healthy"],
		[16, 20, "healthy"],
		[15, 20, "bruised"],
		[10, 20, "bloody"],
		[5, 20, "veryBloody"],
		[2, 20, "veryBloody"],
		[1, 20, "nearDeath"],
		[0, 20, "dead"],

		// A maximum of 7: three quarters is 5.25, a half is 3.5 and a quarter is
		// 1.75, so every boundary falls between whole numbers.
		[7, 7, "healthy"],
		[6, 7, "healthy"],
		[5, 7, "bruised"],
		[4, 7, "bruised"],
		[3, 7, "bloody"],
		[2, 7, "bloody"],
		[1, 7, "veryBloody"],

		// A creature already below the last cut is still not dead until it is at
		// zero, which is the whole reason the two words are separate.
		[1, 1000, "nearDeath"],
		[0, 1000, "dead"],

		// Overhealed past its own maximum, which a temporary hit point pool or a
		// GM raising the maximum after the fact both produce.
		[30, 20, "healthy"],
	];

	for (const [hp, maxHp, want] of cases) {
		assert.equal(bandOf(hp, maxHp), want, `${hp} of ${maxHp} hit points`);
	}

	assert.equal(bandOf(null, 10), null, "a pawn with no hit points was given a band");
});

// THE ONE PLACE THIS DIVERGES FROM THE GO, and it is the behaviour the skull has
// always had. hpBand answers nil without a maximum because a projection with no
// maximum has no band to send; here, a creature at or below zero is dead whether
// or not anybody wrote down what it started with.
test("a creature at zero is dead with or without a maximum", () => {
	assert.equal(bandOf(0, null), "dead");
	assert.equal(bandOf(-4, null), "dead");

	// Above zero and with nothing to measure against, there is still no band --
	// "3 hit points" is not a fraction of anything.
	assert.equal(bandOf(3, null), null, "a pawn with no maximum was given a band");
	assert.equal(bandOf(3, 0), null);
});

test("the number wins over the band, and neither is invented", () => {
	assert.equal(healthOf({ hp: 4, maxHp: 7, hpBand: "dead" }), "bruised");
	assert.equal(healthOf({ hp: null, maxHp: null, hpBand: "bloody" }), "bloody");

	// A room with its labels off sends no number and no band. Nothing downstream
	// may guess: no skull, no ring, no blood.
	assert.equal(healthOf({ hp: null, maxHp: null, hpBand: null }), null);
});

test("worsening is one way", () => {
	assert.equal(worsened("healthy", "bloody"), true);
	assert.equal(worsened("nearDeath", "dead"), true);
	assert.equal(worsened("bloody", "healthy"), false, "being healed shed blood");
	assert.equal(worsened("dead", "dead"), false, "standing still shed blood");
});

// THE MISSING ENTRIES ARE THE FEATURE. A table where every creature carries a
// red ring is a table with no signal in it, so nothing above the halfway line is
// marked at all -- and a corpse is not either, because the skull and the
// desaturated draw already say more than a ring could.
test("only a creature in trouble is ringed, and only near death beats", () => {
	assert.equal(woundRing(null), null);
	assert.equal(woundRing("healthy"), null);
	assert.equal(woundRing("bruised"), null, "a scratch drew a wound ring");
	assert.equal(woundRing("dead"), null, "a corpse drew a wound ring");

	assert.equal(woundRing("bloody")?.beats, false);
	assert.equal(woundRing("veryBloody")?.beats, false);
	assert.equal(woundRing("nearDeath")?.beats, true);

	// It gets louder on the way down and never quieter.
	const weights = (["bloody", "veryBloody", "nearDeath"] as const).map((band) => woundRing(band)?.thickness ?? 0);
	assert.deepEqual(weights, [...weights].sort((a, b) => a - b));
});

// The same line the ring draws: the first blood on the floor and the first ring
// round the pawn are the same moment.
test("nothing above the halfway line sheds blood", () => {
	assert.equal(splatters("healthy"), 0);
	assert.equal(splatters("bruised"), 0);

	assert.ok(splatters("bloody") > 0);
	assert.ok(splatters("dead") > splatters("bloody"), "dying threw no more than a scratch");
});

// IT IS TWO THUMPS AND A LONG REST, NOT A SINE. A sine reads as "selected", or
// as something loading; every interface the reader has ever used pulses that
// way. What makes this a heart is the silence between beats, so that is what is
// pinned here rather than the shape of the thumps.
test("the heartbeat is two knocks and a silence", () => {
	let silent = 0;
	let peaks = 0;

	for (let t = 0; t < BEAT_PERIOD; t++) {
		const value = heartbeat(t);

		assert.ok(value >= 0 && value <= 1, `envelope left its range at ${t}ms`);

		if (value === 0) {
			silent++;
		}
		if (value > heartbeat(t - 1) && value >= heartbeat(t + 1)) {
			peaks++;
		}
	}

	assert.equal(peaks, 2, "a heart that does not go lub-dub is a pulsing ring");
	assert.ok(silent / BEAT_PERIOD > 0.55, `only ${silent}ms of ${BEAT_PERIOD}ms was rest`);

	// And it repeats rather than drifting, at whatever time the page has been
	// open for -- performance.now() is milliseconds since the tab loaded, and
	// the beat has to look the same an hour in.
	assert.equal(heartbeat(50), heartbeat(50 + BEAT_PERIOD * 3600));
});

test("the echo leaves once a beat and fades on the way out", () => {
	const first = echo(0);
	assert.ok(first);
	assert.equal(first.grow, 0, "the ring did not start at the pawn's own edge");

	const later = echo(300);
	assert.ok(later);
	assert.ok(later.grow > first.grow, "the ring did not travel");
	assert.ok(later.alpha < first.alpha, "the ring did not fade");

	// It is gone well before the next beat, which is what keeps two rings from
	// being in flight at once -- that reads as a ripple in water rather than as
	// a heart.
	assert.equal(echo(BEAT_PERIOD - 1), null);
});
