// The blood, which is entirely the client's: no event carries it, no field
// holds it, and the server has never heard of it. That makes these tests the
// only thing standing between a wrong rule and a table covered in marks nobody
// can account for.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { HPBand, Pawn } from "../protocol.ts";
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
	assert.ok(hit.length > 0, "a creature that lost four of its seven hit points shed no blood");

	// Healed back up, which is not a hit however far it moves the number.
	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 20);
	assert.equal(drawn(decals, 20).length, hit.length, "being healed shed blood");

	// And hit for the same four points again, which throws a second burst --
	// the DIFFERENCE is the event, not the band it happens to land in.
	decals.watch([pawn({ hp: 3, maxHp: 7 })], GROUND, CELL, 30);
	assert.equal(drawn(decals, 30).length, hit.length * 2);
});

// EVERY HIT MARKS THE FLOOR, AND THAT IS THE WHOLE CHANGE. It was a band
// crossing once, so four hits in a row could pass without a mark and the fifth
// threw one for a reason nobody at the table could see. What a scratch gets now
// is a scratch -- small, faint and thrown close in -- rather than nothing.
test("every hit marks the floor, including the ones that cross nothing", () => {
	const decals = newDecals();
	decals.watch([pawn({ hp: 100, maxHp: 100 })], GROUND, CELL, 0);

	// Four ordinary hits, none of them crossing a band, every one of them well
	// above the halfway line the old rule waited for.
	let shed = 0;
	for (const [at, hp] of [[10, 99], [20, 80], [30, 76], [40, 60]] as const) {
		decals.watch([pawn({ hp, maxHp: 100 })], GROUND, CELL, at);

		const marks = drawn(decals, at);
		assert.ok(marks.length > shed, `a hit taking it to ${hp} of 100 drew nothing`);
		shed = marks.length;
	}
});

// SIZE, WEIGHT AND SCATTER ALL MOVE TOGETHER. Any one of them alone is a mark
// you would have to compare against its neighbours to judge; the three at once
// is a hit you can tell was heavy without looking at anything else.
test("a heavy blow is bigger and bolder than a scratch", () => {
	const scratch = newDecals();
	scratch.watch([pawn({ hp: 100, maxHp: 100 })], GROUND, CELL, 0);
	scratch.watch([pawn({ hp: 99, maxHp: 100 })], GROUND, CELL, 10);

	const blow = newDecals();
	blow.watch([pawn({ hp: 100, maxHp: 100 })], GROUND, CELL, 0);
	blow.watch([pawn({ hp: 40, maxHp: 100 })], GROUND, CELL, 10);

	const small = drawn(scratch, 5000);
	const large = drawn(blow, 5000);

	const widest = (marks: Mark[]) => Math.max(...marks.map((mark) => mark.half));
	const boldest = (marks: Mark[]) => Math.max(...marks.map((mark) => mark.alpha));

	assert.ok(large.length >= small.length, "a heavy blow threw fewer marks than a scratch");
	assert.ok(widest(large) > widest(small) * 1.5, "sixty points drew the same size mark as one");
	assert.ok(boldest(large) > boldest(small), "a scratch was as bold as a heavy blow");
});

// A MEDIUM CREATURE IS ONE CELL ACROSS, so its own radius is half a cell. The
// two tests below are about where a mark lands relative to that edge.
const RADIUS = CELL / 2;

// Marks are drawn UNDER the pawn, so a small one in the middle is one nobody
// ever sees -- and a scratch throws the smallest mark there is. Every mark has
// to reach past the creature's own edge, whatever its size.
test("every mark reaches past the edge of the pawn that shed it", () => {
	for (const [name, hp] of [["a scratch", 99], ["a solid hit", 80], ["a crit", 41], ["a death", 0]] as const) {
		const decals = newDecals();
		decals.watch([pawn({ hp: 100, maxHp: 100 })], GROUND, CELL, 0);
		decals.watch([pawn({ hp, maxHp: 100 })], GROUND, CELL, 10);

		for (const mark of drawn(decals, 5000)) {
			const from = Math.hypot(mark.x - 100, mark.y - 200);

			assert.ok(from + mark.half > RADIUS * 1.3, `${name} left a mark buried under the token`);
		}
	}
});

// AND THE OFFSET IS THE SIZE, NOT A SCATTER OF ITS OWN. A fleck is thrown out to
// the creature's rim because that is the only way it is visible; a mark big
// enough to cover the token sits over the middle and spills out on every side.
test("a small mark is thrown to the rim and a big one sits over the middle", () => {
	const nearest = (hp: number): number => {
		const decals = newDecals();
		decals.watch([pawn({ hp: 100, maxHp: 100 })], GROUND, CELL, 0);
		decals.watch([pawn({ hp, maxHp: 100 })], GROUND, CELL, 10);

		return Math.min(...drawn(decals, 5000).map((mark) => Math.hypot(mark.x - 100, mark.y - 200)));
	};

	assert.ok(nearest(99) > nearest(41), "a scratch sat no further out than a crit");
	assert.ok(nearest(99) > RADIUS * 0.7, "a scratch was not thrown anywhere near the rim");
	assert.ok(nearest(41) < RADIUS * 0.6, "a crit was thrown out instead of covering the token");
});

// Three marks at once go AROUND the creature rather than into one pile on
// whichever side the numbers happened to fall: each owns a sector of the turn.
test("a burst is thrown around the creature and not into one pile", () => {
	const decals = newDecals();
	decals.watch([pawn({ hp: 100, maxHp: 100 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 20, maxHp: 100 })], GROUND, CELL, 10);

	const marks = drawn(decals, 5000);
	assert.ok(marks.length >= 3, "the test did not actually throw a burst");

	// IT IS THE ANGLES THAT ARE SPREAD, not the distances. Three marks big
	// enough to cover the token OVERLAP, and should: what would read as a pile
	// is three of them thrown to the same side of the creature.
	const angles = marks.map((mark) => Math.atan2(mark.y - 200, mark.x - 100));

	for (let i = 0; i < angles.length; i++) {
		for (let j = i + 1; j < angles.length; j++) {
			const apart = Math.abs(((angles[i] - angles[j] + Math.PI * 3) % (Math.PI * 2)) - Math.PI);

			assert.ok(apart > Math.PI / 6, "two marks of one burst were thrown the same way");
		}
	}
});

// A CREATURE NOBODY WROTE HIT POINTS FOR CANNOT BE SEEN TO BE HIT. There is no
// number to take the difference of, so nothing is drawn and nothing is guessed.
test("a pawn with no hit points at all sheds nothing", () => {
	const decals = newDecals();

	const blank = { hp: null, maxHp: null, hpBand: null };
	decals.watch([pawn(blank)], GROUND, CELL, 0);
	decals.watch([pawn(blank)], GROUND, CELL, 10);
	assert.deepEqual(drawn(decals, 10), []);
});

// THE GM AND THE PARTY SEE THE SAME BLOOD, and that is what the room's label
// setting stopped costing. Hit points reach every viewer now -- see projectPawn
// in internal/room/snapshot.go -- and only the TEXT differs, so a player being
// shown a word watches a monster bleed at exactly the weight the GM does.
test("a player sees the same blood as the GM, word or number", () => {
	// A GM's copy carries hit points and no band, because a GM's copy is never
	// projected. A player's copy of the same monster in an ordinary room carries
	// the same hit points with a band beside them.
	const asGM = (hp: number): Pawn => pawn({ hp, maxHp: 40 });
	const asPlayer = (hp: number, band: HPBand): Pawn => pawn({ hp, maxHp: 40, hpBand: band });

	const theirs = newDecals();
	const ours = newDecals();
	for (const [at, hp, band] of [[0, 40, "healthy"], [10, 31, "bruised"], [20, 12, "bloody"]] as const) {
		theirs.watch([asGM(hp)], GROUND, CELL, at);
		ours.watch([asPlayer(hp, band)], GROUND, CELL, at);
	}

	assert.ok(drawn(theirs, 20).length > 0, "the GM saw no blood at all");
	assert.deepEqual(drawn(ours, 20), drawn(theirs, 20));
});

// A GM taking something already at zero down to minus six is bookkeeping, and
// the pool it is lying in went down when it died.
test("a corpse does not bleed again", () => {
	const decals = newDecals();

	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 0, maxHp: 7 })], GROUND, CELL, 10);
	const died = drawn(decals, 10).length;

	decals.watch([pawn({ hp: -6, maxHp: 7 })], GROUND, CELL, 20);
	assert.equal(drawn(decals, 20).length, died, "a corpse bled again");
});

// A GM fixing a maximum they typed wrong is not a hit, and the fraction it would
// be measured against is the number that just moved.
test("correcting a stat line is not a hit", () => {
	const decals = newDecals();

	decals.watch([pawn({ hp: 60, maxHp: 100 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 40, maxHp: 60 })], GROUND, CELL, 10);
	assert.deepEqual(drawn(decals, 10), [], "a corrected stat block sprayed the floor");

	// And the next real hit bleeds normally.
	decals.watch([pawn({ hp: 30, maxHp: 60 })], GROUND, CELL, 20);
	assert.ok(drawn(decals, 20).length > 0);
});

// A TAB THAT WAS ASLEEP MISSED A FIGHT RATHER THAN TOOK ONE HIT. What arrives on
// a reconnect is the table as it is NOW, and the difference from what this
// remembers is three rounds of combat nobody in this browser watched.
test("a reconnection is not one enormous hit", () => {
	const decals = newDecals();

	decals.watch([pawn({ hp: 100, maxHp: 100 })], GROUND, CELL, 0);
	decals.resync();
	decals.watch([pawn({ hp: 4, maxHp: 100 })], GROUND, CELL, 10);
	assert.deepEqual(drawn(decals, 10), [], "a reconnection bled for a fight nobody watched");

	// The first hit after it is an ordinary one.
	decals.watch([pawn({ hp: 2, maxHp: 100 })], GROUND, CELL, 20);
	assert.ok(drawn(decals, 20).length > 0);
});

// It forgets the NUMBERS and not the FLOOR. Blood already down was shed by hits
// this browser did watch, and a reconnection is not a mop.
test("resyncing keeps the blood already on the floor", () => {
	const decals = newDecals();

	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 3, maxHp: 7 })], GROUND, CELL, 10);
	const shed = drawn(decals, 10).length;
	assert.ok(shed > 0);

	decals.resync();
	assert.equal(drawn(decals, 10).length, shed, "a reconnection wiped the floor");
});

// DYING IS THE ONE EVENT ON A TABLE ALLOWED TO BE OVER THE TOP, and it has to
// stay that way whatever a critical hit does. A creature left on one hit point
// is the worst a living thing can look, and it is still visibly not a death.
test("no hit is ever as big as a death", () => {
	const killed = newDecals();
	killed.watch([pawn({ hp: 40, maxHp: 40 })], GROUND, CELL, 0);
	killed.watch([pawn({ hp: 0, maxHp: 40 })], GROUND, CELL, 10);

	const mauled = newDecals();
	mauled.watch([pawn({ hp: 40, maxHp: 40 })], GROUND, CELL, 0);
	mauled.watch([pawn({ hp: 1, maxHp: 40 })], GROUND, CELL, 10);

	const death = drawn(killed, 10);
	const worst = drawn(mauled, 10);

	assert.ok(
		Math.max(...death.map((mark) => mark.half)) > Math.max(...worst.map((mark) => mark.half)),
		"something that survived left a bigger mark than something that died",
	);
	assert.ok(death.length > worst.length, "dying threw no more than surviving");
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

// WIPING TAKES EVERY FLOOR AND NOT THE ONE BEING LOOKED AT. Somebody reaching
// for Clear blood wants a clean table, and a version of it that left the cellar
// red would have to be pressed once per floor by a person who cannot see the
// floors they are pressing it for.
test("a wipe cleans the whole tower and not the storey in front of you", () => {
	const decals = newDecals();

	// Blood lands on the floor being watched, so a fight on each floor is two
	// fights with a walk downstairs between them.
	const upstairs = pawn({ id: "01UP", hp: 7, maxHp: 7 });
	decals.watch([upstairs], GROUND, CELL, 0);
	decals.watch([{ ...upstairs, hp: 0 }], GROUND, CELL, 10);

	const downstairs = pawn({ id: "01DOWN", layerId: CELLAR, hp: 7, maxHp: 7 });
	decals.watch([downstairs], CELLAR, CELL, 20);
	decals.watch([{ ...downstairs, hp: 0 }], CELLAR, CELL, 30);

	assert.ok(drawn(decals, 30).length > 0, "the ground floor never bled");
	assert.ok(drawn(decals, 30, CELLAR).length > 0, "the cellar never bled");

	decals.wipe();

	assert.deepEqual(drawn(decals, 30), [], "the ground floor kept its blood");
	assert.deepEqual(drawn(decals, 30, CELLAR), [], "the cellar kept its blood");
	assert.equal(decals.settling(30), false);
});

// AND IT IS NOT A RESYNC. Forgetting what every pawn was last seen at would make
// the next snapshot look like first sight, and first sight does not bleed -- so
// a hit that landed while somebody was tidying would be the one hit of the
// evening that left no mark.
test("a wipe does not swallow the next hit", () => {
	const decals = newDecals();

	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.wipe();

	decals.watch([pawn({ hp: 3, maxHp: 7 })], GROUND, CELL, 10);
	assert.ok(drawn(decals, 10).length > 0, "the hit after a wipe was read as first sight");
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

// THE ACCOUNT SETTING, which is the one thing in here a person asks for by
// name. Everything else in this file is a rule about a fight; this is a rule
// about whether the fight is drawn in blood at all.
test("a viewer who turned it off is not bled for", () => {
	const decals = newDecals();
	decals.show(false);

	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 3, maxHp: 7 })], GROUND, CELL, 10);

	assert.equal(drawn(decals, 10).length, 0);
});

// SAYING SO MID-FIGHT CLEANS THE FLOOR AS WELL. "No blood please" is about the
// four rounds already down there, not only about the fifth -- and it costs the
// table nothing, because none of it was ever anybody else's.
test("turning it off takes down what is already there", () => {
	const decals = newDecals();

	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 3, maxHp: 7 })], GROUND, CELL, 10);
	assert.ok(drawn(decals, 10).length > 0, "nothing was on the floor to take down");

	decals.show(false);
	assert.equal(drawn(decals, 10).length, 0);
});

// AND TURNING IT BACK ON DOES NOT PAY OUT THE ARREARS. The hit points are
// recorded while it is off, so what resumes is the next blow -- not one
// enormous mark for everything that landed while nobody was drawing it. This is
// the same failure resync exists to prevent, arrived at from the other side.
test("turning it back on starts from the table as it stands", () => {
	const decals = newDecals();
	decals.show(false);

	decals.watch([pawn({ hp: 40, maxHp: 40 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 4, maxHp: 40 })], GROUND, CELL, 10);

	decals.show(true);
	assert.equal(drawn(decals, 20).length, 0, "the fight it did not draw was paid out in one blow");

	decals.watch([pawn({ hp: 1, maxHp: 40 })], GROUND, CELL, 30);
	assert.ok(drawn(decals, 30).length > 0, "the next hit after it came back did not mark");
});

// The floor goes quiet as well, which is what the frame loop reads. A viewer who
// turned the blood off should not be paying for frames that draw nothing; see
// settling, and frame.ts.
test("a floor with the blood turned off has nothing left to settle", () => {
	const decals = newDecals();

	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 3, maxHp: 7 })], GROUND, CELL, 10);
	assert.ok(decals.settling(10), "a fresh mark is not settling");

	decals.show(false);
	assert.equal(decals.settling(10), false);
});
