











import assert from "node:assert/strict";
import { test } from "node:test";

import type { AudioRamp } from "./ping-sound.ts";
import { BLIP, FULL, PEAK, gainFor, voice } from "./ping-sound.ts";

interface Point {
	how: string;
	value: number;
	at: number;
}

function ramp(): AudioRamp & { points: Point[] } {
	const points: Point[] = [];

	return {
		points,
		setValueAtTime: (value: number, at: number) => points.push({ how: "set", value, at }),
		linearRampToValueAtTime: (value: number, at: number) => points.push({ how: "linear", value, at }),
		exponentialRampToValueAtTime: (value: number, at: number) => points.push({ how: "exp", value, at }),
	};
}

function sounded(at = 0, peak = PEAK) {
	const pitch = ramp();
	const level = ramp();
	voice(pitch, level, at, peak);

	return { pitch: pitch.points, level: level.points };
}

test("the ping is two notes and the second one is higher", () => {
	const { pitch } = sounded();

	assert.equal(pitch.length, 2);
	assert.ok(pitch[1].value > pitch[0].value, `${pitch[1].value} is not above ${pitch[0].value}`);
	assert.ok(pitch[1].at > pitch[0].at, "both pitches start at once");
});



test("the level is struck once per note", () => {
	const { level } = sounded();

	const rises = level.filter((p) => p.value === PEAK && p.how === "linear");
	assert.equal(rises.length, 2, "the second note is never struck");
});




test("the second note is struck rather than faded into", () => {
	const { pitch, level } = sounded();

	const before = level.filter((p) => p.at <= pitch[1].at).pop();
	const after = level.find((p) => p.at > pitch[1].at);

	assert.ok(before !== undefined && after !== undefined);
	assert.ok(before.value < PEAK, `the level was at ${before.value} when the second note began`);
	assert.equal(after.value, PEAK);
});



test("the only fade is the tail", () => {
	const { level } = sounded();

	const fades = level.filter((p) => p.how === "exp");
	assert.equal(fades.length, 1);
	assert.equal(fades[0].at, Math.max(...level.map((p) => p.at)));
	assert.equal(fades[0].at, BLIP);
});




test("nothing ramps exponentially to or from nothing", () => {
	const { level } = sounded();

	for (const [i, point] of level.entries()) {
		if (point.how !== "exp") {
			continue;
		}

		assert.ok(point.value > 0, "an exponential ramp reaches zero");
		assert.ok((level[i - 1]?.value ?? 0) > 0, "an exponential ramp starts from zero");
	}
});



test("the sound is scheduled from now and not from zero", () => {
	const late = sounded(1234.5);

	assert.equal(late.pitch[0].at, 1234.5);
	assert.equal(late.level[0].at, 1234.5);
	assert.equal(late.level[late.level.length - 1].at, 1234.5 + BLIP);
});





test("turning it down changes the level and not the shape", () => {
	const loud = sounded(0, PEAK);
	const soft = sounded(0, PEAK / 4);

	assert.deepEqual(loud.level.map((p) => p.at), soft.level.map((p) => p.at));
	assert.deepEqual(loud.level.map((p) => p.how), soft.level.map((p) => p.how));
	assert.equal(Math.max(...soft.level.map((p) => p.value)), PEAK / 4);
});




test("the two ends of the range are exactly full and exactly silent", () => {
	assert.equal(gainFor(FULL), PEAK);
	assert.equal(gainFor(0), 0);
});




test("the middle of the slider is quieter than half", () => {
	const half = gainFor(FULL / 2);

	assert.ok(half < PEAK / 2, `${half} is not below half of ${PEAK}`);
	assert.ok(half > 0);
});

test("the volume only ever goes one way as the slider does", () => {
	let previous = -1;
	for (let percent = 0; percent <= FULL; percent += 5) {
		const level = gainFor(percent);
		assert.ok(level > previous, `${percent}% is not louder than the step below it`);
		previous = level;
	}
});



test("a setting outside the range is brought into it", () => {
	assert.equal(gainFor(FULL + 40), PEAK);
	assert.equal(gainFor(-10), 0);
});





test("an unreadable setting is full volume", () => {
	assert.equal(gainFor(Number.NaN), PEAK);
	assert.equal(gainFor(Number.POSITIVE_INFINITY), PEAK);
});
