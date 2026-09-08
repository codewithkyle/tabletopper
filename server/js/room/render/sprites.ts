// The pictures on the pawns: one texture array, LRU, exactly as the tiles are.
//
// IT IS A SECOND ARRAY AND NOT MORE LAYERS OF THE FIRST, because a texture
// array has one fixed layer size and these are 256 square where a tile is 512.
// Everything else about it is the tile cache's design, reused rather than
// re-argued: one bind and one instanced draw for every pawn on the table,
// least-recently-drawn eviction, and a loader that fetches by priority and
// gives up on what has scrolled away.
//
// THE STORED SPRITE KEEPS ITS ASPECT RATIO, which is where this departs from
// what the phase plan sketched. "Resized to 256 by 256" is right for a monster
// or a portrait, which are already square, and wrong for the case the object
// kind exists for: a wagon is stored 512 by 171, and squashing it into a square
// and then stretching it across a two-by-four footprint is a wagon that is
// visibly the wrong shape. So the longest edge becomes 256, the layer is partly
// unwritten exactly as an edge tile's is, and the real pixels ride in the slot
// -- which is machinery the tile cache already has.
//
// A PAWN WITH NO PICTURE IS NOT A HOLE. Its initials are drawn once into a
// layer from a 2D canvas, in its kind's colour, and after that it is an
// ordinary sprite. The same door renders the skull a dead creature carries.

import type { Pawn } from "../protocol.ts";
import type { Slot } from "./tiles.ts";
import { Slots, newLoader } from "./tiles.ts";

// SPRITE_SIZE is the layer's edge. A pawn is drawn at a couple of cells across
// -- 128 device pixels at a typical zoom on a HiDPI screen -- so 256 is the
// point past which a bigger texture is memory nobody sees.
export const SPRITE_SIZE = 256;

// SPRITE_LAYERS is the cache's size in pictures. At 256 square RGBA8 a layer is
// a quarter of a megabyte, so this is thirty-two megabytes -- generous for a
// table, which has a few dozen distinct pictures on it even during the stress
// run, where five hundred pawns share the images already loaded.
export const SPRITE_LAYERS = 128;

// KIND_COLORS is the border round a creature and the ground under its initials.
//
// THEY ARE CANVAS COLOURS AND NOT TOKENS, which is the one place in this app
// that is true and is not an oversight. A theme's palette is CSS, and nothing
// here is CSS: these go into a shader as three floats. They are chosen to read
// against both the light table and the dark one rather than to match either.
export const KIND_COLORS: Record<Pawn["kind"], readonly [number, number, number]> = {
	player: [0.29, 0.55, 0.9],
	monster: [0.82, 0.28, 0.28],
	npc: [0.33, 0.67, 0.44],
	object: [0.55, 0.51, 0.46],
};

// CONDITION_COLORS is the eight the protocol validates, as the rings are drawn.
// The names are the server's; these are what they look like.
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

// SKULL is the key the dead-creature glyph is generated under.
export const SKULL = "skull";

export interface SpriteCache {
	// begin and end bracket one frame, exactly as the tile pass's do.
	//
	// rebuilding says the caller is about to ask for its whole working set
	// again, which is the only time this cache forgets what that set was. See
	// the note beside `live` for why it has to remember at all.
	begin(rebuilding: boolean): void;
	end(): boolean;

	// sprite answers the layer a picture lives in, asking for it if it is not
	// here yet, and null while nothing has arrived. The caller falls back to
	// initials, which are always available.
	sprite(url: string, priority: number): Slot | null;

	// initials is the generated disc: a pawn's first letters on its kind's
	// colour, rendered once and then an ordinary sprite.
	initials(kind: Pawn["kind"], name: string): Slot | null;

	// glyph is the other generated entry, and there is one of it.
	glyph(name: string): Slot | null;

	texture(): WebGLTexture;

	// epoch changes whenever anything lands in the array. The pawn pass rebuilds
	// its instances on it, because a picture arriving changes the aspect ratio a
	// quad is fitted to.
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

	// generated is what has been drawn rather than fetched, so that a layer
	// evicted under pressure is rebuilt rather than lost.
	const generated = new Map<string, () => HTMLCanvasElement | null>();

	// live is what the last rebuild asked for, and it exists because the pawn
	// pass rebuilds ON A CHANGE while frames happen continuously.
	//
	// THE LRU IS MEASURED IN FRAMES AND THE WORKING SET IS NOT REFRESHED EVERY
	// FRAME, which is the gap this closes. A picture arriving several frames
	// after the rebuild that asked for it claims a layer, and with a full cache
	// claiming means evicting whatever was least recently USED -- which, with
	// nothing touching them in between, is every picture on the table. So the
	// set is re-touched on each frame, and forgotten only when the caller says
	// it is about to name a new one.
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

	// draw is the shared path for everything generated: build it, upload it,
	// remember how to build it again.
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

				// A null answer is every layer being spoken for by this frame.
				// The picture is dropped rather than evicting one that is on
				// screen right now, and the next rebuild asks for it again --
				// which is the tile cache's policy, for the tile cache's reason.
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

// decodeSprite is the loader's decode for pictures, and the resize is the whole
// of it.
//
// TWO STEPS, BECAUSE createImageBitmap CANNOT FIT. Its resizeWidth and
// resizeHeight are absolute: giving both distorts, and giving one requires
// knowing which edge is longer, which is what the first decode answers. The
// second call is a resize of an already-decoded bitmap, which is cheap and
// still off the main thread.
//
// A picture already inside the box is left alone, which is every monster image
// and every avatar -- both are stored at 256 already.
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

// initialsOf is up to two letters: the first of the first two words, or the
// first two letters of a single word. "Goblin Chief" is GC and "Goblin" is GO.
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

// drawInitials renders the fallback: a square of the kind's colour with the
// letters on it. It is a SQUARE and not a disc, because the pawn shader masks
// every creature to a disc anyway -- drawing one here would be the same circle
// twice, and the second one would have the jagged edge of a 2D canvas rather
// than the shader's antialiased one.
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

// drawSkull is the mark a creature at zero hit points carries.
//
// IT IS THE PLATFORM'S EMOJI rather than a path, and that is a deliberate
// trade: it renders differently on a Mac and on Linux, and it is one line
// instead of thirty of bezier curves for a shape whose whole job is to be
// recognised at twenty pixels across. It is drawn on transparency and composited
// over the greyed sprite.
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

// surface is a fresh 2D context at the layer's size. It is not pooled: these
// run once per distinct pair of kind and initials, a handful of times a
// session, and a pooled canvas would have to be cleared anyway.
function surface(): CanvasRenderingContext2D | null {
	const canvas = document.createElement("canvas");
	canvas.width = SPRITE_SIZE;
	canvas.height = SPRITE_SIZE;

	return canvas.getContext("2d");
}
