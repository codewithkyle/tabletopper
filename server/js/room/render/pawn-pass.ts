// Every pawn on the viewed layer, in one instanced draw.
//
// ONE CALL FOR THE WHOLE TABLE, for the tile pass's reason: the sprite cache is
// a texture array, so a draw can sample every picture on the table at once and
// the layer index rides in as a per-instance attribute. Five hundred pawns and
// five are the same number of draw calls.
//
// CREATURES ARE DISCS AND OBJECTS ARE RECTANGLES, and the difference is one
// float per instance rather than two passes. A creature is masked to a disc
// with a border in its kind's colour and its picture COVERS that disc -- cropped
// rather than letterboxed, because a portrait with bars down the side inside a
// circle looks like a mistake. An object is drawn unmasked with its picture
// CONTAINED in its quad, aspect kept, because the quad is the wagon's actual
// size on the floor and a wagon stretched to fill it is the wrong wagon.
//
// AN OBJECT'S QUAD IS THE PICTURE'S OWN PIXELS, so contain normally fits it
// exactly and the two factors come back at one. They stop being one when the GM
// has typed a different width or height into the pawn's dialog, which is the
// one case where an object is deliberately not the shape of its picture.
//
// THE INSTANCE BUFFER IS REBUILT WHEN THE TABLE CHANGES AND NOT PER FRAME. A
// pan or a zoom changes the matrix and nothing else, which is the common case
// by a wide margin; what forces a rebuild is a pawn moving, appearing, changing
// floor or size, or a picture arriving -- the last because a picture's aspect
// ratio is what the quad is fitted to.

import type { Camera } from "./camera.ts";
import type { Grid, Pawn } from "../protocol.ts";
import type { SpriteCache } from "./sprites.ts";
import { KIND_COLORS } from "./sprites.ts";
import { SKULL } from "./sprites.ts";
import { clipMatrix } from "./camera.ts";
import { createProgram, uniforms } from "./gl.ts";
import { pawnExtents } from "./path.ts";

// FLOATS_PER_INSTANCE: the rectangle, the border colour, the style and the fit.
// Four vec4s, so the attribute count stays at four and one instance is one
// contiguous run of sixteen floats.
const FLOATS_PER_INSTANCE = 16;

// BORDER_PIXELS is the ring round a creature, in DEVICE pixels rather than map
// pixels, so it is the same weight at every zoom. A border that scaled with the
// camera would be invisible zoomed out and a thick band zoomed in.
const BORDER_PIXELS = 2;

// HIDDEN_ALPHA is how a pawn players cannot see looks to the GM. Desaturated as
// well, so "hidden" is legible at a glance rather than a shade of the same
// thing -- a GM scanning a table has to be able to tell without hovering.
const HIDDEN_ALPHA = 0.6;
const HIDDEN_GREY = 0.7;

// SKULL_SCALE is the dead-creature mark, as a fraction of the pawn's diameter.
const SKULL_SCALE = 0.7;

// SPRITE_EDGE is the sprite layer's size. It is written here rather than
// imported so this module's arithmetic does not depend on how the cache was
// built; a test pins the two together.
const SPRITE_EDGE = 256;

const vertexSource = `#version 300 es
#define BORDER_PIXELS ${BORDER_PIXELS}.0

layout(location = 0) in vec2 a_corner;
layout(location = 1) in vec4 a_rect;
layout(location = 2) in vec4 a_border;
layout(location = 3) in vec4 a_style;
layout(location = 4) in vec4 a_fit;

uniform mat3 u_clip;
uniform float u_scale;

out vec2 v_local;
flat out vec4 v_border;
flat out vec4 v_style;
flat out vec4 v_fit;
flat out float v_edge;

void main() {
	v_local = a_corner * 2.0 - 1.0;
	v_border = a_border;
	v_style = a_style;
	v_fit = a_fit;

	// The border is a device-pixel width turned into local units, which is why
	// it is computed here and not in the fragment shader: the half extent is a
	// per-instance value and this is the last place it is one.
	v_edge = BORDER_PIXELS / max(a_rect.z * u_scale, 1e-4);

	vec2 world = a_rect.xy + v_local * a_rect.zw;
	gl_Position = vec4((u_clip * vec3(world, 1.0)).xy, 0.0, 1.0);
}
`;

const fragmentSource = `#version 300 es
precision highp float;
precision highp sampler2DArray;

in vec2 v_local;
flat in vec4 v_border;
flat in vec4 v_style;
flat in vec4 v_fit;
flat in float v_edge;

uniform sampler2DArray u_sprites;

out vec4 outColor;

void main() {
	float layer = v_style.x;
	float shape = v_style.y;
	float alpha = v_style.z;
	float grey  = v_style.w;

	// The picture, fitted. v_fit.xy is how much of the quad the image covers on
	// each axis -- under one for a letterboxed object, over one for a cropped
	// creature -- and v_fit.zw is how much of the 256 square layer the image
	// actually occupies, which is short of 1 for anything that is not square.
	vec2 t = (v_local / v_fit.xy) * 0.5 + 0.5;

	vec4 picture = vec4(0.0);
	if (layer >= 0.0 && t.x >= 0.0 && t.y >= 0.0 && t.x <= 1.0 && t.y <= 1.0) {
		picture = texture(u_sprites, vec3(t * v_fit.zw, layer));
	}

	vec3 rgb;
	float cover;

	if (shape < 0.5) {
		float r = length(v_local);
		float aa = max(fwidth(r), 1e-5);

		cover = 1.0 - smoothstep(1.0 - aa, 1.0, r);
		if (cover <= 0.0) {
			discard;
		}

		// WHATEVER THE PICTURE DOES NOT COVER IS THE KIND'S COLOUR, so a token
		// saved with a transparent background reads as a pawn rather than as a
		// hole in the table with a ring round it.
		rgb = mix(v_border.rgb, picture.rgb, picture.a);

		float border = smoothstep(1.0 - v_edge - aa, 1.0 - v_edge, r);
		rgb = mix(rgb, v_border.rgb, border * v_border.a);
	} else {
		cover = picture.a;
		if (cover <= 0.0) {
			discard;
		}
		rgb = picture.rgb;
	}

	// Desaturation is the GM's marker for a pawn players cannot see, and the
	// whole of what a dead creature is drawn as under its skull.
	rgb = mix(rgb, vec3(dot(rgb, vec3(0.299, 0.587, 0.114))), grey);

	outColor = vec4(rgb, cover * alpha);
}
`;

const names = ["u_clip", "u_sprites", "u_scale"] as const;

// Drawn is one pawn as this pass needs it. It is deliberately NOT room.Pawn:
// the stress test's five hundred synthetic pawns are not in the store and never
// go near it, and a pass that took the protocol's type would have to be handed
// five hundred forged ones.
export interface Drawn {
	id: string;
	kind: Pawn["kind"];
	name: string;
	image: string;
	x: number;
	y: number;
	z: number;
	size: Pawn["size"];

	// width and height are an OBJECT'S size, in map pixels, and are zero for a
	// creature -- whose size comes from its category instead. See pawnExtents.
	width: number;
	height: number;

	// hidden is the GM's copy of a pawn players cannot see. It is never true on
	// a player's, because they are never sent one.
	hidden: boolean;

	// dead draws the skull. It is false when the viewer was told no hit points
	// at all, which is the honest answer -- a player who cannot see a monster's
	// health cannot see that it has run out either.
	dead: boolean;
}

export interface PawnPass {
	// build rebuilds the instance buffer. It is called when the table changes
	// rather than every frame; see the note at the top of this file.
	//
	// alpha multiplies every instance, which is the whole of what makes a
	// second pass a GHOST pass: the same pawns, the same pictures, the same
	// arithmetic, drawn at half.
	build(pawns: readonly Drawn[], grid: Grid, sprites: SpriteCache, alpha?: number): void;

	draw(cam: Camera, deviceWidth: number, deviceHeight: number, dpr: number): void;

	dispose(): void;
}

export function createPawnPass(gl: WebGL2RenderingContext): PawnPass {
	const program = createProgram(gl, vertexSource, fragmentSource);
	const at = uniforms(gl, program, names);

	const vao = gl.createVertexArray();
	gl.bindVertexArray(vao);

	const corners = gl.createBuffer();
	gl.bindBuffer(gl.ARRAY_BUFFER, corners);
	gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([0, 0, 1, 0, 0, 1, 1, 1]), gl.STATIC_DRAW);
	gl.enableVertexAttribArray(0);
	gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);

	const instances = gl.createBuffer();
	gl.bindBuffer(gl.ARRAY_BUFFER, instances);
	const stride = FLOATS_PER_INSTANCE * 4;
	for (let i = 0; i < 4; i++) {
		gl.enableVertexAttribArray(1 + i);
		gl.vertexAttribPointer(1 + i, 4, gl.FLOAT, false, stride, i * 16);
		gl.vertexAttribDivisor(1 + i, 1);
	}

	gl.bindVertexArray(null);
	gl.bindBuffer(gl.ARRAY_BUFFER, null);

	// Grown by doubling and never shrunk, which is the second performance rule:
	// a typed array reallocated per rebuild is a garbage collection pause on
	// every drag, and pauses are the jank that gets worse the longer a session
	// runs.
	let data = new Float32Array(64 * FLOATS_PER_INSTANCE);
	let count = 0;
	let texture: WebGLTexture | null = null;

	// order is the draw order, reused so a rebuild allocates nothing.
	let order: number[] = [];

	const matrix = new Float32Array(9);

	function reserve(needed: number): void {
		const floats = needed * FLOATS_PER_INSTANCE;
		if (floats <= data.length) {
			return;
		}

		let size = data.length;
		while (size < floats) {
			size *= 2;
		}
		data = new Float32Array(size);
	}

	function push(
		x: number, y: number, halfW: number, halfH: number,
		border: readonly [number, number, number], borderAlpha: number,
		layer: number, shape: number, alpha: number, grey: number,
		kx: number, ky: number, uvW: number, uvH: number,
	): void {
		const at = count * FLOATS_PER_INSTANCE;

		data[at] = x;
		data[at + 1] = y;
		data[at + 2] = halfW;
		data[at + 3] = halfH;

		data[at + 4] = border[0];
		data[at + 5] = border[1];
		data[at + 6] = border[2];
		data[at + 7] = borderAlpha;

		data[at + 8] = layer;
		data[at + 9] = shape;
		data[at + 10] = alpha;
		data[at + 11] = grey;

		data[at + 12] = kx;
		data[at + 13] = ky;
		data[at + 14] = uvW;
		data[at + 15] = uvH;

		count++;
	}

	return {
		build(pawns, grid, sprites, alpha = 1) {
			count = 0;
			texture = sprites.texture();

			// Two instances each at worst: the pawn and, for a dead creature,
			// the skull over it.
			reserve(pawns.length * 2);

			// SORTED BY z AND THEN BY id, which is the server's own order for a
			// draw: a pawn spawned later sits on top, and two pawns that
			// somehow share a z are drawn in an order that is the same on every
			// client rather than whichever way the array happened to be built.
			if (order.length !== pawns.length) {
				order = pawns.map((_, i) => i);
			} else {
				for (let i = 0; i < pawns.length; i++) {
					order[i] = i;
				}
			}
			order.sort((a, b) => pawns[a].z - pawns[b].z || (pawns[a].id < pawns[b].id ? -1 : 1));

			for (const index of order) {
				const pawn = pawns[index];
				const [halfW, halfH] = pawnExtents(pawn, grid.cellSize);
				const object = pawn.kind === "object";

				const opacity = alpha * (pawn.hidden ? HIDDEN_ALPHA : 1);
				const grey = pawn.hidden ? HIDDEN_GREY : pawn.dead ? 1 : 0;

				// The picture, or the initials that stand in for it -- both
				// while one is loading and for a pawn that has none at all.
				//
				// ASKING FOR IT IS ALSO WHAT KEEPS IT. sprite() marks the layer
				// used, which is what the cache's eviction skips, so a rebuild
				// is a rebuild AND a refresh of the whole working set. That is
				// why a picture arriving forces one: without it the set would go
				// stale between rebuilds, and with it eviction can only ever
				// reach a picture nothing on this floor is drawn from.
				const slot = sprites.sprite(pawn.image, 0) ?? sprites.initials(pawn.kind, pawn.name);
				const layer = slot ? slot.layer : -1;
				const [kx, ky] = slot
					? fitFactors(slot.w, slot.h, halfW, halfH, !object)
					: [1, 1];

				push(
					pawn.x, pawn.y, halfW, halfH,
					KIND_COLORS[pawn.kind] ?? KIND_COLORS.npc,
					object ? 0 : 1,
					layer, object ? 1 : 0, opacity, grey,
					kx, ky,
					slot ? slot.w / SPRITE_EDGE : 1,
					slot ? slot.h / SPRITE_EDGE : 1,
				);

				if (pawn.dead && !object) {
					const skull = sprites.glyph(SKULL);
					if (skull) {
						const size = halfW * SKULL_SCALE;
						const [sx, sy] = fitFactors(skull.w, skull.h, size, size, false);

						push(
							pawn.x, pawn.y, size, size,
							KIND_COLORS[pawn.kind] ?? KIND_COLORS.npc, 0,
							skull.layer, 1, opacity, 0,
							sx, sy, skull.w / SPRITE_EDGE, skull.h / SPRITE_EDGE,
						);
					}
				}
			}
		},

		draw(cam, deviceWidth, deviceHeight, dpr) {
			if (count === 0 || !texture) {
				return;
			}

			gl.useProgram(program);
			gl.bindVertexArray(vao);
			gl.bindBuffer(gl.ARRAY_BUFFER, instances);
			gl.bufferData(gl.ARRAY_BUFFER, data.subarray(0, count * FLOATS_PER_INSTANCE), gl.DYNAMIC_DRAW);

			gl.activeTexture(gl.TEXTURE0);
			gl.bindTexture(gl.TEXTURE_2D_ARRAY, texture);
			gl.uniform1i(at.u_sprites, 0);
			gl.uniform1f(at.u_scale, cam.zoom * dpr);
			gl.uniformMatrix3fv(at.u_clip, false, clipMatrix(cam, deviceWidth, deviceHeight, dpr, matrix));

			gl.enable(gl.BLEND);
			gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
			gl.drawArraysInstanced(gl.TRIANGLE_STRIP, 0, 4, count);
			gl.disable(gl.BLEND);

			gl.bindVertexArray(null);
			gl.bindBuffer(gl.ARRAY_BUFFER, null);
		},

		dispose() {
			gl.deleteProgram(program);
			gl.deleteVertexArray(vao);
			gl.deleteBuffer(corners);
			gl.deleteBuffer(instances);
		},
	};
}

// fitFactors is how much of the quad the picture covers on each axis.
//
// ONE EXPRESSION FOR BOTH FITS, and the only difference is min or max. Contain
// shrinks the image until it is inside the quad, which letterboxes it; cover
// grows it until the quad is inside the image, which crops it. Contain is an
// object -- a wagon must be its own shape on the floor -- and cover is a
// creature, whose disc must not have bars in it.
//
// A factor over one means the image runs off the edge of the quad and the
// fragments past it are simply not drawn; a factor under one means the quad has
// nothing in it out there, which is the letterbox.
export function fitFactors(
	spriteW: number,
	spriteH: number,
	halfW: number,
	halfH: number,
	cover: boolean,
): [number, number] {
	if (spriteW <= 0 || spriteH <= 0 || halfW <= 0 || halfH <= 0) {
		return [1, 1];
	}

	const image = spriteW / spriteH;
	const quad = halfW / halfH;
	const pick = cover ? Math.max : Math.min;

	return [pick(1, image / quad), pick(1, quad / image)];
}
