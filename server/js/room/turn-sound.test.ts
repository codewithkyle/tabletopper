import assert from "node:assert/strict";
import { test } from "node:test";
import type { AudioRamp } from "./turn-sound.ts";
import { CHIME, FULL, NOTES, PEAK, RINGS, STEP, VOICES, gainFor, strike } from "./turn-sound.ts";
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
	const gain = ramp();
	const ends: number[] = [];
	for (let note = 0; note < VOICES; note++) {
		ends.push(strike(pitch, gain, note, at, peak));
	}
	return { pitch: pitch.points, gain: gain.points, ends };
}
test("the chime is three notes and each one is higher than the last", () => {
	const { pitch } = sounded();
	assert.equal(pitch.length, VOICES);
	for (let note = 1; note < VOICES; note++) {
		assert.ok(pitch[note].value > pitch[note - 1].value, `${pitch[note].value} is not above ${pitch[note - 1].value}`);
		assert.ok(pitch[note].at > pitch[note - 1].at, `note ${note} does not start after the one before it`);
	}
});
test("every note is struck and then fades", () => {
	const { gain } = sounded();
	assert.equal(gain.filter((p) => p.how === "linear" && p.value === PEAK).length, VOICES);
	assert.equal(gain.filter((p) => p.how === "exp").length, VOICES);
});
test("a note is still ringing when the next one is struck", () => {
	const { ends } = sounded();
	for (let note = 1; note < VOICES; note++) {
		assert.ok(STEP * note < ends[note - 1], `note ${note} begins after note ${note - 1} has died away`);
	}
});
test("the last note rings the longest, which is where the chime ends", () => {
	const { ends } = sounded();
	assert.equal(Math.max(...ends), ends[VOICES - 1]);
	assert.equal(CHIME, ends[VOICES - 1]);
	assert.ok(RINGS[VOICES - 1] > RINGS[0]);
});
test("nothing ramps exponentially to or from nothing", () => {
	const { gain } = sounded();
	for (const [i, point] of gain.entries()) {
		if (point.how !== "exp") {
			continue;
		}
		assert.ok(point.value > 0, "an exponential ramp reaches zero");
		assert.ok((gain[i - 1]?.value ?? 0) > 0, "an exponential ramp starts from zero");
	}
});
test("the chime is scheduled from now and not from zero", () => {
	const late = sounded(1234.5);
	assert.equal(late.pitch[0].at, 1234.5);
	assert.equal(late.gain[0].at, 1234.5);
	assert.equal(Math.max(...late.ends), 1234.5 + CHIME);
});
test("turning it down changes the level and not the shape", () => {
	const loud = sounded(0, PEAK);
	const soft = sounded(0, PEAK / 4);
	assert.deepEqual(loud.gain.map((p) => p.at), soft.gain.map((p) => p.at));
	assert.deepEqual(loud.gain.map((p) => p.how), soft.gain.map((p) => p.how));
	assert.equal(Math.max(...soft.gain.map((p) => p.value)), PEAK / 4);
});
test("the two ends of the range are exactly full and exactly silent", () => {
	assert.equal(gainFor(FULL), PEAK);
	assert.equal(gainFor(0), 0);
});
test("the volume only ever goes one way as the slider does", () => {
	let previous = -1;
	for (let percent = 0; percent <= FULL; percent += 5) {
		const at = gainFor(percent);
		assert.ok(at > previous, `${percent}% is not louder than the step below it`);
		previous = at;
	}
});
test("a setting outside the range, or no setting at all, is brought into it", () => {
	assert.equal(gainFor(FULL + 40), PEAK);
	assert.equal(gainFor(-10), 0);
	assert.equal(gainFor(Number.NaN), PEAK);
});
test("the chime carries no note the ping carries", () => {
	assert.ok(!NOTES.includes(1046.5), "the chime opens on the ping's low note");
	assert.ok(!NOTES.includes(1568), "the chime carries the ping's high note");
});
