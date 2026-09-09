// The blood on the floor.
//
// IT IS A BURST ON A HIT AND NOT AN AMBIENT DRIP, which is where this parts
// company with the app it replaces. The old one spawned a splatter under every
// wounded pawn every nought to two seconds for as long as the wound lasted --
// so a party with one hurt character pinned a core for the whole session, and
// what the drip told you was only what the ring already said. A burst says
// something the ring cannot: somebody just got hit, right there, this instant.
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
import type { HPBand } from "../protocol.ts";
import type { SpriteCache } from "./sprites.ts";
import { SPRITE_SIZE } from "./sprites.ts";
import { healthOf, splatters, worsened } from "./wounds.ts";
import { pawnExtents } from "./path.ts";

// VARIANTS is how many splatters the sheet was cut into. They live under
// /images/blood and are 256 square, which is the sprite cache's layer size, so
// each is an ordinary cache entry and none of it is special-cased.
const VARIANTS = 9;

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

// FRESH and DRIED are what the sheet's pure red is multiplied by. The art has no
// desaturation anywhere in it -- the green and blue channels are essentially
// zero -- so the red channel carries all of the shape's shading and a tint
// against it recolours the whole set from one texture. That is what drying is,
// and it is the door through which a green-blooded ooze walks later.
const FRESH: readonly [number, number, number] = [1, 0.13, 0.1];
const DRIED: readonly [number, number, number] = [0.34, 0.06, 0.05];

// JITTER is how far from the pawn's centre a mark may land, as a fraction of its
// radius, and SPREAD_MIN/MAX are how big it is drawn against the same radius. A
// dragon throws more blood than a goblin because both of these are measured
// against the creature that shed it.
const JITTER = 0.55;
const SPREAD_MIN = 1.1;
const SPREAD_MAX = 1.7;
const POOL_SPREAD = 2.4;

// CAP is how many marks one floor holds. Past it the oldest is faded out over
// EVICT_MS and dropped -- a long session should stain a room, not bury it.
export const CAP = 96;
const EVICT_MS = 400;

interface Decal {
	x: number;
	y: number;
	half: number;
	rotation: number;
	sprite: string;
	born: number;
	dry: number;
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
	// one -- and sheds blood for the ones whose health has just got worse.
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
}

export function newDecals(): Decals {
	const byLayer = new Map<string, Decal[]>();

	// seen is the band each pawn was last known to be in, and it is what makes
	// this a transition rather than a state. A pawn met for the first time is
	// RECORDED AND NOT BLED FOR: somebody opening the room mid-fight should not
	// be greeted by a burst of blood under every wounded creature in it.
	const seen = new Map<string, HPBand | null>();

	// thrown counts the marks each pawn has shed, and its only job is to make
	// the second splatter of a burst land somewhere other than the first.
	const thrown = new Map<string, number>();

	// A reused tuple, because the tint is computed per decal per frame and this
	// is the second performance rule: an allocation in the frame loop is a
	// garbage collection pause during a drag.
	const tint: [number, number, number] = [0, 0, 0];

	function spawn(pawn: Pawn, band: HPBand, cellSize: number, now: number): void {
		const count = splatters(band);
		if (count === 0) {
			return;
		}

		const list = byLayer.get(pawn.layerId) ?? [];
		byLayer.set(pawn.layerId, list);

		const [halfW, halfH] = pawnExtents(pawn, cellSize);
		const radius = Math.max(halfW, halfH);
		const already = thrown.get(pawn.id) ?? 0;

		// A DEATH LEAVES A POOL AS WELL AS A SPRAY, and the pool goes down first
		// so the spray lands on top of it. The skull marks the creature; this
		// marks the floor it fell on, and it is the one that is still there when
		// the body has been cleared away.
		if (band === "dead") {
			place(list, pawn, radius * POOL_SPREAD, already, now, POOL_DRY_MS, POOL_REST_ALPHA, radius);
		}

		for (let i = 0; i < count; i++) {
			const seed = already + i + 1;
			const next = generator(pawn.id, band, seed);
			const spread = SPREAD_MIN + next() * (SPREAD_MAX - SPREAD_MIN);

			list.push({
				x: pawn.x + (next() - 0.5) * 2 * JITTER * radius,
				y: pawn.y + (next() - 0.5) * 2 * JITTER * radius,
				half: radius * spread,
				rotation: Math.floor(next() * 360),
				sprite: spriteFor(Math.floor(next() * VARIANTS)),
				born: now,
				dry: DRY_MS,
				rest: REST_ALPHA,
				evicting: 0,
			});
		}

		thrown.set(pawn.id, already + count + 1);
		evict(list, now);
	}

	// place is the death pool: centred, unjittered, and turned only so that two
	// creatures dying on the same square do not leave the same mark twice.
	function place(
		list: Decal[], pawn: Pawn, half: number, seed: number,
		now: number, dry: number, rest: number, radius: number,
	): void {
		const next = generator(pawn.id, "dead", seed);

		list.push({
			x: pawn.x + (next() - 0.5) * 0.3 * radius,
			y: pawn.y + (next() - 0.5) * 0.3 * radius,
			half,
			rotation: Math.floor(next() * 360),
			sprite: spriteFor(Math.floor(next() * VARIANTS)),
			born: now,
			dry,
			rest,
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

				const band = healthOf(pawn);
				const was = seen.get(pawn.id);
				seen.set(pawn.id, band);

				// Nothing to compare against: either this is the first sight of
				// the pawn, or the viewer has never been told anything about its
				// health -- a monster in a room whose labels are off, which
				// never bleeds because nobody watching it knows it was hit.
				if (band === null || was === undefined || was === null) {
					continue;
				}
				if (!worsened(was, band)) {
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

				spawn(pawn, band, cellSize, now);
			}

			// Pawns that have left the table take their memory with them. The
			// blood they shed stays exactly where it was: a body being cleared
			// away does not clean the floor.
			if (seen.size > pawns.length) {
				const here = new Set(pawns.map((pawn) => pawn.id));
				for (const id of seen.keys()) {
					if (!here.has(id)) {
						seen.delete(id);
						thrown.delete(id);
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

				let alpha = (WET_ALPHA + (decal.rest - WET_ALPHA) * dried) * rise;
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
	};
}

// spriteFor is the URL of one of the nine. They are numbered from one because
// that is what the sheet they were cut from looks like.
export function spriteFor(variant: number): string {
	return `/images/blood/${(variant % VARIANTS) + 1}.webp`;
}

// generator is a small deterministic source, seeded off the pawn's id, the band
// it landed in and which mark of the burst this is. Math.random would put
// different blood on every screen at the table for no gain.
function generator(id: string, band: HPBand, nth: number): () => number {
	let state = hash(`${id}:${band}:${nth}`);

	return () => {
		state = (Math.imul(state, 1664525) + 1013904223) >>> 0;

		return state / 0x100000000;
	};
}

// hash is FNV-1a, which is enough to turn a ULID into a seed and short enough to
// read.
function hash(text: string): number {
	let value = 0x811c9dc5;
	for (let i = 0; i < text.length; i++) {
		value ^= text.charCodeAt(i);
		value = Math.imul(value, 0x01000193) >>> 0;
	}

	return value >>> 0;
}

function clamp(value: number): number {
	return value < 0 ? 0 : value > 1 ? 1 : value;
}
