// THE BAND TABLE BELOW IS COPIED FROM THE GO, deliberately and line for line.
// It is TestTheHealthBandsSitWhereTheyAreDescribed in
// internal/room/projection_test.go, and the two exist as a pair: a GM is sent
// hit points and works the band out here, a player is sent the band already
// worked out there, and the moment those disagree the same creature is bloody
// to one of them and bruised to the other.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { HPBand } from "../protocol.ts";
import {
	BEAT_HEART, BEAT_NONE, BEAT_PERIOD, BEAT_SLOW, DEATH_SPLATTERS, SEVERITY_UNKNOWN, SLOW_PERIOD,
	bandOf, beats, bleeds, fastBeat, healthOf, heartbeat, hurt, severityOf, slowBeat, splatters,
} from "./wounds.ts";

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

// A HIT IS MEASURED AGAINST THE CREATURE IT LANDED ON and nothing else. This is
// the property the whole floor rests on: the same eight points is a scratch on
// an ogre and most of a goblin, and if these two ever came out equal the marks
// would stop meaning anything.
test("the same damage is worth more to a smaller creature", () => {
	assert.ok(severityOf(8, 7) > severityOf(8, 40));
	assert.ok(severityOf(8, 40) > severityOf(8, 200));

	// And a bigger hit is worth more than a smaller one on the same creature.
	assert.ok(severityOf(20, 40) > severityOf(8, 40));
});

test("a hit that is not a hit is worth nothing", () => {
	assert.equal(severityOf(0, 40), 0, "standing still shed blood");
	assert.equal(severityOf(-5, 40), 0, "being healed shed blood");
});

// NOBODY WROTE A MAXIMUM DOWN, which is a GM counting damage on a monster with
// no stat line. There is no fraction to take, so it draws an ORDINARY hit --
// neither the smallest nor the largest, because both would be a claim about a
// number that does not exist.
test("a hit on a creature with no maximum is an ordinary one", () => {
	assert.equal(severityOf(3, null), SEVERITY_UNKNOWN);
	assert.equal(severityOf(3, 0), SEVERITY_UNKNOWN);

	assert.ok(SEVERITY_UNKNOWN > 0 && SEVERITY_UNKNOWN < 1);
});

// IT NEVER LEAVES ITS RANGE, whatever a GM types. A hit far bigger than the
// creature's maximum is what a critical against a goblin looks like, and it is
// the top of the scale rather than a mark the size of the room.
test("severity is a fraction and stays one", () => {
	assert.equal(severityOf(40, 40), 1);
	assert.equal(severityOf(400, 40), 1);

	for (const [damage, maxHp] of [[1, 1000], [1, 7], [3, 8], [19, 40], [39, 40]]) {
		const value = severityOf(damage, maxHp);
		assert.ok(value > 0 && value <= 1, `${damage} of ${maxHp} left the range at ${value}`);
	}
});

// THE CURVE IS THE POINT OF THE FUNCTION. An ordinary fight is a long run of
// hits under a fifth of a creature's maximum, and read straight every one of
// them would land at the bottom of the scale and draw the same smallest mark.
// The root lifts that band into the middle of what can actually be seen.
test("the ordinary hit is not squashed against the floor of the scale", () => {
	// A d8 against a forty-point monster: twelve per cent, which a linear
	// reading would put a long way below halfway.
	assert.ok(severityOf(5, 40) > 0.3, "the commonest hit in a fight was drawn as nothing");

	// And it is still clearly below what a quarter of the creature is worth,
	// so the scale has not simply been pushed to the top instead.
	assert.ok(severityOf(5, 40) < severityOf(10, 40));
});

// THE MISSING ENTRIES ARE THE FEATURE. A table where every creature looks
// wounded is a table with no signal in it, so nothing above the halfway line is
// marked at all -- and a corpse is not either, because the grey and the skull it
// already carries say more than a red rim could, and a red rim under a full grey
// would be greyed away in the same breath.
test("only a creature in trouble is marked, and a corpse is marked otherwise", () => {
	assert.equal(hurt(null), 0);
	assert.equal(hurt("healthy"), 0);
	assert.equal(hurt("bruised"), 0, "a scratch marked the portrait");
	assert.equal(hurt("dead"), 0, "a corpse was given a red rim under its grey");

	// And it gets worse on the way down, never better.
	const worse = (["bloody", "veryBloody", "nearDeath"] as const).map(hurt);
	assert.deepEqual(worse, [...worse].sort((a, b) => a - b));
	assert.equal(worse[worse.length - 1], 1);
});

// The pulse is a glow on the inside of the rim -- it beats without moving, so it
// stays out of the middle of the portrait where the face is, and it stays inside
// the disc where the creature's own state lives.
test("the pulse starts at very bloodied and becomes a heart at the end", () => {
	assert.equal(beats(null), BEAT_NONE);
	assert.equal(beats("healthy"), BEAT_NONE);
	assert.equal(beats("bruised"), BEAT_NONE);
	assert.equal(beats("bloody"), BEAT_NONE, "a creature at half health had a pulse");

	assert.equal(beats("veryBloody"), BEAT_SLOW);
	assert.equal(beats("nearDeath"), BEAT_HEART);

	// A CORPSE DOES NOT BEAT. It is the only entry here that would be actively
	// wrong rather than merely noisy, and it is the frame loop's floor: a table
	// of dead things has to be able to go quiet.
	assert.equal(beats("dead"), BEAT_NONE, "a corpse had a heartbeat");
});

test("blood only shows on a portrait once the creature is badly hurt", () => {
	assert.equal(bleeds(null), false);
	assert.equal(bleeds("healthy"), false);
	assert.equal(bleeds("bruised"), false);
	assert.equal(bleeds("bloody"), false);

	assert.equal(bleeds("veryBloody"), true);
	assert.equal(bleeds("nearDeath"), true);

	// A corpse keeps its blood while the body under it goes grey, which is the
	// one place the two are deliberately out of step.
	assert.equal(bleeds("dead"), true);
});

// EVERY HIT MARKS THE FLOOR. This is the rule the band table was replaced to
// get: the mark for a scratch is small and faint rather than absent, so the
// floor says a blow landed here at the moment and in the place it landed.
test("every hit throws at least one mark and no hit throws a death", () => {
	for (const severity of [0, 0.01, 0.2, SEVERITY_UNKNOWN, 0.5, 0.79, 0.8, 1]) {
		const count = splatters(severity);

		assert.ok(count >= 1, `a hit at ${severity} threw nothing`);
		assert.ok(count < DEATH_SPLATTERS, `a hit at ${severity} threw as much as a death`);
	}
});

test("a worse hit throws more, and never fewer", () => {
	const counts = [0, 0.25, 0.5, 0.75, 1].map(splatters);

	assert.deepEqual(counts, [...counts].sort((a, b) => a - b));
	assert.ok(counts[counts.length - 1] > counts[0], "the worst hit threw no more than the lightest");
});

// IT IS TWO THUMPS AND A LONG REST, NOT A SINE. A sine reads as "selected", or
// as something loading; every interface the reader has ever used pulses that
// way. What makes this a heart is the silence between beats, so that is what is
// pinned here rather than the shape of the thumps.
test("the heartbeat is two knocks and a silence, at either rate", () => {
	for (const period of [BEAT_PERIOD, SLOW_PERIOD]) {
		let silent = 0;
		let peaks = 0;

		for (let t = 0; t < period; t++) {
			const value = heartbeat(t, period);

			assert.ok(value >= 0 && value <= 1, `envelope left its range at ${t}ms`);

			if (value === 0) {
				silent++;
			}
			if (value > heartbeat(t - 1, period) && value >= heartbeat(t + 1, period)) {
				peaks++;
			}
		}

		assert.equal(peaks, 2, `a heart that does not go lub-dub is a pulsing ring at ${period}ms`);
		assert.ok(silent / period > 0.55, `only ${silent}ms of ${period}ms was rest`);
	}

	// And it repeats rather than drifting, at whatever time the page has been
	// open for -- performance.now() is milliseconds since the tab loaded, and
	// the beat has to look the same an hour in.
	assert.equal(heartbeat(50, BEAT_PERIOD), heartbeat(50 + BEAT_PERIOD * 3600, BEAT_PERIOD));
});

// THE KNOCK IS THE SAME LENGTH AT BOTH RATES and only the rest between beats
// changes, which is what a slower heart actually is. Stretching the thump as
// well would turn it into a swell.
test("a slower heart is a longer rest and not a longer knock", () => {
	const knock = (period: number): number => {
		let moving = 0;
		for (let t = 0; t < period; t++) {
			if (heartbeat(t, period) > 0) {
				moving++;
			}
		}

		return moving;
	};

	assert.equal(knock(SLOW_PERIOD), knock(BEAT_PERIOD));
});

// The two rates have to be told apart at a glance across a table carrying both,
// and they are told apart twice over: a dying creature beats more often AND
// harder than a badly hurt one.
test("a dying creature beats faster and harder than a bloodied one", () => {
	const over = 12_000;

	const started = (beat: (now: number) => number): number => {
		let count = 0;
		for (let t = 1; t < over; t++) {
			if (beat(t) > 0 && beat(t - 1) === 0) {
				count++;
			}
		}

		return count;
	};

	const loudest = (beat: (now: number) => number): number => {
		let peak = 0;
		for (let t = 0; t < over; t++) {
			peak = Math.max(peak, beat(t));
		}

		return peak;
	};

	assert.ok(started(fastBeat) > started(slowBeat) * 1.5, "the two rates were too close to tell apart");
	assert.ok(loudest(fastBeat) > loudest(slowBeat), "a dying creature beat no harder than a bloodied one");

	// The slow one still has to be worth drawing. It is the whole signal for a
	// creature at a quarter of its hit points, which is most monsters for most
	// of a fight.
	assert.ok(loudest(slowBeat) > 0.5, "a bloodied creature's pulse was too faint to see");

	// And neither ever leaves the range the shader multiplies by.
	for (const beat of [slowBeat, fastBeat]) {
		for (let t = 0; t < over; t += 7) {
			assert.ok(beat(t) >= 0 && beat(t) <= 1, `a beat left its range at ${t}ms`);
		}
	}
});
