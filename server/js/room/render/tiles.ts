// The tile cache and the tile loader: which pieces of which map are on the GPU,
// which are being fetched, and what to draw in the meantime.
//
// THE CACHE IS ONE TEXTURE ARRAY AND NOT N TEXTURES. Ninety-six separate
// textures would be ninety-six bind calls and ninety-six draw calls, because a
// draw call can only use the textures bound to it; one array texture with
// ninety-six layers is one bind and one instanced draw for the whole viewport,
// with the layer index riding in as a per-instance attribute. That is the whole
// reason the renderer can put a 12000 by 9000 map on screen in one call.
//
// A MISSING TILE IS DRAWN FROM ITS ANCESTOR RATHER THAN LEFT BLANK. The parent
// of (z, x, y) is (z+1, x>>1, y>>1) and covers twice as much ground at half the
// detail, so while a level loads the map is already there, coarse, and sharpens
// in place. A blank tile that filled in would flash; this does not.
//
// LEVEL z IS NOT THE NATIVE IMAGE SHRUNK BY 2^z, and getting that wrong is the
// subtle bug in this file. The tiler resizes each level to
// ceil(size / 2^z) pixels -- a CEIL -- and that level then represents the WHOLE
// map. So 9000 rows at level 5 is 282 pixels, and 282 times 32 is 9024: mapping
// a tile back to native pixels with a shift would draw the map 24 pixels taller
// than it is and slide the grid off the features it lines up with. The scale is
// size / levelPixels(size, z) per axis per level, which is what levelScale is.

import type { MapRef } from "../protocol.ts";
import type { Rect } from "./camera.ts";
import { levelPixels, levelTiles } from "./pyramid.ts";

// LAYERS is the cache's size in tiles. At 512 square RGBA8 a layer is one
// megabyte, so this is ninety-six megabytes of video memory -- generous for a
// 1440p viewport, which covers about fifteen tiles at native resolution plus
// the ancestors behind them.
export const LAYERS = 96;

// IN_FLIGHT is how many tiles are fetched at once. Browsers allow six
// connections per host over HTTP/1.1 and many more over HTTP/2; eight keeps the
// pipe full either way without burying a tile that has just come into view
// behind a queue of tiles that have left it.
const IN_FLIGHT = 8;

// UPLOADS_PER_FRAME bounds the one part of the arrival path that is synchronous
// and on the main thread. Decoding happens off-thread in createImageBitmap;
// texSubImage3D does not, so a burst of twenty arrivals uploaded in one frame
// is a dropped frame. Four is imperceptible and the rest wait one frame.
const UPLOADS_PER_FRAME = 4;

// levelFor picks which level of the pyramid to draw at this zoom.
//
// THE DEVICE PIXEL RATIO IS PART OF THE SCALE, not a correction applied after.
// A retina display at zoom 1 is showing native map pixels at half their size,
// so it wants level 0; the same zoom on a 1x display wants level 0 too, and a
// 1x display at zoom 0.5 wants level 1. One expression covers all three.
export function levelFor(zoom: number, dpr: number, maxZoom: number): number {
	const scale = zoom * dpr;
	if (!(scale > 0)) {
		return maxZoom;
	}

	return Math.min(Math.max(Math.round(Math.log2(1 / scale)), 0), maxZoom);
}

// levelScale is how many native map pixels one pixel of level z stands for. It
// is size / levelPixels rather than 2^z for the reason at the top of this file.
export function levelScale(size: number, z: number): number {
	const pixels = levelPixels(size, z);

	return pixels > 0 ? size / pixels : 1;
}

// TileRange is an inclusive box of tile indexes. x1 below x0 is an empty range,
// which is what a viewport entirely off the map produces.
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

// visibleRange is which tiles of level z the viewport covers, intersected with
// the map. The rectangle is in native map pixels, which is the only space
// anything outside this file works in.
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

// tileRect is one tile's footprint in native map pixels. The last tile of a row
// ends exactly at the map's edge rather than overshooting, which is what keeps
// a level from being drawn slightly too large.
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

// uvFor maps a native rectangle into the texture coordinates of one tile at
// level z. It is the same expression whether the tile is the one that was asked
// for or an ancestor standing in for it: both levels cover the whole map, so a
// native coordinate resolves in either.
//
// The result is not clamped to [0, 1] on purpose. A caller that asks about a
// rectangle outside the tile has a bug, and a silently clamped answer would
// draw the tile's edge pixel smeared across it.
export function uvFor(map: MapRef, z: number, x: number, y: number, rect: Rect, out: Rect): Rect {
	const scaleX = levelScale(map.width, z);
	const scaleY = levelScale(map.height, z);

	out.x1 = (rect.x1 / scaleX - x * map.tileSize) / map.tileSize;
	out.y1 = (rect.y1 / scaleY - y * map.tileSize) / map.tileSize;
	out.x2 = (rect.x2 / scaleX - x * map.tileSize) / map.tileSize;
	out.y2 = (rect.y2 / scaleY - y * map.tileSize) / map.tileSize;

	return out;
}

// tileKey identifies one tile of one generation of one map. The generation is
// in it because a re-tiled map is a different picture at the same coordinates,
// and a cache that answered from the old one would serve a map the GM replaced.
export function tileKey(map: MapRef, z: number, x: number, y: number): string {
	return `${map.assetId}:${map.gen}:${z}:${x}:${y}`;
}

// mapPrefix is every tile of one map, for abandoning a fetch queue.
export function mapPrefix(map: MapRef): string {
	return `${map.assetId}:${map.gen}:`;
}

// tileURL is the route in internal/controllers/map-tiles.go. The extension is
// in the path rather than left to the Content-Type because the browser's cache
// keys on the path, and these are served immutable for a year.
export function tileURL(map: MapRef, z: number, x: number, y: number): string {
	return `/assets/maps/${map.assetId}/tiles/${map.gen}/${z}/${x}_${y}.webp`;
}

// Slot is one tile's place in the texture array. w and h are the tile's real
// pixels, which are short of tileSize along the map's right and bottom edges --
// an edge tile is not padded, so its texture layer is partly unwritten and its
// UV maximum is short of 1.
export interface Slot {
	layer: number;
	w: number;
	h: number;
	used: number;
}

// Slots is the cache's bookkeeping with no WebGL in it: which key lives in
// which layer of the array, and which layer to overwrite when a new tile
// arrives and every layer is taken.
//
// IT NEVER EVICTS A LAYER USED THIS FRAME. Without that rule a viewport needing
// more tiles than the cache holds would evict the tile it uploaded a moment ago
// to make room for the next one, and go round for ever at one frame each --
// slow, and it looks like the map is dissolving. Refusing instead means the
// missing tiles are drawn from their ancestors, which is what they were going
// to be drawn from anyway.
export class Slots {
	private readonly byKey = new Map<string, Slot>();
	private readonly free: number[] = [];
	private frame = 0;

	// A parameter property would be shorter and is not erasable syntax: the
	// tsconfig sets erasableSyntaxOnly so that node --test can strip these
	// files and run them directly, with no build step between the source and
	// the test.
	readonly capacity: number;

	constructor(capacity: number) {
		this.capacity = capacity;
		for (let i = capacity - 1; i >= 0; i--) {
			this.free.push(i);
		}
	}

	// tick advances the frame counter, which is the clock the LRU is measured
	// against. It is frames rather than milliseconds because what matters is
	// "was this drawn recently", and a room nobody is touching renders no
	// frames at all.
	tick(): void {
		this.frame++;
	}

	// get marks the tile as used this frame, which both keeps it and protects
	// it from eviction until the next one.
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

	// claim finds a layer for a tile that has just arrived, and answers null
	// when every layer is spoken for by this frame.
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

// Loader fetches tiles by priority and gives up on the ones that have scrolled
// away. Every method is called from the frame loop and none of them does any
// work in an event handler.
export interface Loader {
	// begin, want and end bracket one frame's demand. Anything not wanted
	// between them is no longer wanted, and an in-flight request for it is
	// aborted -- which is what makes a change of level cheap rather than a
	// queue of tiles for a zoom nobody is at any more.
	begin(): void;
	want(key: string, url: string, priority: number): void;
	end(): void;

	// drain uploads what has arrived, at most a few per frame, and answers how
	// many are still waiting. A non-zero answer keeps the frame loop alive.
	drain(upload: (key: string, bitmap: ImageBitmap) => void): number;

	// abandon drops everything queued or in flight for one map, which is what a
	// re-tiled or replaced map calls for: those URLs are gone.
	abandon(prefix: string): void;

	// fetched is the benchmark's counter: how many tiles this session has
	// actually pulled over the network.
	fetched(): number;

	stop(): void;
}

interface Wanted {
	key: string;
	url: string;
	priority: number;
}

export function newLoader(invalidate: () => void): Loader {
	const queue: Wanted[] = [];
	const inFlight = new Map<string, AbortController>();
	const ready: { key: string; bitmap: ImageBitmap }[] = [];

	// missing is a 404 and is never retried. The tile route answers 404 for a
	// coordinate outside the pyramid, and asking again every frame for the rest
	// of the session would be a request per frame for ever.
	const missing = new Set<string>();
	const wantedThisFrame = new Set<string>();

	let total = 0;
	let stopped = false;

	function start(item: Wanted): void {
		const controller = new AbortController();
		inFlight.set(item.key, controller);
		total++;

		fetch(item.url, { credentials: "same-origin", signal: controller.signal })
			.then((response) => {
				if (response.status === 404) {
					missing.add(item.key);

					return null;
				}
				if (!response.ok) {
					return null;
				}

				return response.blob();
			})
			.then((blob) => {
				if (!blob) {
					return null;
				}

				// premultiplyAlpha and colorSpaceConversion are both turned off
				// so the bytes that reach the GPU are the bytes the tiler
				// wrote. The default for either is "browser decides", and two
				// browsers deciding differently is a map that is a shade off on
				// one of them.
				return createImageBitmap(blob, {
					premultiplyAlpha: "none",
					colorSpaceConversion: "none",
				});
			})
			.then((bitmap) => {
				if (bitmap) {
					ready.push({ key: item.key, bitmap });
					invalidate();
				}
			})
			.catch(() => {
				// An abort, a dropped connection, or an image the decoder
				// refused. None of them is worth a message: the tile is drawn
				// from its ancestor and asked for again the next time it is
				// wanted.
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

			queue.push({ key, url, priority });
		},

		end() {
			if (stopped) {
				return;
			}

			// A tile that has left the viewport is not worth the connection it
			// is holding. This is what "abort on level change" is: every tile of
			// the level just left goes unwanted in the same frame.
			for (const [key, controller] of inFlight) {
				if (!wantedThisFrame.has(key)) {
					controller.abort();
				}
			}

			if (queue.length === 0) {
				return;
			}

			// NEAREST THE MIDDLE OF THE SCREEN FIRST. Somebody who has just
			// panned is looking at the centre of the viewport, and filling in
			// from the edges is the same tiles arriving in the least useful
			// order.
			queue.sort((a, b) => a.priority - b.priority);

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

		abandon(prefix) {
			for (const [key, controller] of inFlight) {
				if (key.startsWith(prefix)) {
					controller.abort();
				}
			}
			for (let i = ready.length - 1; i >= 0; i--) {
				if (ready[i].key.startsWith(prefix)) {
					ready[i].bitmap.close();
					ready.splice(i, 1);
				}
			}
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
