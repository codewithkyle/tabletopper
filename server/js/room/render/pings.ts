export interface RingTarget {
	ellipse(
		x: number, y: number, radius: number,
		color: readonly [number, number, number], alpha: number, thickness: number,
	): void;
}
export const RINGS = 3;
const RING_MS = 700;
const STAGGER = 180;
export const LIFE = RING_MS + (RINGS - 1) * STAGGER;
export const RADIUS_MAX = 2;
export const RADIUS_MIN = 0.25;
const WIDTH = 2;
const FADE = 0.25;
export const CAP = 16;
export function ringAt(age: number, index: number): { radius: number; alpha: number } | null {
	const t = (age - index * STAGGER) / RING_MS;
	if (t < 0 || t >= 1) {
		return null;
	}
	const eased = 1 - (1 - t) ** 3;
	return {
		radius: RADIUS_MAX + (RADIUS_MIN - RADIUS_MAX) * eased,
		alpha: t < 1 - FADE ? 1 : (1 - t) / FADE,
	};
}
interface Ping {
	x: number;
	y: number;
	born: number;
	color: readonly [number, number, number];
}
export interface Pings {
	add(layerID: string, x: number, y: number, color: readonly [number, number, number], now: number): void;
	build(layerID: string, now: number, cell: number, into: RingTarget): void;
	settling(now: number): boolean;
}
export function newPings(): Pings {
	const byLayer = new Map<string, Ping[]>();
	return {
		add(layerID, x, y, color, now) {
			const list = byLayer.get(layerID) ?? [];
			byLayer.set(layerID, list);
			list.push({ x, y, born: now, color });
			if (list.length > CAP) {
				list.splice(0, list.length - CAP);
			}
		},
		build(layerID, now, cell, into) {
			const list = byLayer.get(layerID);
			if (!list) {
				return;
			}
			for (const ping of list) {
				const age = now - ping.born;
				for (let i = 0; i < RINGS; i++) {
					const ring = ringAt(age, i);
					if (!ring) {
						continue;
					}
					into.ellipse(ping.x, ping.y, cell * ring.radius, ping.color, ring.alpha, WIDTH);
				}
			}
		},
		settling(now) {
			let live = false;
			for (const [layerID, list] of byLayer) {
				let kept = 0;
				for (const ping of list) {
					if (now - ping.born >= LIFE) {
						continue;
					}
					list[kept++] = ping;
				}
				list.length = kept;
				if (kept === 0) {
					byLayer.delete(layerID);
					continue;
				}
				live = true;
			}
			return live;
		},
	};
}
