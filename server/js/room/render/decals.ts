// The blood on the floor.
//
// IT IS A BURST ON A HIT AND NOT AN AMBIENT DRIP, which is where this parts
// company with the app it replaces. The old one spawned a splatter under every
// wounded pawn every nought to two seconds for as long as the wound lasted --
// so a party with one hurt character pinned a core for the whole session, and
// what the drip told you was only what the ring already said. A burst says
// something the ring cannot: somebody just got hit, right there, this instant.
//
// AND A HIT IS NOW LITERALLY A HIT. What lands here is the difference between
// two hit-point totals, so every blow marks the floor and the mark is sized by
// what the blow was worth against that creature's own maximum -- a scratch on an
// ogre and a scratch on a goblin are not the same event and no longer draw the
// same thing. It was a band transition once, which meant four hits in a row
// could pass without a mark and the fifth threw one for a reason nobody could
// see. See severityOf in wounds.ts.
//
// AND IT DRIES RATHER THAN VANISHING. Fresh blood is bright and mostly opaque;
// over the next six seconds it darkens to a dried maroon at about a third alpha
// and then STOPS, costing nothing for the rest of the evening. The floor of a
// long fight ends up stained where the fight actually happened, which is a map
// that tells you about the session without anybody drawing on it.
//
// THAT STOPPING IS THE WHOLE BUDGET. frame.ts promises that a room nobody is
// touching renders no frames at all, so every animation here is finite by
// construction: a rise, a dry, and then a decal that is a static quad. settling
// is what tells the frame loop the difference.
//
// NOTHING HERE IS SENT ANYWHERE. Decals are per-viewer decoration -- no event,
// no field, no protocol change, and a reload wipes them. What keeps two people
// at the same table seeing the SAME blood is that the jitter, the rotation and
// the choice of splatter are drawn from a generator seeded off the pawn's own
// id: everybody who watched the fight from the same point gets the same floor.
// Somebody who joined halfway through gets a different arrangement of the marks
// made after they arrived, which is a difference nobody at a table can perceive
// and not worth a byte on the wire to fix.

import type { Pawn } from "../protocol.ts";
import type { SpriteCache } from "./sprites.ts";
import { SPRITE_SIZE } from "./sprites.ts";
import {
	BLOOD_DRIED as DRIED, BLOOD_FRESH as FRESH,
	BLOOD_VARIANTS, DEATH_SPLATTERS, bloodSprite, seed, severityOf, splatters,
} from "./wounds.ts";
import { pawnExtents } from "./path.ts";

// BLOOD_PRIORITY is behind every pawn picture, which ask at zero. A portrait
// that has not arrived is a pawn drawn as initials; a splatter that has not
// arrived is a splatter nobody knows was coming.
const BLOOD_PRIORITY = 1;

// RISE_MS is the mark landing: it fades up and settles from slightly too big to
// its real size, which is what makes a hit feel like an impact rather than like
// an image being switched on.
const RISE_MS = 120;
const IMPACT = 1.15;

// DRY_MS and POOL_DRY_MS are how long the colour takes to go from arterial to
// dried. A pool takes longer because there is more of it, which is both true
// and the reason a death reads as a slower, heavier event than a hit.
const DRY_MS = 6000;
const POOL_DRY_MS = 9000;

// WET is the alpha a mark lands at; REST and POOL_REST are what it settles to
// and stays at for ever. Low enough that a floor full of them is scenery rather
// than a red sheet over the map.
const WET_ALPHA = 0.85;
const REST_ALPHA = 0.38;
const POOL_REST_ALPHA = 0.5;

// EVERYTHING BELOW IS MEASURED AGAINST THE CREATURE THAT SHED IT, so a dragon
// throws more blood than a goblin without any of it being written down twice.
//
// HIT_SPREAD_MIN and HIT_SPREAD_MAX are how big a mark is drawn against the
// pawn's radius, and a hit picks between them by how bad it was.
//
// THE SMALLEST IS SMALLER THAN ANYTHING THAT USED TO BE ON THE FLOOR. It has to
// be: every hit marks now, so the mark for a scratch has to read as a fleck
// rather than as a splatter, or a fight fills the room with blood that all means
// the same thing.
const HIT_SPREAD_MIN = 0.5;
const HIT_SPREAD_MAX = 1.6;

// SHOW IS WHERE A MARK'S OUTER EDGE GOES, as a multiple of the pawn's radius,
// and it is what decides how far from the centre the mark is thrown rather than
// a scatter chosen on its own.
//
// THE PAWN IS DRAWN OVER THE FLOOR, WHICH MAKES A SMALL MARK IN THE MIDDLE AN
// INVISIBLE ONE. A scratch throws the smallest mark there is, so if it lands
// under the token it is never seen at all -- which is the bug this replaced a
// plain scatter to fix. Putting the EDGE at a fixed reach instead means the
// offset falls out of the size: a fleck is thrown out to the creature's rim and
// peeks past it, and a mark big enough to cover the token sits over the middle
// and spills out on every side. Nothing has to be tuned twice.
//
// SPREAD_FLOOR keeps the biggest marks off the exact centre so that a death's
// four do not stack, and WANDER is the slop on all of it, so a burst is blood
// rather than a compass rose.
const SHOW = 1.45;
const SPREAD_FLOOR = 0.35;
const WANDER = 0.22;

// DEATH_SPREAD is the spray, POOL_SPREAD the pool underneath it. THE POOL IS THE
// BIGGEST THING THE FLOOR EVER GETS and no hit may reach it: dying is the one
// event on a table that is allowed to be over the top.
const DEATH_SPREAD = 1.6;
const POOL_SPREAD = 2.4;

// VARY_MIN and VARY_MAX are the wobble on a mark's size, so that two hits of the
// same weight are not the same drawing.
const VARY_MIN = 0.85;
const VARY_MAX = 1.15;

// HIT_FAINT is how dim the smallest hit lands, against a full-strength one.
//
// SIZE AND WEIGHT MOVING TOGETHER IS WHAT MAKES IT READ. Size alone is a mark
// you would have to compare against its neighbours to judge; a scratch that is
// both small AND faint is one you can tell was a scratch on its own.
const HIT_FAINT = 0.55;

// TAU is a whole turn. A burst is spread around the creature by giving each mark
// its own sector of one, which is what keeps three marks from landing on top of
// each other however the wander falls.
//
// SECTOR is how much of its own sector a mark may use, and the margin it leaves
// is the point of it: sectors that TOUCH are not a spread, because the two marks
// either side of a boundary can both land on it and end up a couple of degrees
// apart. A mark alone gets the whole turn, since there is nothing to keep it
// away from.
const TAU = Math.PI * 2;
const SECTOR = 0.7;

// CAP is how many marks one floor holds. Past it the oldest is faded out over
// EVICT_MS and dropped -- a long session should stain a room, not bury it.
//
// IT WENT UP WHEN EVERY HIT STARTED MARKING. A fight is fifty or sixty blows
// rather than the dozen band crossings it used to be, and a cap that evicted the
// first round's blood before the last round landed would be a floor that only
// ever remembers the end of a fight.
export const CAP = 160;
const EVICT_MS = 400;

interface Decal {
	x: number;
	y: number;
	half: number;
	rotation: number;
	sprite: string;
	born: number;
	dry: number;

	// wet is the alpha it lands at and rest is what it dries to and stays at.
	// Both are on the mark rather than global because a scratch and a killing
	// blow land at different weights -- see HIT_FAINT.
	wet: number;
	rest: number;

	// evicting is 0 while the mark is staying, and otherwise the moment it began
	// fading out to make room for a newer one.
	evicting: number;
}

// DecalPass is what decals are drawn through. It is named here as an interface
// rather than imported as a class so this module holds no WebGL and its tests
// need no canvas.
export interface DecalTarget {
	begin(): void;
	add(
		x: number, y: number, halfW: number, halfH: number,
		color: readonly [number, number, number], alpha: number,
		layer: number, uvW: number, uvH: number, rotation: number,
	): void;
}

export interface Decals {
	// watch reads every pawn in the store -- on every floor, not just the viewed
	// one -- and sheds blood for the ones that have just lost hit points.
	//
	// IT IS EVERY FLOOR BECAUSE THE MEMORY HAS TO BE. A goblin hurt in the
	// cellar while the GM is looking at the ground floor must not be a goblin
	// that bleeds all over again the moment they go down there, so what happened
	// is recorded everywhere and only the viewed floor is drawn on.
	watch(pawns: readonly Pawn[], viewedID: string, cellSize: number, now: number): void;

	// build fills the pass with one floor's marks. It runs every frame, because
	// the marks are still changing for the first six seconds of their lives.
	build(layerID: string, now: number, sprites: SpriteCache, into: DecalTarget): void;

	// settling is the frame loop's question: is anything still changing? A floor
	// of dried blood answers false, which is what lets the room go quiet.
	settling(now: number): boolean;

	// clear drops one floor's marks. See the caller: it is wired to the same
	// event that wipes a floor's drawing, because "clear this floor" is one
	// gesture at a table and it would be strange for the blood to survive it.
	clear(layerID: string): void;

	// wipe drops every floor's marks, and it is the viewer asking rather than
	// the room.
	//
	// IT IS THE ONE THING IN HERE NOBODY ELSE SEES. What is on these floors was
	// never sent: each browser drew it out of hit points it watched change, so
	// two people at one table have two slightly different floors already -- the
	// one who was reconnecting through the ogre fight has less blood than the
	// one who sat and watched it. Wiping is the same kind of local act, which
	// is why it is a menu item every viewer gets rather than the GM's command.
	//
	// WHAT A PAWN WAS LAST SEEN AT IS KEPT. Forgetting it as well would make
	// the next arriving snapshot look like first sight, and first sight does not
	// bleed -- so the hit that landed while the floor was being cleaned would be
	// the one hit of the evening that left no mark.
	wipe(): void;

	// resync forgets what every pawn's hit points were, WITHOUT dropping a drop
	// of what is already on the floor.
	//
	// IT IS THE RECONNECT. A tab that was asleep or offline through three rounds
	// comes back to a snapshot in which a monster is forty points down, and the
	// difference between what this remembers and what just arrived is not a hit:
	// it is everything that happened while nobody was watching. Bleeding for it
	// would put one enormous mark on the floor for a fight that took place
	// somewhere else. So the next watch records and does not bleed, which is
	// exactly what first sight already does.
	resync(): void;
}

export function newDecals(): Decals {
	const byLayer = new Map<string, Decal[]>();

	// seen is the hit points each pawn was last known to have, and it is what
	// makes this a DIFFERENCE rather than a state. A pawn met for the first time
	// is RECORDED AND NOT BLED FOR: somebody opening the room mid-fight should
	// not be greeted by a burst of blood under every wounded creature in it.
	const seen = new Map<string, Seen>();

	// A reused tuple, because the tint is computed per decal per frame and this
	// is the second performance rule: an allocation in the frame loop is a
	// garbage collection pause during a drag.
	const tint: [number, number, number] = [0, 0, 0];

	function spawn(pawn: Pawn, damage: number, cellSize: number, now: number): void {
		const list = byLayer.get(pawn.layerId) ?? [];
		byLayer.set(pawn.layerId, list);

		const [halfW, halfH] = pawnExtents(pawn, cellSize);
		const radius = Math.max(halfW, halfH);

		// DEATH IS NOT ON THE SCALE, it replaces it. A killing blow is whatever
		// size it happened to be -- a goblin can die to one point -- and what the
		// table needs to see is that something died, not that it was finished off
		// cheaply.
		const dead = pawn.hp !== null && pawn.hp <= 0;
		const strength = dead ? 1 : severityOf(damage, pawn.maxHp);
		const count = dead ? DEATH_SPLATTERS : splatters(strength);

		// A DEATH LEAVES A POOL AS WELL AS A SPRAY, and the pool goes down first
		// so the spray lands on top of it. The skull marks the creature; this
		// marks the floor it fell on, and it is the one that is still there when
		// the body has been cleared away.
		if (dead) {
			pool(list, pawn, radius, now);
		}

		const weight = dead ? 1 : HIT_FAINT + (1 - HIT_FAINT) * strength;
		const reach = dead ? DEATH_SPREAD : HIT_SPREAD_MIN + (HIT_SPREAD_MAX - HIT_SPREAD_MIN) * strength;

		for (let i = 0; i < count; i++) {
			const next = generator(pawn, i);
			const half = radius * reach * (VARY_MIN + next() * (VARY_MAX - VARY_MIN));

			// THROWN OUT BY WHATEVER IT TAKES TO BE SEEN. The distance is not a
			// scatter of its own: it is whatever puts this mark's outer edge at
			// SHOW, so the smaller the mark the further out it goes. See SHOW.
			//
			// AND EACH MARK OF A BURST OWNS A SECTOR of the turn, so a hit that
			// throws three of them throws them around the creature rather than
			// into one pile on whichever side the numbers happened to fall.
			const push = Math.max(radius * SPREAD_FLOOR, radius * SHOW - half);
			const slice = count > 1 ? SECTOR : 1;
			const angle = ((i + (1 - slice) / 2 + slice * next()) * TAU) / count;
			const distance = push + (next() - 0.5) * WANDER * radius;

			list.push({
				x: pawn.x + Math.cos(angle) * distance,
				y: pawn.y + Math.sin(angle) * distance,
				half,
				rotation: Math.floor(next() * 360),
				sprite: bloodSprite(Math.floor(next() * BLOOD_VARIANTS)),
				born: now,
				dry: DRY_MS,
				wet: WET_ALPHA * weight,
				rest: REST_ALPHA * weight,
				evicting: 0,
			});
		}

		evict(list, now);
	}

	// pool is the death pool: centred, barely jittered, and turned only so that
	// two creatures dying on the same square do not leave the same mark twice.
	// It dries slower than a spray because there is more of it, which is both
	// true and the reason a death reads as a heavier event than a hit.
	function pool(list: Decal[], pawn: Pawn, radius: number, now: number): void {
		const next = generator(pawn, POOL_MARK);

		list.push({
			x: pawn.x + (next() - 0.5) * 0.3 * radius,
			y: pawn.y + (next() - 0.5) * 0.3 * radius,
			half: radius * POOL_SPREAD,
			rotation: Math.floor(next() * 360),
			sprite: bloodSprite(Math.floor(next() * BLOOD_VARIANTS)),
			born: now,
			dry: POOL_DRY_MS,
			wet: WET_ALPHA,
			rest: POOL_REST_ALPHA,
			evicting: 0,
		});
	}

	// evict counts only the marks that are STAYING, so a burst that arrives
	// while an earlier eviction is still fading out does not push the cap down
	// past itself and take twice as much floor with it.
	function evict(list: Decal[], now: number): void {
		let staying = 0;
		for (const decal of list) {
			if (decal.evicting === 0) {
				staying++;
			}
		}

		let over = staying - CAP;
		for (let i = 0; i < list.length && over > 0; i++) {
			if (list[i].evicting === 0) {
				list[i].evicting = now;
				over--;
			}
		}
	}

	return {
		watch(pawns, viewedID, cellSize, now) {
			for (const pawn of pawns) {
				// AN OBJECT DOES NOT BLEED. A wagon and a door have hit points
				// and can be broken, and neither of them has any in it.
				if (pawn.kind === "object") {
					continue;
				}

				const was = seen.get(pawn.id);
				seen.set(pawn.id, { hp: pawn.hp, maxHp: pawn.maxHp });

				const damage = hit(was, pawn);
				if (damage === 0) {
					continue;
				}

				// A PAWN PLAYERS CANNOT SEE DOES NOT BLEED EITHER, and this is
				// the GM's copy alone -- nobody else is sent it. An ambusher
				// waiting in the dark is not on the table as far as the table is
				// concerned, and blood under nothing is a mark the GM has to
				// remember the meaning of.
				if (pawn.layerId !== viewedID || !pawn.visible) {
					continue;
				}

				spawn(pawn, damage, cellSize, now);
			}

			// Pawns that have left the table take their memory with them. The
			// blood they shed stays exactly where it was: a body being cleared
			// away does not clean the floor.
			if (seen.size > pawns.length) {
				const here = new Set(pawns.map((pawn) => pawn.id));
				for (const id of seen.keys()) {
					if (!here.has(id)) {
						seen.delete(id);
					}
				}
			}
		},

		build(layerID, now, sprites, into) {
			into.begin();

			const list = byLayer.get(layerID);
			if (!list || list.length === 0) {
				return;
			}

			// The sweep that retires finished evictions, done here because the
			// marks go on animating whether or not the table changed.
			let kept = 0;
			for (const decal of list) {
				if (decal.evicting !== 0 && now - decal.evicting >= EVICT_MS) {
					continue;
				}
				list[kept++] = decal;
			}
			list.length = kept;

			for (const decal of list) {
				const slot = sprites.sprite(decal.sprite, BLOOD_PRIORITY);
				if (!slot) {
					continue;
				}

				const age = now - decal.born;
				const rise = clamp(age / RISE_MS);
				const dried = clamp((age - RISE_MS) / decal.dry);

				let alpha = (decal.wet + (decal.rest - decal.wet) * dried) * rise;
				if (decal.evicting !== 0) {
					alpha *= 1 - clamp((now - decal.evicting) / EVICT_MS);
				}

				tint[0] = FRESH[0] + (DRIED[0] - FRESH[0]) * dried;
				tint[1] = FRESH[1] + (DRIED[1] - FRESH[1]) * dried;
				tint[2] = FRESH[2] + (DRIED[2] - FRESH[2]) * dried;

				// A MARK AT ZERO ALPHA IS STILL SENT, which is the frame it lands
				// on and the frame an eviction finishes. The shader discards it
				// for nothing, and skipping it here would mean the pool's idea of
				// what is on the floor and the pass's disagreed for a frame.
				const half = decal.half * (IMPACT + (1 - IMPACT) * rise);

				into.add(
					decal.x, decal.y, half, half,
					tint, alpha,
					slot.layer, slot.w / SPRITE_SIZE, slot.h / SPRITE_SIZE,
					decal.rotation,
				);
			}
		},

		settling(now) {
			for (const list of byLayer.values()) {
				for (const decal of list) {
					if (decal.evicting !== 0 || now - decal.born < RISE_MS + decal.dry) {
						return true;
					}
				}
			}

			return false;
		},

		clear(layerID) {
			byLayer.delete(layerID);
		},

		wipe() {
			byLayer.clear();
		},

		resync() {
			seen.clear();
		},
	};
}

// Seen is what a pawn's hit points were the last time this looked. The maximum
// is remembered as well as the number so that a GM correcting a stat line can be
// told apart from a creature being hit; see hit.
interface Seen {
	hp: number | null;
	maxHp: number | null;
}

// hit is how much damage a pawn has just taken, or zero for anything that is not
// a hit. EVERY GUARD IN IT IS A WAY OF NOT BLEEDING, and each one is a case that
// came up rather than a precaution.
function hit(was: Seen | undefined, pawn: Pawn): number {
	// The first sight of a pawn, or one whose hit points nobody ever wrote down.
	// Recorded by the caller, never bled for.
	if (was === undefined || was.hp === null || pawn.hp === null) {
		return 0;
	}

	// A CORPSE DOES NOT BLEED AGAIN. A GM taking something already at zero down
	// to minus six is bookkeeping, and the pool is on the floor already.
	if (was.hp <= 0) {
		return 0;
	}

	// THE STAT LINE CHANGED RATHER THAN THE CREATURE. A GM fixing a maximum they
	// typed wrong is not a hit, and the fraction it would be measured against is
	// the number that just moved.
	if (was.maxHp !== pawn.maxHp) {
		return 0;
	}

	const damage = was.hp - pawn.hp;

	return damage > 0 ? damage : 0;
}

// POOL_MARK is the death pool's place in the sequence below. It is negative so
// that it cannot collide with a spray's index, whatever the spray's length.
const POOL_MARK = -1;

// generator is a small deterministic source, seeded off the pawn's id, the hit
// points it landed on and which mark of the burst this is. Math.random would put
// different blood on every screen at the table for no gain.
//
// THE HIT POINTS ARE THE KEY RATHER THAN A COUNTER, and that is what keeps a
// table in agreement now that everybody is sent the numbers: two people watching
// the same blow land compute the same seed from it, whether or not they were
// both in the room for the ones before it.
function generator(pawn: Pawn, nth: number): () => number {
	let state = seed(`${pawn.id}:${pawn.hp ?? "?"}:${nth}`);

	return () => {
		state = (Math.imul(state, 1664525) + 1013904223) >>> 0;

		return state / 0x100000000;
	};
}

function clamp(value: number): number {
	return value < 0 ? 0 : value > 1 ? 1 : value;
}
