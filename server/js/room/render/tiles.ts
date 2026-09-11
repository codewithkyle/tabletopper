import type { MapRef } from "../protocol.ts";
import type { Rect } from "../model/types.ts";
import { levelPixels, levelTiles } from "./pyramid.ts";
export const LAYERS = 96;
const IN_FLIGHT = 8;
const RETRY_FLOOR = 1_000;
const RETRY_CEILING = 60_000;
const UPLOADS_PER_FRAME = 4;
export function levelFor(zoom: number, dpr: number, maxZoom: number): number {
	const scale = zoom * dpr;
	if (!(scale > 0)) {
		return maxZoom;
	}
	return Math.min(Math.max(Math.round(Math.log2(1 / scale)), 0), maxZoom);
}
export function levelScale(size: number, z: number): number {
	const pixels = levelPixels(size, z);
	return pixels > 0 ? size / pixels : 1;
}
export interface TileRange {
	x0: number;
	y0: number;
	x1: number;
	y1: number;
}
export function newRange(): TileRange {
	return { x0: 0, y0: 0, x1: -1, y1: -1 };
}
export function rangeCount(r: TileRange): number {
	return Math.max(0, r.x1 - r.x0 + 1) * Math.max(0, r.y1 - r.y0 + 1);
}
export function visibleRange(map: MapRef, z: number, view: Rect, out: TileRange): TileRange {
	out.x0 = 0;
	out.y0 = 0;
	out.x1 = -1;
	out.y1 = -1;
	const across = levelTiles(map.width, map.tileSize, z);
	const down = levelTiles(map.height, map.tileSize, z);
	if (across < 1 || down < 1) {
		return out;
	}
	const spanX = map.tileSize * levelScale(map.width, z);
	const spanY = map.tileSize * levelScale(map.height, z);
	const left = Math.max(view.x1, 0);
	const right = Math.min(view.x2, map.width);
	const top = Math.max(view.y1, 0);
	const bottom = Math.min(view.y2, map.height);
	if (right <= left || bottom <= top) {
		return out;
	}
	out.x0 = clamp(Math.floor(left / spanX), 0, across - 1);
	out.x1 = clamp(Math.ceil(right / spanX) - 1, 0, across - 1);
	out.y0 = clamp(Math.floor(top / spanY), 0, down - 1);
	out.y1 = clamp(Math.ceil(bottom / spanY) - 1, 0, down - 1);
	return out;
}
export function tileRect(map: MapRef, z: number, x: number, y: number, out: Rect): Rect {
	const scaleX = levelScale(map.width, z);
	const scaleY = levelScale(map.height, z);
	const pixelsX = levelPixels(map.width, z);
	const pixelsY = levelPixels(map.height, z);
	out.x1 = x * map.tileSize * scaleX;
	out.y1 = y * map.tileSize * scaleY;
	out.x2 = Math.min((x + 1) * map.tileSize, pixelsX) * scaleX;
	out.y2 = Math.min((y + 1) * map.tileSize, pixelsY) * scaleY;
	return out;
}
export function uvFor(map: MapRef, z: number, x: number, y: number, rect: Rect, out: Rect): Rect {
	const scaleX = levelScale(map.width, z);
	const scaleY = levelScale(map.height, z);
	out.x1 = (rect.x1 / scaleX - x * map.tileSize) / map.tileSize;
	out.y1 = (rect.y1 / scaleY - y * map.tileSize) / map.tileSize;
	out.x2 = (rect.x2 / scaleX - x * map.tileSize) / map.tileSize;
	out.y2 = (rect.y2 / scaleY - y * map.tileSize) / map.tileSize;
	return out;
}
export function tileKey(map: MapRef, z: number, x: number, y: number): string {
	return `${map.assetId}:${map.gen}:${z}:${x}:${y}`;
}
export function tileURL(map: MapRef, z: number, x: number, y: number): string {
	return `/assets/maps/${map.assetId}/tiles/${map.gen}/${z}/${x}_${y}.webp`;
}
export interface Slot {
	layer: number;
	w: number;
	h: number;
	used: number;
}
export class Slots {
	private readonly byKey = new Map<string, Slot>();
	private readonly free: number[] = [];
	private frame = 0;
	readonly capacity: number;
	constructor(capacity: number) {
		this.capacity = capacity;
		for (let i = capacity - 1; i >= 0; i--) {
			this.free.push(i);
		}
	}
	tick(): void {
		this.frame++;
	}
	get(key: string): Slot | undefined {
		const slot = this.byKey.get(key);
		if (slot) {
			slot.used = this.frame;
		}
		return slot;
	}
	has(key: string): boolean {
		return this.byKey.has(key);
	}
	claim(key: string, w: number, h: number): Slot | null {
		const existing = this.byKey.get(key);
		if (existing) {
			existing.w = w;
			existing.h = h;
			existing.used = this.frame;
			return existing;
		}
		let layer = this.free.pop();
		if (layer === undefined) {
			layer = this.evict();
		}
		if (layer === undefined) {
			return null;
		}
		const slot: Slot = { layer, w, h, used: this.frame };
		this.byKey.set(key, slot);
		return slot;
	}
	private evict(): number | undefined {
		let victim: string | undefined;
		let oldest = Infinity;
		for (const [key, slot] of this.byKey) {
			if (slot.used === this.frame) {
				continue;
			}
			if (slot.used < oldest) {
				oldest = slot.used;
				victim = key;
			}
		}
		if (victim === undefined) {
			return undefined;
		}
		const slot = this.byKey.get(victim);
		this.byKey.delete(victim);
		return slot?.layer;
	}
	get size(): number {
		return this.byKey.size;
	}
}
export interface Loader {
	begin(): void;
	want(key: string, url: string, priority: number): void;
	end(): void;
	drain(upload: (key: string, bitmap: ImageBitmap) => void): number;
	fetched(): number;
	stop(): void;
}
interface Wanted {
	key: string;
	url: string;
	priority: number;
}
export type Decode = (blob: Blob) => Promise<ImageBitmap | null>;
export function decodeAsIs(blob: Blob): Promise<ImageBitmap | null> {
	return createImageBitmap(blob, { premultiplyAlpha: "none", colorSpaceConversion: "none" });
}
export function newLoader(invalidate: () => void, decode: Decode = decodeAsIs): Loader {
	const queue: Wanted[] = [];
	const inFlight = new Map<string, AbortController>();
	const ready: { key: string; bitmap: ImageBitmap }[] = [];
	const missing = new Set<string>();
	const wantedThisFrame = new Set<string>();
	const failures = new Map<string, number>();
	const retryAt = new Map<string, number>();
	function failed(key: string): void {
		const n = (failures.get(key) ?? 0) + 1;
		failures.set(key, n);
		retryAt.set(key, performance.now() + Math.min(RETRY_CEILING, RETRY_FLOOR * 2 ** (n - 1)));
	}
	function recovered(key: string): void {
		failures.delete(key);
		retryAt.delete(key);
	}
	let total = 0;
	let stopped = false;
	function start(item: Wanted): void {
		const controller = new AbortController();
		inFlight.set(item.key, controller);
		total++;
		let decoding = false;
		fetch(item.url, { credentials: "same-origin", signal: controller.signal })
			.then((response) => {
				if (response.status === 404) {
					missing.add(item.key);
					return null;
				}
				if (!response.ok) {
					failed(item.key);
					return null;
				}
				return response.blob();
			})
			.then((blob) => {
				if (!blob) {
					return null;
				}
				decoding = true;
				return decode(blob);
			})
			.then((bitmap) => {
				if (bitmap) {
					recovered(item.key);
					ready.push({ key: item.key, bitmap });
					invalidate();
				}
			})
			.catch((err: unknown) => {
				if (err instanceof DOMException && err.name === "AbortError") {
					return;
				}
				if (decoding) {
					missing.add(item.key);
				} else {
					failed(item.key);
				}
			})
			.finally(() => {
				inFlight.delete(item.key);
			});
	}
	return {
		begin() {
			queue.length = 0;
			wantedThisFrame.clear();
		},
		want(key, url, priority) {
			wantedThisFrame.add(key);
			if (missing.has(key) || inFlight.has(key)) {
				return;
			}
			const until = retryAt.get(key);
			if (until !== undefined && until > performance.now()) {
				return;
			}
			queue.push({ key, url, priority });
		},
		end() {
			if (stopped || queue.length === 0) {
				return;
			}
			queue.sort((a, b) => a.priority - b.priority);
			let spare = IN_FLIGHT - inFlight.size;
			for (const [key, controller] of inFlight) {
				if (spare >= queue.length) {
					break;
				}
				if (wantedThisFrame.has(key)) {
					continue;
				}
				controller.abort();
				inFlight.delete(key);
				spare++;
			}
			for (const item of queue) {
				if (inFlight.size >= IN_FLIGHT) {
					break;
				}
				if (!inFlight.has(item.key)) {
					start(item);
				}
			}
		},
		drain(upload) {
			const count = Math.min(ready.length, UPLOADS_PER_FRAME);
			for (let i = 0; i < count; i++) {
				const item = ready[i];
				upload(item.key, item.bitmap);
				item.bitmap.close();
			}
			ready.splice(0, count);
			return ready.length;
		},
		fetched: () => total,
		stop() {
			stopped = true;
			for (const controller of inFlight.values()) {
				controller.abort();
			}
			inFlight.clear();
			for (const item of ready) {
				item.bitmap.close();
			}
			ready.length = 0;
		},
	};
}
function clamp(v: number, low: number, high: number): number {
	return Math.min(Math.max(v, low), high);
}
