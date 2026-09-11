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
	decals.watch([pawn({ hp: 1, maxHp: 20 })], GROUND, CELL, 0);
	assert.deepEqual(drawn(decals, 0), []);
	decals.watch([pawn({ hp: 1, maxHp: 20 })], GROUND, CELL, 100);
	assert.deepEqual(drawn(decals, 100), []);
});
test("blood is shed on the way down and not on the way back up", () => {
	const decals = newDecals();
	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 3, maxHp: 7 })], GROUND, CELL, 10);
	const hit = drawn(decals, 10);
	assert.ok(hit.length > 0, "a creature that lost four of its seven hit points shed no blood");
	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 20);
	assert.equal(drawn(decals, 20).length, hit.length, "being healed shed blood");
	decals.watch([pawn({ hp: 3, maxHp: 7 })], GROUND, CELL, 30);
	assert.equal(drawn(decals, 30).length, hit.length * 2);
});
test("every hit marks the floor, including the ones that cross nothing", () => {
	const decals = newDecals();
	decals.watch([pawn({ hp: 100, maxHp: 100 })], GROUND, CELL, 0);
	let shed = 0;
	for (const [at, hp] of [[10, 99], [20, 80], [30, 76], [40, 60]] as const) {
		decals.watch([pawn({ hp, maxHp: 100 })], GROUND, CELL, at);
		const marks = drawn(decals, at);
		assert.ok(marks.length > shed, `a hit taking it to ${hp} of 100 drew nothing`);
		shed = marks.length;
	}
});
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
const RADIUS = CELL / 2;
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
test("a burst is thrown around the creature and not into one pile", () => {
	const decals = newDecals();
	decals.watch([pawn({ hp: 100, maxHp: 100 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 20, maxHp: 100 })], GROUND, CELL, 10);
	const marks = drawn(decals, 5000);
	assert.ok(marks.length >= 3, "the test did not actually throw a burst");
	const angles = marks.map((mark) => Math.atan2(mark.y - 200, mark.x - 100));
	for (let i = 0; i < angles.length; i++) {
		for (let j = i + 1; j < angles.length; j++) {
			const apart = Math.abs(((angles[i] - angles[j] + Math.PI * 3) % (Math.PI * 2)) - Math.PI);
			assert.ok(apart > Math.PI / 6, "two marks of one burst were thrown the same way");
		}
	}
});
test("a pawn with no hit points at all sheds nothing", () => {
	const decals = newDecals();
	const blank = { hp: null, maxHp: null, hpBand: null };
	decals.watch([pawn(blank)], GROUND, CELL, 0);
	decals.watch([pawn(blank)], GROUND, CELL, 10);
	assert.deepEqual(drawn(decals, 10), []);
});
test("a player sees the same blood as the GM, word or number", () => {
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
test("a corpse does not bleed again", () => {
	const decals = newDecals();
	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 0, maxHp: 7 })], GROUND, CELL, 10);
	const died = drawn(decals, 10).length;
	decals.watch([pawn({ hp: -6, maxHp: 7 })], GROUND, CELL, 20);
	assert.equal(drawn(decals, 20).length, died, "a corpse bled again");
});
test("correcting a stat line is not a hit", () => {
	const decals = newDecals();
	decals.watch([pawn({ hp: 60, maxHp: 100 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 40, maxHp: 60 })], GROUND, CELL, 10);
	assert.deepEqual(drawn(decals, 10), [], "a corrected stat block sprayed the floor");
	decals.watch([pawn({ hp: 30, maxHp: 60 })], GROUND, CELL, 20);
	assert.ok(drawn(decals, 20).length > 0);
});
test("a reconnection is not one enormous hit", () => {
	const decals = newDecals();
	decals.watch([pawn({ hp: 100, maxHp: 100 })], GROUND, CELL, 0);
	decals.resync();
	decals.watch([pawn({ hp: 4, maxHp: 100 })], GROUND, CELL, 10);
	assert.deepEqual(drawn(decals, 10), [], "a reconnection bled for a fight nobody watched");
	decals.watch([pawn({ hp: 2, maxHp: 100 })], GROUND, CELL, 20);
	assert.ok(drawn(decals, 20).length > 0);
});
test("resyncing keeps the blood already on the floor", () => {
	const decals = newDecals();
	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 3, maxHp: 7 })], GROUND, CELL, 10);
	const shed = drawn(decals, 10).length;
	assert.ok(shed > 0);
	decals.resync();
	assert.equal(drawn(decals, 10).length, shed, "a reconnection wiped the floor");
});
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
	const wagon = { id: "wagon", kind: "object" as const, width: 128, height: 256 };
	decals.watch([pawn({ ...wagon, hp: 30, maxHp: 30 })], GROUND, CELL, 0);
	decals.watch([pawn({ ...wagon, hp: 2, maxHp: 30 })], GROUND, CELL, 10);
	assert.deepEqual(drawn(decals, 10), [], "an object bled");
	decals.watch([pawn({ id: "hidden", visible: false, hp: 7, maxHp: 7 })], GROUND, CELL, 20);
	decals.watch([pawn({ id: "hidden", visible: false, hp: 1, maxHp: 7 })], GROUND, CELL, 30);
	assert.deepEqual(drawn(decals, 30), [], "a pawn players cannot see bled");
});
test("a hit on another floor is remembered rather than saved up", () => {
	const decals = newDecals();
	decals.watch([pawn({ layerId: CELLAR, hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.watch([pawn({ layerId: CELLAR, hp: 1, maxHp: 7 })], GROUND, CELL, 10);
	decals.watch([pawn({ layerId: CELLAR, hp: 1, maxHp: 7 })], CELLAR, CELL, 20);
	assert.deepEqual(drawn(decals, 20, CELLAR), [], "blood was saved up for a floor change");
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
	const biggest = Math.max(...marks.map((mark) => mark.half));
	const rest = marks.map((mark) => mark.half).filter((half) => half !== biggest);
	assert.ok(biggest > Math.max(...rest) * 1.3, "a death left nothing bigger than a scratch");
});
test("a bigger creature throws bigger marks", () => {
	const small = newDecals();
	small.watch([pawn({ size: "tiny", hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	small.watch([pawn({ size: "tiny", hp: 1, maxHp: 7 })], GROUND, CELL, 10);
	const large = newDecals();
	large.watch([pawn({ size: "gargantuan", hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	large.watch([pawn({ size: "gargantuan", hp: 1, maxHp: 7 })], GROUND, CELL, 10);
	assert.ok(drawn(large, 10)[0].half > drawn(small, 10)[0].half * 4);
});
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
	assert.ok(landing.half > wet.half);
	assert.deepEqual(later, dried);
});
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
	assert.ok(new Set(ours.map((mark) => `${mark.x},${mark.y}`)).size > 1);
	assert.ok(new Set(ours.map((mark) => mark.rotation)).size > 1);
});
test("a floor holds a bounded amount of blood", () => {
	const decals = newDecals();
	const party = (hp: number): Pawn[] =>
		Array.from({ length: 40 }, (_, i) => pawn({ id: `pawn-${i}`, x: i * 10, hp, maxHp: 20 }));
	decals.watch(party(20), GROUND, CELL, 0);
	decals.watch(party(9), GROUND, CELL, 10);
	decals.watch(party(0), GROUND, CELL, 20);
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
test("a wipe cleans the whole tower and not the storey in front of you", () => {
	const decals = newDecals();
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
test("a wipe does not swallow the next hit", () => {
	const decals = newDecals();
	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.wipe();
	decals.watch([pawn({ hp: 3, maxHp: 7 })], GROUND, CELL, 10);
	assert.ok(drawn(decals, 10).length > 0, "the hit after a wipe was read as first sight");
});
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
	assert.equal(bloodSprite(9), "/images/blood/1.webp");
	assert.equal(bloodSprite(-1), "/images/blood/9.webp");
});
test("a viewer who turned it off is not bled for", () => {
	const decals = newDecals();
	decals.show(false);
	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 3, maxHp: 7 })], GROUND, CELL, 10);
	assert.equal(drawn(decals, 10).length, 0);
});
test("turning it off takes down what is already there", () => {
	const decals = newDecals();
	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 3, maxHp: 7 })], GROUND, CELL, 10);
	assert.ok(drawn(decals, 10).length > 0, "nothing was on the floor to take down");
	decals.show(false);
	assert.equal(drawn(decals, 10).length, 0);
});
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
test("a floor with the blood turned off has nothing left to settle", () => {
	const decals = newDecals();
	decals.watch([pawn({ hp: 7, maxHp: 7 })], GROUND, CELL, 0);
	decals.watch([pawn({ hp: 3, maxHp: 7 })], GROUND, CELL, 10);
	assert.ok(decals.settling(10), "a fresh mark is not settling");
	decals.show(false);
	assert.equal(decals.settling(10), false);
});
