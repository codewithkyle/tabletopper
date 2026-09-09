// How hurt a creature is, and what that does to the way it is drawn.
//
// INSIDE THE DISC IS THE CREATURE'S OWN STATE; OUTSIDE IT IS WHAT HAS BEEN DONE
// TO IT. That is the rule this file exists to keep. The rings outside a pawn are
// its conditions -- a countable stack of named things, up to the sixteen the
// protocol allows -- and health is not one of them: it is not something applied
// to the creature, it IS the creature. So every wound below is drawn INSIDE the
// pawn's own circle, by the pawn's own shader, and nothing here ever touches the
// ring stack or grows the pawn's footprint.
//
// IT WAS A RING ONCE AND THAT WAS WRONG TWICE. It made the condition stack
// something you had to know to subtract one from, and the red it had to be drawn
// in is the red a condition ring is already drawn in -- a creature carrying a red
// condition and a creature at a twentieth of its hit points were the same
// hairline at the same radius.
//
// EVERY VIEWER IS SENT THE NUMBERS AND THE ROOM'S LABEL SETTING DOES NOT REACH
// THIS FILE. That setting governs TEXT -- the word under the pointer, the line
// in the details window -- and nothing below is text. A table that has turned
// the labels off has not turned off the fact that the goblin is bleeding, so a
// room set to none draws every wound here and prints no number anywhere. See
// PawnLabels in internal/room/state.go and projectPawn in snapshot.go, which
// hold the sending half of it.
//
// bandOf MIRRORS THE GO AND IS NOW THE MAIN ROAD rather than the GM's alone. It
// is the arrangement hp.ts already has with evaluateHP and path.ts has with
// snap.go: the client resolves it so the table can be drawn without asking, and
// the server resolves it because the server is what actually holds the pawn.
// The other half is hpBand in internal/room/snapshot.go, and the band it still
// sends is what an INTERFACE reads to decide whether to print a word.
//
// NOTHING HERE IS SENT ANYWHERE. It is all drawn from state the client already
// has -- no event, no field, not a byte on the wire.

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

// Health is a viewer's copy of what a creature's hit points are. Null is what it
// reports when they were told NOTHING -- no number and no band -- which is a
// monster in a room whose labels are off, and is why every consumer below has to
// treat that differently from "healthy".
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

// A HIT IS THE DIFFERENCE BETWEEN TWO HIT-POINT TOTALS, and severityOf is how
// big that difference looked. It is the fraction of the creature's OWN maximum
// that went, so the same eight points is a scratch on a giant and most of a
// goblin -- which is the only reason a mark on the floor can say anything about
// a hit rather than just that one happened.
//
// IT WAS A BAND TRANSITION ONCE AND THAT WAS A PROXY FOR THIS. Blood appeared
// only when a creature crossed one of five lines, so four hits in a row could
// pass without a mark and the fifth threw one for a reason nobody at the table
// could see. The difference IS the event. The band was a way of noticing it.
//
// THE SQUARE ROOT IS THE POINT OF THE FUNCTION. Damage as a fraction of a
// maximum lives at the bottom of its range -- a d8 against a forty-point monster
// is twelve per cent, and an ordinary fight is a long run of numbers under a
// fifth. Read straight, almost every hit would draw the same smallest mark and
// the scale would be spent on hits nobody ever deals. The root spreads that
// common band across the middle of what can be seen, and leaves the top of it
// for the hit somebody is going to remember.
export function severityOf(damage: number, maxHp: number | null): number {
	if (damage <= 0) {
		return 0;
	}
	if (maxHp === null || maxHp < 1) {
		return SEVERITY_UNKNOWN;
	}

	const fraction = damage / maxHp;

	return fraction >= 1 ? 1 : Math.sqrt(fraction);
}

// SEVERITY_UNKNOWN is a hit on a creature nobody wrote a maximum for. There is
// no fraction to take, so it is drawn as an ordinary hit rather than as the
// smallest or the largest: a GM counting damage upward on a monster with no
// stat line still gets a mark for every one of them.
export const SEVERITY_UNKNOWN = 0.4;

// hurt is how far into the creature's own picture the injury has got, from 0 for
// untouched to 1 for about to die. The pawn shader reads it as one number and
// spends it on three things at once: blood soaking in from the rim, that blood
// pooling toward the bottom of the disc, and the colour going out of the
// creature.
//
// NOTHING ABOVE THE HALFWAY LINE IS MARKED AT ALL, which is the most important
// entry in the table and the one that is missing. A table where every creature
// looks wounded is a table with no signal in it, so the mark starts at the word
// a table already says out loud and gets worse twice on the way down.
//
// AND A CORPSE IS NOT MARKED EITHER -- not because it is unhurt, but because the
// desaturation and the skull that a dead creature already carries say more than
// any of this could, and a red rim underneath a full grey would be greyed away
// in the same breath. What a corpse keeps is the blood ON it; see bleeds.
export function hurt(band: HPBand | null): number {
	switch (band) {
		case "bloody":
			return 0.45;
		case "veryBloody":
			return 0.75;
		case "nearDeath":
			return 1;
		default:
			return 0;
	}
}

// BEAT_NONE, BEAT_SLOW and BEAT_HEART are what a creature's pulse is doing, and
// they ride into the shader as a number because a per-instance branch is a float
// compare either way.
//
// THE PULSE IS A GRADIENT ON THE INSIDE OF THE RIM AND NOTHING TRAVELS. It was a
// ring collapsing toward the centre once, and the travel was the wrong idea
// twice: a shape crossing the token pulls the eye off the token, and the ring
// arrives ON the face at the end of every beat, which is the one part of a pawn
// that has to stay legible. So the geometry is fixed -- bright at the creature's
// edge, falling off quickly inward -- and only its brightness beats.
//
// EITHER WAY IT STAYS INSIDE THE DISC, which is the rule this whole file is
// about: outward would sail into the ring stack the conditions own.
export const BEAT_NONE = 0;
export const BEAT_SLOW = 1;
export const BEAT_HEART = 2;

// beats is which of those a band gets: the same heart labouring at two rates.
export function beats(band: HPBand | null): number {
	switch (band) {
		case "veryBloody":
			return BEAT_SLOW;
		case "nearDeath":
			return BEAT_HEART;
		default:
			return BEAT_NONE;
	}
}

// bleeds is whether the creature carries actual blood on its portrait -- one of
// the nine splatters, composited into the disc and faded out toward the middle
// so it soaks in from the edges rather than covering the face. See the rim mask
// in pawn-pass.ts: the splatter's dense centre is exactly where the face is, and
// exactly what gets masked away.
//
// A CORPSE KEEPS ITS BLOOD, DRIED. The body goes grey and the splatter over it
// goes to the same dark maroon the floor's marks settle at -- bright arterial red
// on a black and white portrait reads as paint rather than as blood, and both
// changes land on the frame the skull arrives. See BLOOD_DRIED.
export function bleeds(band: HPBand | null): boolean {
	return band === "veryBloody" || band === "nearDeath" || band === "dead";
}

// BLOOD_FRESH and BLOOD_DRIED are what the sheet's pure red is multiplied by.
// The art has no desaturation anywhere in it -- the green and blue channels are
// essentially zero -- so the red channel carries all of a splatter's shading and
// a tint against it recolours the whole set from one texture.
//
// THEY ARE SHARED BY THE FLOOR AND THE PORTRAIT so that a corpse and the pool it
// is lying in are the same colour. A creature's blood dries the moment it dies:
// bright red on a body that has just gone grey and white reads as paint rather
// than as blood, and the two changes land together, on the frame the skull
// arrives.
export const BLOOD_FRESH: readonly [number, number, number] = [1, 0.13, 0.1];
export const BLOOD_DRIED: readonly [number, number, number] = [0.34, 0.06, 0.05];

// BLOOD_VARIANTS is how many splatters the sheet was cut into. They live under
// /images/blood at 256 square, which is the sprite cache's layer size, so each
// is an ordinary cache entry and none of it is special-cased.
export const BLOOD_VARIANTS = 9;

// bloodSprite is the URL of one of the nine. They are numbered from one because
// that is what the sheet they were cut from looks like, and the index is wrapped
// rather than trusted -- an off-by-one should be a repeated splatter and never a
// 404 the loader remembers for the rest of the session.
export function bloodSprite(variant: number): string {
	return `/images/blood/${(((variant % BLOOD_VARIANTS) + BLOOD_VARIANTS) % BLOOD_VARIANTS) + 1}.webp`;
}

// seed is FNV-1a, which is enough to turn a ULID into a number and short enough
// to read. Everything blood-related that wants to look random is a function of
// this rather than of Math.random, so that a table looking at the same creature
// sees the same wounds on it.
export function seed(text: string): number {
	let value = 0x811c9dc5;
	for (let i = 0; i < text.length; i++) {
		value ^= text.charCodeAt(i);
		value = Math.imul(value, 0x01000193) >>> 0;
	}

	return value >>> 0;
}

// BEAT_PERIOD and SLOW_PERIOD are one beat, in milliseconds. Around 57 a minute
// for a creature that is dying and 33 for one that is badly hurt: the same heart
// labouring at two rates, so a table with both on it reads as one language
// rather than as two effects.
export const BEAT_PERIOD = 1050;
export const SLOW_PERIOD = 1800;

// SLOW_PEAK and BEAT_PEAK are how hard each of them hits.
const SLOW_PEAK = 0.55;
const BEAT_PEAK = 1;

// THUMPS is lub-dub: two knocks close together and then a long rest, with the
// second quieter than the first.
//
// IT IS NOT A SINE WAVE, AND THAT IS THE WHOLE POINT. A sine reads as
// "selected", or as something loading; every interface the reader has ever used
// pulses that way. Two thumps and a silence is a heart, unmistakably, and it
// costs exactly the same arithmetic.
//
// THE THUMPS THEMSELVES ARE THE SAME LENGTH AT BOTH RATES. Only the rest between
// beats changes, which is what a slower heart actually is -- stretching the knock
// as well would turn it into a swell.
const THUMPS: readonly (readonly [number, number])[] = [[0, 1], [220, 0.78]];
const ATTACK = 50;
const DECAY = 150;

// heartbeat is that envelope, from 0 at rest to 1 at the top of a thump.
//
// IT IS THE WHOLE OF THE PULSE NOW. It used to drive a ring that travelled from
// the creature's rim to its centre, and the travel was the wrong idea: a shape
// crossing the token pulled the eye away from the token, and it collapsed onto
// the face at the end of every beat. What the shader draws instead is a gradient
// anchored at the rim with a short fall-off inward, and this is its brightness --
// so nothing MOVES, it just beats.
export function heartbeat(now: number, period: number): number {
	const t = phase(now, period);

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

// slowBeat and fastBeat are the two rates as the shader takes them: one number
// each, both read every frame and handed to every instance at once.
export function slowBeat(now: number): number {
	return SLOW_PEAK * heartbeat(now, SLOW_PERIOD);
}

export function fastBeat(now: number): number {
	return BEAT_PEAK * heartbeat(now, BEAT_PERIOD);
}

// phase is where in a repeating window the clock is. It is written once because
// performance.now() is milliseconds since the tab loaded and every one of these
// has to look the same an hour into a session as it did in the first minute.
function phase(now: number, period: number): number {
	return ((now % period) + period) % period;
}

// splatters is how many marks ONE HIT throws on the floor.
//
// IT IS NEVER ZERO, and that is the rule this replaced a band table to get: a
// hit that lands leaves something, always, at the moment and in the place it
// landed. What a small hit gets is a small faint mark rather than no mark, which
// is a thing the floor can say and a band transition never could.
//
// AND IT IS NEVER FOUR. Four is the death spray, and nothing a living creature
// survives should be mistaken across a table for something dying.
export function splatters(severity: number): number {
	if (severity >= HEAVY) {
		return 3;
	}
	if (severity >= SOLID) {
		return 2;
	}

	return 1;
}

// SOLID and HEAVY are where a hit earns its second and third mark. They read on
// the CURVED scale rather than the raw fraction, which puts them at a quarter of
// a creature's maximum and at about two thirds of it.
const SOLID = 0.5;
const HEAVY = 0.8;

// DEATH_SPLATTERS is the spray a corpse throws on top of its pool, deliberately
// out of reach of the scale above. See decals.ts, where the pool is laid first
// and this lands on it.
export const DEATH_SPLATTERS = 4;
