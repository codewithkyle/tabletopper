











































import type { Pawn } from "../protocol.ts";
import type { SpriteCache } from "./sprites.ts";
import { SPRITE_SIZE } from "./sprites.ts";
import {
	BLOOD_DRIED as DRIED, BLOOD_FRESH as FRESH,
	BLOOD_VARIANTS, DEATH_SPLATTERS, bloodSprite, seed, severityOf, splatters,
} from "./wounds.ts";
import { pawnExtents } from "./path.ts";




const BLOOD_PRIORITY = 1;




const RISE_MS = 120;
const IMPACT = 1.15;




const DRY_MS = 6000;
const POOL_DRY_MS = 9000;




const WET_ALPHA = 0.85;
const REST_ALPHA = 0.38;
const POOL_REST_ALPHA = 0.5;











const HIT_SPREAD_MIN = 0.5;
const HIT_SPREAD_MAX = 1.6;
















const SHOW = 1.45;
const SPREAD_FLOOR = 0.35;
const WANDER = 0.22;




const DEATH_SPREAD = 1.6;
const POOL_SPREAD = 2.4;



const VARY_MIN = 0.85;
const VARY_MAX = 1.15;






const HIT_FAINT = 0.55;










const TAU = Math.PI * 2;
const SECTOR = 0.7;








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

	
	
	
	wet: number;
	rest: number;

	
	
	evicting: number;
}




export interface DecalTarget {
	begin(): void;
	add(
		x: number, y: number, halfW: number, halfH: number,
		color: readonly [number, number, number], alpha: number,
		layer: number, uvW: number, uvH: number, rotation: number,
	): void;
}

export interface Decals {
	
	
	
	
	
	
	
	watch(pawns: readonly Pawn[], viewedID: string, cellSize: number, now: number): void;

	
	
	build(layerID: string, now: number, sprites: SpriteCache, into: DecalTarget): void;

	
	
	settling(now: number): boolean;

	
	
	
	clear(layerID: string): void;

	
	
	
	
	
	
	
	
	
	
	
	
	
	
	wipe(): void;

	
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	show(on: boolean): void;

	
	
	
	
	
	
	
	
	
	
	resync(): void;
}

export function newDecals(): Decals {
	const byLayer = new Map<string, Decal[]>();

	
	
	
	
	let showing = true;

	
	
	
	
	const seen = new Map<string, Seen>();

	
	
	
	const tint: [number, number, number] = [0, 0, 0];

	function spawn(pawn: Pawn, damage: number, cellSize: number, now: number): void {
		const list = byLayer.get(pawn.layerId) ?? [];
		byLayer.set(pawn.layerId, list);

		const [halfW, halfH] = pawnExtents(pawn, cellSize);
		const radius = Math.max(halfW, halfH);

		
		
		
		
		const dead = pawn.hp !== null && pawn.hp <= 0;
		const strength = dead ? 1 : severityOf(damage, pawn.maxHp);
		const count = dead ? DEATH_SPLATTERS : splatters(strength);

		
		
		
		
		if (dead) {
			pool(list, pawn, radius, now);
		}

		const weight = dead ? 1 : HIT_FAINT + (1 - HIT_FAINT) * strength;
		const reach = dead ? DEATH_SPREAD : HIT_SPREAD_MIN + (HIT_SPREAD_MAX - HIT_SPREAD_MIN) * strength;

		for (let i = 0; i < count; i++) {
			const next = generator(pawn, i);
			const half = radius * reach * (VARY_MIN + next() * (VARY_MAX - VARY_MIN));

			
			
			
			
			
			
			
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
				
				
				if (pawn.kind === "object") {
					continue;
				}

				const was = seen.get(pawn.id);
				seen.set(pawn.id, { hp: pawn.hp, maxHp: pawn.maxHp });

				const damage = hit(was, pawn);
				if (damage === 0) {
					continue;
				}

				
				
				
				
				
				if (!showing) {
					continue;
				}

				
				
				
				
				
				if (pawn.layerId !== viewedID || !pawn.visible) {
					continue;
				}

				spawn(pawn, damage, cellSize, now);
			}

			
			
			
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

		show(on) {
			showing = on;

			if (!on) {
				byLayer.clear();
			}
		},

		resync() {
			seen.clear();
		},
	};
}




interface Seen {
	hp: number | null;
	maxHp: number | null;
}




function hit(was: Seen | undefined, pawn: Pawn): number {
	
	
	if (was === undefined || was.hp === null || pawn.hp === null) {
		return 0;
	}

	
	
	if (was.hp <= 0) {
		return 0;
	}

	
	
	
	if (was.maxHp !== pawn.maxHp) {
		return 0;
	}

	const damage = was.hp - pawn.hp;

	return damage > 0 ? damage : 0;
}



const POOL_MARK = -1;









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
