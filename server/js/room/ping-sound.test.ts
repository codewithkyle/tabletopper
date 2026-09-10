// The shape of the sound and the setting that scales it, which are the two
// things here that can be wrong without anybody being able to point at why.
//
// IT SHIPPED AS A SINGLE BOOP. The pitch was stepped halfway through a note that
// was already decaying exponentially from its peak, so the second note arrived at
// 2.5 percent of peak -- scheduled, and 32 dB down. The user heard one note and
// said so.
//
// SO THE TESTS ASK "DOES THE LEVEL RISE TWICE", not "are the constants these
// numbers". Retuning the pitches, the lengths or the volume is somebody's ear's
// business; a second note nobody can hear is a defect at any tuning.

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

// THE ASSERTION THAT WOULD HAVE CAUGHT IT. One rise to peak is one note however
// many pitches were scheduled underneath it.
test("the level is struck once per note", () => {
	const { level } = sounded();

	const rises = level.filter((p) => p.value === PEAK && p.how === "linear");
	assert.equal(rises.length, 2, "the second note is never struck");
});

// AND THE SECOND PITCH HAS TO ARRIVE ON THE WAY UP. This is the same defect
// stated as a question about ordering rather than about counting: the event
// before it is the dip to nothing, and the one after it is a rise to peak.
test("the second note is struck rather than faded into", () => {
	const { pitch, level } = sounded();

	const before = level.filter((p) => p.at <= pitch[1].at).pop();
	const after = level.find((p) => p.at > pitch[1].at);

	assert.ok(before !== undefined && after !== undefined);
	assert.ok(before.value < PEAK, `the level was at ${before.value} when the second note began`);
	assert.equal(after.value, PEAK);
});

// ONE DECAY, AT THE END, because it is the only part of the sound with nothing
// after it. A decay in the middle is the bug.
test("the only fade is the tail", () => {
	const { level } = sounded();

	const fades = level.filter((p) => p.how === "exp");
	assert.equal(fades.length, 1);
	assert.equal(fades[0].at, Math.max(...level.map((p) => p.at)));
	assert.equal(fades[0].at, BLIP);
});

// AN EXPONENTIAL RAMP CANNOT REACH ZERO AND CANNOT START FROM IT: a ramp to zero
// is silently ignored, and one from zero is invalid. Both fail as silence, which
// is indistinguishable from a volume turned all the way down.
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

// It is scheduled RELATIVE to the moment it is asked for, so a context that has
// been running all evening sounds the same as one made a second ago.
test("the sound is scheduled from now and not from zero", () => {
	const late = sounded(1234.5);

	assert.equal(late.pitch[0].at, 1234.5);
	assert.equal(late.level[0].at, 1234.5);
	assert.equal(late.level[late.level.length - 1].at, 1234.5 + BLIP);
});

// THE VOLUME SCALES THE STRIKE AND NOTHING ELSE. The dip between the notes and
// the tail after them are "as close to nothing as a ramp may get", which is a
// property of the ramp rather than of how loud somebody wanted this -- so a
// quieter sound is the same shape, not a shorter one.
test("turning it down changes the level and not the shape", () => {
	const loud = sounded(0, PEAK);
	const soft = sounded(0, PEAK / 4);

	assert.deepEqual(loud.level.map((p) => p.at), soft.level.map((p) => p.at));
	assert.deepEqual(loud.level.map((p) => p.how), soft.level.map((p) => p.how));
	assert.equal(Math.max(...soft.level.map((p) => p.value)), PEAK / 4);
});

// THE ENDS OF THE SLIDER ARE EXACT. Full is the level the sound was tuned at, and
// the bottom is the mute -- a curve that merely got very close to zero would be a
// mute you could still hear through headphones in a quiet room.
test("the two ends of the range are exactly full and exactly silent", () => {
	assert.equal(gainFor(FULL), PEAK);
	assert.equal(gainFor(0), 0);
});

// SQUARED AND NOT LINEAR, because loudness is not. Halfway down is about half as
// loud to an ear rather than "slightly less", which is what puts the middle of
// the range in the middle of the travel.
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

// A NUMBER FROM OUTSIDE IS CLAMPED RATHER THAN TRUSTED. It comes off an attribute
// the page rendered and out of an event's detail, and neither is a promise.
test("a setting outside the range is brought into it", () => {
	assert.equal(gainFor(FULL + 40), PEAK);
	assert.equal(gainFor(-10), 0);
});

// AND SOMETHING THAT IS NOT A NUMBER AT ALL IS FULL, NOT SILENT. A page from a
// build that does not send the setting, or a value somebody put in devtools, must
// still be a page where pings work: a feature that fails quiet is a feature
// nobody reports as broken.
test("an unreadable setting is full volume", () => {
	assert.equal(gainFor(Number.NaN), PEAK);
	assert.equal(gainFor(Number.POSITIVE_INFINITY), PEAK);
});
