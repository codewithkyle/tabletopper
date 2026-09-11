





















import type { Pawn } from "../protocol.ts";
import type { Slot } from "./tiles.ts";
import { Slots, newLoader } from "./tiles.ts";




export const SPRITE_SIZE = 256;





export const SPRITE_LAYERS = 128;







export const KIND_COLORS: Record<Pawn["kind"], readonly [number, number, number]> = {
	player: [0.29, 0.55, 0.9],
	monster: [0.82, 0.28, 0.28],
	npc: [0.33, 0.67, 0.44],
	object: [0.55, 0.51, 0.46],
};



export const CONDITION_COLORS: Record<string, readonly [number, number, number]> = {
	red: [0.94, 0.27, 0.27],
	orange: [0.98, 0.57, 0.24],
	yellow: [0.98, 0.83, 0.25],
	green: [0.3, 0.76, 0.42],
	blue: [0.3, 0.6, 0.96],
	purple: [0.65, 0.4, 0.94],
	pink: [0.96, 0.5, 0.75],
	white: [0.95, 0.95, 0.95],
};


export const SKULL = "skull";

export interface SpriteCache {
	
	
	
	
	
	begin(rebuilding: boolean): void;
	end(): boolean;

	
	
	
	sprite(url: string, priority: number): Slot | null;

	
	
	initials(kind: Pawn["kind"], name: string): Slot | null;

	
	glyph(name: string): Slot | null;

	texture(): WebGLTexture;

	
	
	
	epoch(): number;

	dispose(): void;
}

export function createSpriteCache(gl: WebGL2RenderingContext, invalidate: () => void): SpriteCache {
	const maxLayers = Math.min(SPRITE_LAYERS, gl.getParameter(gl.MAX_ARRAY_TEXTURE_LAYERS) as number);

	const texture = gl.createTexture();
	gl.bindTexture(gl.TEXTURE_2D_ARRAY, texture);
	gl.texStorage3D(gl.TEXTURE_2D_ARRAY, 1, gl.RGBA8, SPRITE_SIZE, SPRITE_SIZE, maxLayers);
	gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
	gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
	gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
	gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
	gl.bindTexture(gl.TEXTURE_2D_ARRAY, null);

	const slots = new Slots(maxLayers);
	const loader = newLoader(invalidate, decodeSprite);

	
	
	const generated = new Map<string, () => HTMLCanvasElement | null>();

	
	
	
	
	
	
	
	
	
	
	const live = new Set<string>();

	let epoch = 0;

	function upload(key: string, source: TexImageSource, w: number, h: number): Slot | null {
		const slot = slots.claim(key, w, h);
		if (!slot) {
			return null;
		}

		gl.bindTexture(gl.TEXTURE_2D_ARRAY, texture);
		gl.texSubImage3D(gl.TEXTURE_2D_ARRAY, 0, 0, 0, slot.layer, w, h, 1, gl.RGBA, gl.UNSIGNED_BYTE, source);
		gl.bindTexture(gl.TEXTURE_2D_ARRAY, null);

		epoch++;

		return slot;
	}

	
	
	function draw(key: string, build: () => HTMLCanvasElement | null): Slot | null {
		live.add(key);

		const resident = slots.get(key);
		if (resident) {
			return resident;
		}

		generated.set(key, build);

		const canvas = build();
		if (!canvas) {
			return null;
		}

		return upload(key, canvas, canvas.width, canvas.height);
	}

	return {
		begin(rebuilding) {
			slots.tick();

			if (rebuilding) {
				live.clear();
			} else {
				for (const key of live) {
					slots.get(key);
				}
			}

			loader.begin();
		},

		sprite(url, priority) {
			if (url === "") {
				return null;
			}

			live.add(url);

			const resident = slots.get(url);
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

		texture: () => texture,
		epoch: () => epoch,

		dispose() {
			loader.stop();
			generated.clear();
			live.clear();
			gl.deleteTexture(texture);
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



export function initialsOf(name: string): string {
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
