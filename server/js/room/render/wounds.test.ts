






import assert from "node:assert/strict";
import { test } from "node:test";

import type { HPBand } from "../protocol.ts";
import {
	BEAT_HEART, BEAT_NONE, BEAT_PERIOD, BEAT_SLOW, DEATH_SPLATTERS, SEVERITY_UNKNOWN, SLOW_PERIOD,
	bandOf, beats, bleeds, fastBeat, healthOf, heartbeat, hurt, severityOf, slowBeat, splatters,
} from "./wounds.ts";

test("the bands sit exactly where the server puts them", () => {
	const cases: [number, number, HPBand][] = [
		
		
		
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

		
		
		[20, 20, "healthy"],
		[16, 20, "healthy"],
		[15, 20, "bruised"],
		[10, 20, "bloody"],
		[5, 20, "veryBloody"],
		[2, 20, "veryBloody"],
		[1, 20, "nearDeath"],
		[0, 20, "dead"],

		
		
		[7, 7, "healthy"],
		[6, 7, "healthy"],
		[5, 7, "bruised"],
		[4, 7, "bruised"],
		[3, 7, "bloody"],
		[2, 7, "bloody"],
		[1, 7, "veryBloody"],

		
		
		[1, 1000, "nearDeath"],
		[0, 1000, "dead"],

		
		
		[30, 20, "healthy"],
	];

	for (const [hp, maxHp, want] of cases) {
		assert.equal(bandOf(hp, maxHp), want, `${hp} of ${maxHp} hit points`);
	}

	assert.equal(bandOf(null, 10), null, "a pawn with no hit points was given a band");
});





test("a creature at zero is dead with or without a maximum", () => {
	assert.equal(bandOf(0, null), "dead");
	assert.equal(bandOf(-4, null), "dead");

	
	
	assert.equal(bandOf(3, null), null, "a pawn with no maximum was given a band");
	assert.equal(bandOf(3, 0), null);
});

test("the number wins over the band, and neither is invented", () => {
	assert.equal(healthOf({ hp: 4, maxHp: 7, hpBand: "dead" }), "bruised");
	assert.equal(healthOf({ hp: null, maxHp: null, hpBand: "bloody" }), "bloody");

	
	
	assert.equal(healthOf({ hp: null, maxHp: null, hpBand: null }), null);
});





test("the same damage is worth more to a smaller creature", () => {
	assert.ok(severityOf(8, 7) > severityOf(8, 40));
	assert.ok(severityOf(8, 40) > severityOf(8, 200));

	
	assert.ok(severityOf(20, 40) > severityOf(8, 40));
});

test("a hit that is not a hit is worth nothing", () => {
	assert.equal(severityOf(0, 40), 0, "standing still shed blood");
	assert.equal(severityOf(-5, 40), 0, "being healed shed blood");
});





test("a hit on a creature with no maximum is an ordinary one", () => {
	assert.equal(severityOf(3, null), SEVERITY_UNKNOWN);
	assert.equal(severityOf(3, 0), SEVERITY_UNKNOWN);

	assert.ok(SEVERITY_UNKNOWN > 0 && SEVERITY_UNKNOWN < 1);
});




test("severity is a fraction and stays one", () => {
	assert.equal(severityOf(40, 40), 1);
	assert.equal(severityOf(400, 40), 1);

	for (const [damage, maxHp] of [[1, 1000], [1, 7], [3, 8], [19, 40], [39, 40]]) {
		const value = severityOf(damage, maxHp);
		assert.ok(value > 0 && value <= 1, `${damage} of ${maxHp} left the range at ${value}`);
	}
});





test("the ordinary hit is not squashed against the floor of the scale", () => {
	
	
	assert.ok(severityOf(5, 40) > 0.3, "the commonest hit in a fight was drawn as nothing");

	
	
	assert.ok(severityOf(5, 40) < severityOf(10, 40));
});






test("only a creature in trouble is marked, and a corpse is marked otherwise", () => {
	assert.equal(hurt(null), 0);
	assert.equal(hurt("healthy"), 0);
	assert.equal(hurt("bruised"), 0, "a scratch marked the portrait");
	assert.equal(hurt("dead"), 0, "a corpse was given a red rim under its grey");

	
	const worse = (["bloody", "veryBloody", "nearDeath"] as const).map(hurt);
	assert.deepEqual(worse, [...worse].sort((a, b) => a - b));
	assert.equal(worse[worse.length - 1], 1);
});




test("the pulse starts at very bloodied and becomes a heart at the end", () => {
	assert.equal(beats(null), BEAT_NONE);
	assert.equal(beats("healthy"), BEAT_NONE);
	assert.equal(beats("bruised"), BEAT_NONE);
	assert.equal(beats("bloody"), BEAT_NONE, "a creature at half health had a pulse");

	assert.equal(beats("veryBloody"), BEAT_SLOW);
	assert.equal(beats("nearDeath"), BEAT_HEART);

	
	
	
	assert.equal(beats("dead"), BEAT_NONE, "a corpse had a heartbeat");
});

test("blood only shows on a portrait once the creature is badly hurt", () => {
	assert.equal(bleeds(null), false);
	assert.equal(bleeds("healthy"), false);
	assert.equal(bleeds("bruised"), false);
	assert.equal(bleeds("bloody"), false);

	assert.equal(bleeds("veryBloody"), true);
	assert.equal(bleeds("nearDeath"), true);

	
	
	assert.equal(bleeds("dead"), true);
});




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

	
	
	
	assert.equal(heartbeat(50, BEAT_PERIOD), heartbeat(50 + BEAT_PERIOD * 3600, BEAT_PERIOD));
});




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

	
	
	
	assert.ok(loudest(slowBeat) > 0.5, "a bloodied creature's pulse was too faint to see");

	
	for (const beat of [slowBeat, fastBeat]) {
		for (let t = 0; t < over; t += 7) {
			assert.ok(beat(t) >= 0 && beat(t) <= 1, `a beat left its range at ${t}ms`);
		}
	}
});
