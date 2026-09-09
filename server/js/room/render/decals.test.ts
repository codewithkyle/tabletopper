// The blood, which is entirely the client's: no event carries it, no field
// holds it, and the server has never heard of it. That makes these tests the
// only thing standing between a wrong rule and a table covered in marks nobody
// can account for.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { Pawn } from "../protocol.ts";
import type { SpriteCache } from "./sprites.ts";
import type { DecalTarget } from "./decals.ts";
import { CAP, newDecals } from "./decals.ts";
import { bloodSprite } from "./wounds.ts";

const GROUND = "01LAYERGROUND";
const CELLAR = "01LAYERCELLAR";
const CELL = 64;

function pawn(over: Partial<Pawn> = {}): Pawn {
	return {
		id: "01PAWN",
		kind: "monster",
		layerId: GROUND,
		name: "Goblin",
		image: "",
		x: 100,
		y: 200,
		z: 1,
		size: "medium",
		width: 0,
		height: 0,
		rotation: 0,
		visible: true,
		hp: 7,
		maxHp: 7,
		hpBand: null,
		ac: 15,
		conditions: [],
		ownerId: null,
		monsterId: null,
		characterId: null,
		...over,
	};
}

interface Mark {
	x: number;
	y: number;
	half: number;
	color: [number, number, number];
	alpha: number;
	rotation: number;
	sprite: number;
}

// The recorder COPIES the colour, because the pool hands out one reused tuple
// per frame rather than allocating one per mark.
function recorder(): DecalTarget & { marks: Mark[] } {
	const marks: Mark[] = [];

	return {
		marks,
		begin() {
			marks.length = 0;
		},
		add(x, y, halfW, _halfH, color, alpha, layer, _uvW, _uvH, rotation) {
			marks.push({
				x, y, half: halfW, alpha, rotation, sprite: layer,
				color: [color[0], color[1], color[2]],
			});
		},
	};
}

// Every splatter is resident the moment it is asked for, so these tests are
// about the pool's rules rather than about a cache that has not filled yet.
const sprites = {
	sprite: (url: string) => ({ layer: url.length, w: 256, h: 256, used: 0 }),
} as unknown as SpriteCache;

function drawn(decals: ReturnType<typeof newDecals>, now: number, layer = GROUND): Mark[] {
	const into = recorder();
	decals.build(layer, now, sprites, into);

	return into.marks;
}

test("a pawn met for the first time does not bleed", () => {
	const decals = newDecals();

	// Somebody opening the room mid-fight is sent every creature in it at
	// whatever health it is already on. A pool that treated arrival as a
	// transition would greet them with blood under every wounded thing on the
	// table.
	decals.watch([pawn({ hp: 1, maxHp: 20 })], GROUND, CELL, 0);
	assert.deepEqual(drawn(decals, 0), []);

	// And staying where it is is not a transition either.
	decals.watch([pawn({ hp: 1, maxHp: 20 })], GROUND, CELL, 100);
	assert.deepEqual(drawn(decals, 100), []);
});

test("blood is shed on the way down and not on the way back up", () => {
	const decals = newDecals();

	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 3, maxHp: 7 })], GROUND, CELL, 10);

	const hit = drawn(decals, 10);
	assert.equal(hit.length, 1, "a creature crossing the halfway line shed no blood");

	// Healed back up, then hurt again to the SAME band it was in before. Only
	// the second of those is a worsening.
	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 20);
	assert.equal(drawn(decals, 20).length, 1, "being healed shed blood");

	decals.watch([pawn({ hp: 3, maxHp: 7 })], GROUND, CELL, 30);
	assert.equal(drawn(decals, 30).length, 2);
});

// A ONE POINT SCRATCH IS NOT A SPRAY. The trigger is the band rather than the
// number, which is also what makes a GM reading hit points and a player reading
// a word see the same blood at the same moment.
test("a scratch throws nothing, and neither does a bruise", () => {
	const decals = newDecals();

	decals.watch([pawn({ hp: 100, maxHp: 100 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 99, maxHp: 100 })], GROUND, CELL, 10);
	decals.watch([pawn({ hp: 80, maxHp: 100 })], GROUND, CELL, 20);
	assert.deepEqual(drawn(decals, 20), [], "twenty points inside one band drew blood");

	// Bruised crosses a band and STILL draws nothing. The first blood on the
	// floor and the first ring round the pawn are the same moment, and that
	// moment is the halfway line -- a table where everything is marked is a
	// table with no signal in it.
	decals.watch([pawn({ hp: 60, maxHp: 100 })], GROUND, CELL, 30);
	assert.deepEqual(drawn(decals, 30), [], "a bruise bled");

	decals.watch([pawn({ hp: 50, maxHp: 100 })], GROUND, CELL, 40);
	assert.equal(drawn(decals, 40).length, 1, "crossing the halfway line drew nothing");
});

test("a room that tells the viewer nothing sheds nothing", () => {
	const decals = newDecals();

	// Labels off: a monster arrives with no number and no band, so no viewer can
	// know it was hit and no floor may show that it was.
	const unlabelled = { hp: null, maxHp: null, hpBand: null };
	decals.watch([pawn(unlabelled)], GROUND, CELL, 0);
	decals.watch([pawn(unlabelled)], GROUND, CELL, 10);
	assert.deepEqual(drawn(decals, 10), []);

	// A player in an ordinary room is sent the band instead of the number, and
	// bleeds off exactly that.
	decals.watch([pawn({ hp: null, maxHp: null, hpBand: "healthy" })], GROUND, CELL, 20);
	decals.watch([pawn({ hp: null, maxHp: null, hpBand: "veryBloody" })], GROUND, CELL, 30);
	assert.equal(drawn(decals, 30).length, 2);
});

test("objects and creatures nobody can see do not bleed", () => {
	const decals = newDecals();

	// A wagon has hit points and can be broken apart. There is no blood in it.
	const wagon = { id: "wagon", kind: "object" as const, width: 128, height: 256 };
	decals.watch([pawn({ ...wagon, hp: 30, maxHp: 30 })], GROUND, CELL, 0);
	decals.watch([pawn({ ...wagon, hp: 2, maxHp: 30 })], GROUND, CELL, 10);
	assert.deepEqual(drawn(decals, 10), [], "an object bled");

	// AND AN AMBUSHER IN THE DARK DOES NOT EITHER. This is the GM's copy alone
	// -- nobody else is sent a hidden pawn -- and blood under nothing is a mark
	// the GM has to remember the meaning of.
	decals.watch([pawn({ id: "hidden", visible: false, hp: 7, maxHp: 7 })], GROUND, CELL, 20);
	decals.watch([pawn({ id: "hidden", visible: false, hp: 1, maxHp: 7 })], GROUND, CELL, 30);
	assert.deepEqual(drawn(decals, 30), [], "a pawn players cannot see bled");
});

// A goblin hurt in the cellar while the GM is upstairs must not bleed all over
// again the moment they go down there. What happened is recorded on every floor;
// only the viewed one is drawn on.
test("a hit on another floor is remembered rather than saved up", () => {
	const decals = newDecals();

	decals.watch([pawn({ layerId: CELLAR, hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.watch([pawn({ layerId: CELLAR, hp: 1, maxHp: 7 })], GROUND, CELL, 10);

	// The GM walks down. Nothing changed about the goblin, and nothing is owed.
	decals.watch([pawn({ layerId: CELLAR, hp: 1, maxHp: 7 })], CELLAR, CELL, 20);
	assert.deepEqual(drawn(decals, 20, CELLAR), [], "blood was saved up for a floor change");

	// The next hit, now that it is being watched, lands where it happened.
	decals.watch([pawn({ layerId: CELLAR, hp: 0, maxHp: 7 })], CELLAR, CELL, 30);
	assert.ok(drawn(decals, 30, CELLAR).length > 0);
	assert.deepEqual(drawn(decals, 30, GROUND), [], "the cellar's blood turned up upstairs");
});

test("a death leaves a pool as well as a spray", () => {
	const decals = newDecals();

	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 0, maxHp: 7 })], GROUND, CELL, 10);

	const marks = drawn(decals, 10);
	assert.ok(marks.length >= 2);

	// The pool is much bigger than anything a hit throws, and it is the mark
	// still on the floor when the body has been cleared away.
	const biggest = Math.max(...marks.map((mark) => mark.half));
	const rest = marks.map((mark) => mark.half).filter((half) => half !== biggest);
	assert.ok(biggest > Math.max(...rest) * 1.3, "a death left nothing bigger than a scratch");
});

// A wound is measured against the creature that shed it, so a dragon throws more
// blood than a goblin rather than the same disc at every size.
test("a bigger creature throws bigger marks", () => {
	const small = newDecals();
	small.watch([pawn({ size: "tiny", hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	small.watch([pawn({ size: "tiny", hp: 1, maxHp: 7 })], GROUND, CELL, 10);

	const large = newDecals();
	large.watch([pawn({ size: "gargantuan", hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	large.watch([pawn({ size: "gargantuan", hp: 1, maxHp: 7 })], GROUND, CELL, 10);

	assert.ok(drawn(large, 10)[0].half > drawn(small, 10)[0].half * 4);
});

// THE WHOLE FRAME BUDGET IS THIS TEST. frame.ts promises that a room nobody is
// touching renders nothing at all, so blood that went on animating for ever
// would quietly burn a core on six machines for the rest of the session.
test("the floor stops changing once the blood has dried", () => {
	const decals = newDecals();

	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 3, maxHp: 7 })], GROUND, CELL, 1000);

	assert.equal(decals.settling(1000), true, "a fresh mark did not ask for a frame");
	assert.equal(decals.settling(5000), true);
	assert.equal(decals.settling(1000 + 60_000), false, "the blood never stopped drying");
});

test("blood arrives bright and settles dark, and then stays", () => {
	const decals = newDecals();

	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 3, maxHp: 7 })], GROUND, CELL, 0);

	const landing = drawn(decals, 0)[0];
	const wet = drawn(decals, 200)[0];
	const dried = drawn(decals, 30_000)[0];
	const later = drawn(decals, 300_000)[0];

	assert.equal(landing.alpha, 0, "the mark did not fade in");
	assert.ok(wet.alpha > dried.alpha, "the blood did not fade as it dried");
	assert.ok(wet.color[0] > dried.color[0], "the blood did not darken as it dried");

	// It lands slightly too big and settles, which is what makes a hit read as
	// an impact rather than as an image being switched on.
	assert.ok(landing.half > wet.half);

	// AND THEN NOTHING. A dried mark is a static quad for the rest of the
	// evening -- same colour, same alpha, same size.
	assert.deepEqual(later, dried);
});

// Nobody is sent anybody else's blood, so what keeps a table agreeing about it
// is that the arrangement is a function of the pawn's own id.
test("two people watching the same fight see the same floor", () => {
	const hits: Pawn[][] = [
		[pawn({ id: "01ARI", hp: 14, maxHp: 14 }), pawn({ id: "01RIN", hp: 9, maxHp: 9 })],
		[pawn({ id: "01ARI", hp: 6, maxHp: 14 }), pawn({ id: "01RIN", hp: 9, maxHp: 9 })],
		[pawn({ id: "01ARI", hp: 1, maxHp: 14 }), pawn({ id: "01RIN", hp: 2, maxHp: 9 })],
		[pawn({ id: "01ARI", hp: 0, maxHp: 14 }), pawn({ id: "01RIN", hp: 0, maxHp: 9 })],
	];

	const mine = newDecals();
	const theirs = newDecals();
	for (let i = 0; i < hits.length; i++) {
		mine.watch(hits[i], GROUND, CELL, i * 100);
		theirs.watch(hits[i], GROUND, CELL, i * 100);
	}

	const ours = drawn(mine, 500);
	assert.ok(ours.length > 4, "the fight left almost no blood");
	assert.deepEqual(ours, drawn(theirs, 500));

	// And it is genuinely scattered rather than a stack of identical marks in
	// one place, which a seed that ignored which splatter this was would give.
	assert.ok(new Set(ours.map((mark) => `${mark.x},${mark.y}`)).size > 1);
	assert.ok(new Set(ours.map((mark) => mark.rotation)).size > 1);
});

// A long session should stain a room, not bury it.
test("a floor holds a bounded amount of blood", () => {
	const decals = newDecals();
	const party = (hp: number): Pawn[] =>
		Array.from({ length: 40 }, (_, i) => pawn({ id: `pawn-${i}`, x: i * 10, hp, maxHp: 20 }));

	decals.watch(party(20), GROUND, CELL, 0);
	decals.watch(party(9), GROUND, CELL, 10);
	decals.watch(party(0), GROUND, CELL, 20);

	// Past EVICT_MS the marks that were retired are gone rather than merely
	// invisible, so the array does not grow for the rest of the session.
	assert.ok(drawn(decals, 20).length > CAP, "the test did not actually overfill the floor");
	assert.ok(drawn(decals, 5000).length <= CAP, "a long fight buried the map");
});

test("clearing a floor takes its blood with it", () => {
	const decals = newDecals();

	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 0, maxHp: 7 })], GROUND, CELL, 10);
	assert.ok(drawn(decals, 10).length > 0);

	decals.clear(GROUND);
	assert.deepEqual(drawn(decals, 10), []);
	assert.equal(decals.settling(10), false);
});

// A body being carried off does not clean the floor. The pawn goes; what it shed
// is scenery now.
test("blood outlives the creature that shed it", () => {
	const decals = newDecals();

	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 0, maxHp: 7 })], GROUND, CELL, 10);
	const shed = drawn(decals, 10).length;

	decals.watch([], GROUND, CELL, 20);
	assert.equal(drawn(decals, 20).length, shed);
});

test("the nine splatters are addressed as the sheet numbers them", () => {
	assert.equal(bloodSprite(0), "/images/blood/1.webp");
	assert.equal(bloodSprite(8), "/images/blood/9.webp");

	// Wrapped rather than out of range, so an off-by-one is a repeated splatter
	// and never a 404 the loader remembers for the rest of the session -- and a
	// negative index wraps too, because the pawn overlay picks its splatter from
	// a hash rather than from a counter.
	assert.equal(bloodSprite(9), "/images/blood/1.webp");
	assert.equal(bloodSprite(-1), "/images/blood/9.webp");
});
