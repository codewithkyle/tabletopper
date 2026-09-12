import type { Pawn } from "../protocol.ts";
import type { Slot } from "../gl/texture-array.ts";
import type { TextureStats } from "./stats.ts";
import { KIND_COLORS } from "../model/color.ts";
import { createTextureArray } from "../gl/texture-array.ts";
import { newLoader } from "../gl/loader.ts";
export const SPRITE_SIZE = 256;
const SPRITE_LAYERS = 128;
export const SKULL = "skull";
export interface SpriteCache {
	begin(rebuilding: boolean): void;
	end(): boolean;
	sprite(url: string, priority: number): Slot | null;
	initials(kind: Pawn["kind"], name: string): Slot | null;
	glyph(name: string): Slot | null;
	texture(): WebGLTexture;
	epoch(): number;
	stats(): TextureStats;
	dispose(): void;
}
export function createSpriteCache(gl: WebGL2RenderingContext, invalidate: () => void): SpriteCache {
	const store = createTextureArray(gl, SPRITE_SIZE, SPRITE_LAYERS);
	const loader = newLoader(invalidate, decodeSprite);
	const live = new Set<string>();
	let epoch = 0;
	function upload(key: string, source: TexImageSource, w: number, h: number): Slot | null {
		const slot = store.upload(key, source, w, h);
		if (slot) {
			epoch++;
		}
		return slot;
	}
	function draw(key: string, build: () => HTMLCanvasElement | null): Slot | null {
		live.add(key);
		const resident = store.get(key);
		if (resident) {
			return resident;
		}
		const canvas = build();
		if (!canvas) {
			return null;
		}
		return upload(key, canvas, canvas.width, canvas.height);
	}
	return {
		begin(rebuilding) {
			store.tick();
			if (rebuilding) {
				live.clear();
			} else {
				for (const key of live) {
					store.get(key);
				}
			}
			loader.begin();
		},
		sprite(url, priority) {
			if (url === "") {
				return null;
			}
			live.add(url);
			const resident = store.get(url);
			if (resident) {
				return resident;
			}
			loader.want(url, url, priority);
			return null;
		},
		initials(kind, name) {
			const letters = initialsOf(name);
			return draw(`initials:${kind}:${letters}`, () => drawInitials(kind, letters));
		},
		glyph(name) {
			return draw(`glyph:${name}`, () => (name === SKULL ? drawSkull() : null));
		},
		end() {
			loader.end();
			let uploaded = 0;
			const waiting = loader.drain((key, bitmap) => {
				uploaded++;
				upload(key, bitmap, bitmap.width, bitmap.height);
			});
			return uploaded > 0 || waiting > 0;
		},
		texture: () => store.texture,
		epoch: () => epoch,
		stats: () => ({
			resident: store.resident(),
			capacity: store.capacity,
			evictions: store.evictions(),
			loader: loader.stats(),
		}),
		dispose() {
			loader.stop();
			live.clear();
			store.dispose();
		},
	};
}
async function decodeSprite(blob: Blob): Promise<ImageBitmap | null> {
	const options: ImageBitmapOptions = { premultiplyAlpha: "none", colorSpaceConversion: "none" };
	const full = await createImageBitmap(blob, options);
	const longest = Math.max(full.width, full.height);
	if (longest <= SPRITE_SIZE || longest === 0) {
		return full;
	}
	const scale = SPRITE_SIZE / longest;
	const resized = await createImageBitmap(full, {
		...options,
		resizeWidth: Math.max(1, Math.round(full.width * scale)),
		resizeHeight: Math.max(1, Math.round(full.height * scale)),
		resizeQuality: "high",
	});
	full.close();
	return resized;
}
function initialsOf(name: string): string {
	const words = name.trim().split(/\s+/).filter((word) => word !== "");
	if (words.length === 0) {
		return "?";
	}
	if (words.length === 1) {
		return words[0].slice(0, 2).toUpperCase();
	}
	return (words[0][0] + words[1][0]).toUpperCase();
}
function drawInitials(kind: Pawn["kind"], letters: string): HTMLCanvasElement | null {
	const ctx = surface();
	if (!ctx) {
		return null;
	}
	const [r, g, b] = KIND_COLORS[kind] ?? KIND_COLORS.npc;
	ctx.fillStyle = `rgb(${r * 255} ${g * 255} ${b * 255})`;
	ctx.fillRect(0, 0, SPRITE_SIZE, SPRITE_SIZE);
	ctx.fillStyle = "rgb(255 255 255)";
	ctx.font = `600 ${letters.length > 1 ? 104 : 140}px system-ui, sans-serif`;
	ctx.textAlign = "center";
	ctx.textBaseline = "middle";
	ctx.fillText(letters, SPRITE_SIZE / 2, SPRITE_SIZE / 2 + 4);
	return ctx.canvas;
}
function drawSkull(): HTMLCanvasElement | null {
	const ctx = surface();
	if (!ctx) {
		return null;
	}
	ctx.font = `${Math.round(SPRITE_SIZE * 0.8)}px system-ui, sans-serif`;
	ctx.textAlign = "center";
	ctx.textBaseline = "middle";
	ctx.fillText("\u{1F480}", SPRITE_SIZE / 2, SPRITE_SIZE / 2);
	return ctx.canvas;
}
function surface(): CanvasRenderingContext2D | null {
	const canvas = document.createElement("canvas");
	canvas.width = SPRITE_SIZE;
	canvas.height = SPRITE_SIZE;
	return canvas.getContext("2d");
}
