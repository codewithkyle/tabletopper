// The mute, which is the only part of the ping's noise that has rules. The blip
// itself is four constants and an envelope; what can be WRONG is whether a
// browser that refuses storage takes the room down with it, and whether asking
// for quiet is still quiet after a reload.
//
// EVERY ONE OF THESE IS A REAL BROWSER. A private window throws on the first
// read; a browser set to block site data throws on the write; and both of those
// arrive as an exception out of a getter rather than as a null.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { AudioRamp, MuteStore } from "./ping-sound.ts";
import { BLIP, KEY, PEAK, newMute, voice } from "./ping-sound.ts";

function store(initial: Record<string, string> = {}): MuteStore & { held: Record<string, string> } {
	const held = { ...initial };

	return {
		held,
		getItem: (key) => held[key] ?? null,
		setItem: (key, value) => {
			held[key] = value;
		},
	};
}

// AND THE SHAPE OF THE SOUND, which is the other thing here that can be wrong
// without anybody being able to point at why. It shipped as a single boop: the
// pitch was stepped halfway through a note that was already decaying
// exponentially from its peak, so the second note arrived at 2.5 percent of
// peak -- scheduled, and 32 dB down. The user heard one note and said so.
//
// SO THE TEST IS "DOES THE LEVEL RISE TWICE", not "are the constants these
// numbers". Retuning the pitches, the lengths or the volume is somebody's ear's
// business; a second note nobody can hear is a defect at any tuning.

interface Point {
	how: string;
	value: number;
	at: number;
}

function ramp(): AudioRamp & { points: Point[] } {
	const points: Point[] = [];

	return {
		points,
		setValueAtTime: (value, at) => points.push({ how: "set", value, at }),
		linearRampToValueAtTime: (value, at) => points.push({ how: "linear", value, at }),
		exponentialRampToValueAtTime: (value, at) => points.push({ how: "exp", value, at }),
	};
}

function sounded(at = 0) {
	const pitch = ramp();
	const level = ramp();
	voice(pitch, level, at);

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
// is indistinguishable from a working mute.
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

// A PING'S WHOLE JOB IS TO REACH SOMEBODY WHO IS NOT LOOKING, so a feature that
// shipped silent would ship switched off for everybody who never finds the menu.
test("a browser that has never been asked is not muted", () => {
	assert.equal(newMute(store()).muted(), false);
});

test("asking for quiet is still quiet after a reload", () => {
	const held = store();

	assert.equal(newMute(held).toggle(), true);
	assert.equal(held.held[KEY], "1");
	assert.equal(newMute(held).muted(), true);
});

test("and asking again puts it back", () => {
	const held = store({ [KEY]: "1" });
	const mute = newMute(held);

	assert.equal(mute.muted(), true);
	assert.equal(mute.toggle(), false);
	assert.equal(mute.muted(), false);
	assert.equal(newMute(held).muted(), false);
});

// ONLY "1" IS MUTED. Anything else is a key some other build wrote, a key a
// person typed into devtools, or a value that meant something once -- and the
// safe reading of all three is the default.
test("a value nobody here wrote is not a mute", () => {
	for (const value of ["0", "", "true", "yes", "null"]) {
		assert.equal(newMute(store({ [KEY]: value })).muted(), false, value);
	}
});

// A PRIVATE WINDOW THROWS ON THE READ, and a room that would not open in one
// would be a room somebody could not join from a shared machine.
test("a browser that refuses to be read opens unmuted", () => {
	const angry: MuteStore = {
		getItem() {
			throw new Error("nope");
		},
		setItem() {},
	};

	assert.equal(newMute(angry).muted(), false);
});

// AND ONE THAT REFUSES THE WRITE STILL GOES QUIET. Somebody who asked for quiet
// gets quiet; the only thing they lose is that the next reload asks again, and
// telling them so is worse than not.
test("a browser that refuses to be written to still mutes for the session", () => {
	const angry: MuteStore = {
		getItem: () => null,
		setItem() {
			throw new Error("nope");
		},
	};

	const mute = newMute(angry);

	assert.equal(mute.toggle(), true);
	assert.equal(mute.muted(), true);
});

// AND A BROWSER WITH NO STORAGE AT ALL is the same case one step earlier:
// reading the localStorage PROPERTY throws before any key is asked for.
test("no storage at all is a working session that forgets", () => {
	const mute = newMute(null);

	assert.equal(mute.muted(), false);
	assert.equal(mute.toggle(), true);
	assert.equal(mute.muted(), true);
});
