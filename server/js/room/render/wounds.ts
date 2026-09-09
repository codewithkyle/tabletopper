// How hurt a creature is, and the three things that answers: the ring round it,
// the heartbeat when it is nearly gone, and how much blood a hit puts on the
// floor.
//
// THE BAND IS ALREADY ON THE WIRE AND THIS IS ONLY THE OTHER HALF OF IT. A
// player is sent one of six words instead of a number, on purpose -- see HPBand
// in internal/room/state.go, which argues at length that a bar is a number drawn
// sideways and that a party who can see a monster is at three fifths can work
// out its maximum from two hits. Everything below reads that word, so nothing
// here can leak a number the server decided to withhold, and a room with its
// labels off draws no ring and sheds no blood at all -- exactly as it already
// draws no skull.
//
// A GM IS SENT THE NUMBER INSTEAD, so bandOf below turns one into the other.
// That mirrors Go, which is the arrangement hp.ts already has with evaluateHP
// and path.ts has with snap.go: the client resolves it so the table can be drawn
// without asking, and the server resolves it because the server is what actually
// holds the pawn. The other half is hpBand in internal/room/snapshot.go.
//
// NOTHING HERE IS SENT ANYWHERE. The rings are drawn from state the client
// already has, and the blood is decoration that never leaves the browser -- no
// event, no field, not a byte on the wire.

import type { HPBand, Pawn } from "../protocol.ts";

// bandOf is hpBand in internal/room/snapshot.go, and the cuts must stay the
// same as that function's: three quarters, a half, a quarter and a twentieth,
// tested downward so the first match wins. They multiply rather than divide so
// that a maximum of 7 has exact thresholds rather than ones that depend on which
// way integer division fell -- the same reason the Go does.
//
// IT IS THAT FUNCTION PLUS ONE CASE. hpBand answers nil without a maximum,
// because a projection with no maximum has no band to send. Here a creature at
// or below zero is dead whether or not anybody wrote down what it started with,
// which is what the skull has always done and is the honest reading of the one
// number there is.
export function bandOf(hp: number | null, maxHp: number | null): HPBand | null {
	if (hp === null) {
		return null;
	}
	if (hp <= 0) {
		return "dead";
	}
	if (maxHp === null || maxHp < 1) {
		return null;
	}

	if (hp * 20 <= maxHp) {
		return "nearDeath";
	}
	if (hp * 4 <= maxHp) {
		return "veryBloody";
	}
	if (hp * 2 <= maxHp) {
		return "bloody";
	}
	if (hp * 4 <= maxHp * 3) {
		return "bruised";
	}

	return "healthy";
}

// Healthy is what a pawn reports when the viewer was told something about it.
// Null is what it reports when they were told NOTHING -- no number and no band
// -- which is a monster in a room whose labels are off, and is why every
// consumer below has to treat the two differently.
type Health = Pick<Pawn, "hp" | "maxHp" | "hpBand">;

// healthOf is the one rule for reading a viewer's copy of a creature's health,
// and it is the rule the skull has always used: the number where there is one,
// the band where there is not, and nothing at all where there is neither.
//
// THE NUMBER WINS WHERE BOTH ARRIVE, which is every player character in every
// room and every monster in a room whose labels are full.
export function healthOf(pawn: Health): HPBand | null {
	return pawn.hp !== null ? bandOf(pawn.hp, pawn.maxHp) : pawn.hpBand;
}

// RANK is the six bands as a scale, so "got worse" is a comparison rather than a
// list of pairs. It is the only place their order is written down.
const RANK: Record<HPBand, number> = {
	healthy: 0,
	bruised: 1,
	bloody: 2,
	veryBloody: 3,
	nearDeath: 4,
	dead: 5,
};

// worsened is what throws blood, and it is a band transition rather than any
// drop in hit points ON PURPOSE. A GM applying a one-point scratch to a full
// orc should not spray the floor, and -- more importantly -- a GM and a player
// watching the same hit land must see the same thing. They read different
// fields, a number and a word, and the band is where those two agree.
export function worsened(from: HPBand, to: HPBand): boolean {
	return RANK[to] > RANK[from];
}

// WoundRing is the ring drawn round a creature's own edge, inside its
// conditions.
export interface WoundRing {
	color: readonly [number, number, number];

	// thickness is in DEVICE pixels, as every other ring in this app is: a line
	// that scaled with the camera would be a hairline zoomed out, which is where
	// a GM is when they most want to count them.
	thickness: number;

	// beats says this one has a heartbeat, and it is the only thing in the
	// renderer that asks for frames indefinitely. See the note on heartbeat.
	beats: boolean;
}

// woundRing is the whole visual vocabulary of injury, and its most important
// entry is the one that is missing.
//
// BRUISED DRAWS NOTHING, and neither does healthy. A table where every creature
// carries a red ring is a table with no signal in it -- the ring has to mean
// something, and what it means is "this one is in trouble". So it starts at the
// halfway line, which is the word a table already says out loud, and gets louder
// twice on the way down.
//
// AND A CORPSE DRAWS NOTHING EITHER. A dead creature is already desaturated with
// a skull over it, which is a stronger mark than any ring; a heartbeat on
// something with no heart would be the renderer contradicting itself.
export function woundRing(band: HPBand | null): WoundRing | null {
	switch (band) {
		case "bloody":
			return { color: [0.72, 0.11, 0.11], thickness: 2, beats: false };
		case "veryBloody":
			return { color: [0.9, 0.13, 0.13], thickness: 3, beats: false };
		case "nearDeath":
			return { color: [1, 0.2, 0.18], thickness: 3, beats: true };
		default:
			return null;
	}
}

// BEAT_PERIOD is one beat, in milliseconds. Around 57 a minute: slow enough to
// read as a heart rather than as a blinking cursor, fast enough to be alarming.
export const BEAT_PERIOD = 1050;

// THUMPS is lub-dub: two knocks close together and then a long rest, with the
// second quieter than the first.
//
// IT IS NOT A SINE WAVE, AND THAT IS THE WHOLE POINT. A sine reads as
// "selected", or as something loading; every interface the reader has ever used
// pulses that way. Two thumps and a silence is a heart, unmistakably, and it
// costs exactly the same arithmetic.
const THUMPS: readonly (readonly [number, number])[] = [[0, 1], [220, 0.78]];
const ATTACK = 50;
const DECAY = 150;

// heartbeat is the envelope, from 0 at rest to 1 at the top of a thump.
export function heartbeat(now: number): number {
	const t = ((now % BEAT_PERIOD) + BEAT_PERIOD) % BEAT_PERIOD;

	let peak = 0;
	for (const [at, height] of THUMPS) {
		const since = t - at;
		if (since < 0 || since > ATTACK + DECAY) {
			continue;
		}

		const value = height * (since < ATTACK ? since / ATTACK : 1 - (since - ATTACK) / DECAY);
		if (value > peak) {
			peak = value;
		}
	}

	return peak;
}

// ECHO_MS is how long the ring thrown off by a beat takes to travel out and
// vanish. It runs past the thump that launched it, which is what makes the
// heartbeat read as something leaving the body rather than as a ring changing
// width.
const ECHO_MS = 520;

export interface Echo {
	// grow is how far out the ring has travelled, from 0 at the pawn's edge to
	// 1 at its full spread.
	grow: number;
	alpha: number;
}

// echo is the ring the first thump throws outward. There is one per beat rather
// than one per thump: two expanding rings in flight at once reads as a ripple in
// water, which is a different thing entirely.
//
// THE FADE IS SQUARED so the ring is faint for most of its travel and the eye
// catches the moment it leaves rather than the whole journey.
export function echo(now: number): Echo | null {
	const t = ((now % BEAT_PERIOD) + BEAT_PERIOD) % BEAT_PERIOD;
	if (t > ECHO_MS) {
		return null;
	}

	const grow = t / ECHO_MS;

	return { grow, alpha: 0.7 * (1 - grow) * (1 - grow) };
}

// splatters is how many marks a creature throws on the floor for arriving in a
// band. Nothing above the halfway line sheds any, which is the ring's rule
// again: the first blood on the floor and the first ring round the pawn are the
// same moment.
export function splatters(band: HPBand): number {
	switch (band) {
		case "bloody":
			return 1;
		case "veryBloody":
			return 2;
		case "nearDeath":
			return 3;
		case "dead":
			return 4;
		default:
			return 0;
	}
}
