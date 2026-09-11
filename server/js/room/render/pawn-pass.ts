import type { Camera } from "./camera.ts";
import type { Grid, HPBand, Pawn } from "../protocol.ts";
import type { SpriteCache } from "./sprites.ts";
import { SKULL, SPRITE_SIZE } from "./sprites.ts";
import { BLOOD_DRIED, BLOOD_FRESH, KIND_COLORS } from "../model/color.ts";
import { BEAT_NONE, beats, bleeds, bloodSprite, hurt, seed } from "../model/health.ts";
import { clipMatrix } from "./camera.ts";
import { createProgram, uniforms } from "./gl.ts";
import { pawnExtents, radians } from "../model/shape.ts";
import { compareStack } from "../model/stack.ts";
import type { Rgb } from "../model/types.ts";
const FLOATS_PER_INSTANCE = 20;
const BORDER_PIXELS = 2;
export const HIDDEN_ALPHA = 0.6;
const HIDDEN_GREY = 0.7;
const SKULL_SCALE = 0.7;
const SHAPE_DISC = 0;
const SHAPE_RECT = 1;
const SHAPE_STAIN = 2;
const STAIN_ALPHA = 0.62;
const BLOOD_INNER = 0.25;
const PULSE_REACH = 0.24;
const vertexSource = `#version 300 es
#define BORDER_PIXELS ${BORDER_PIXELS}.0
layout(location = 0) in vec2 a_corner;
layout(location = 1) in vec4 a_rect;
layout(location = 2) in vec4 a_border;
layout(location = 3) in vec4 a_style;
layout(location = 4) in vec4 a_fit;
layout(location = 5) in vec4 a_spin;
uniform mat3 u_clip;
uniform float u_scale;
out vec2 v_local;
flat out vec4 v_border;
flat out vec4 v_style;
flat out vec4 v_fit;
flat out vec2 v_wound;
flat out float v_edge;
void main() {
	v_local = a_corner * 2.0 - 1.0;
	v_border = a_border;
	v_style = a_style;
	v_fit = a_fit;
	v_wound = a_spin.zw;
	v_edge = BORDER_PIXELS / max(a_rect.z * u_scale, 1e-4);
	vec2 offset = v_local * a_rect.zw;
	vec2 turned = vec2(
		offset.x * a_spin.x - offset.y * a_spin.y,
		offset.x * a_spin.y + offset.y * a_spin.x
	);
	vec2 world = a_rect.xy + turned;
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
flat in vec2 v_wound;
flat in float v_edge;
uniform sampler2DArray u_sprites;
uniform vec2 u_pulse;
const vec3 LUMA = vec3(0.299, 0.587, 0.114);
const vec3 BLOOD = vec3(0.42, 0.03, 0.03);
const vec3 PALLOR = vec3(0.82, 0.88, 1.0);
const vec3 PULSE = vec3(1.0, 0.24, 0.2);
out vec4 outColor;
void main() {
	float layer = v_style.x;
	float shape = v_style.y;
	float alpha = v_style.z;
	float grey  = v_style.w;
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
		rgb = mix(v_border.rgb, picture.rgb, picture.a);
		float border = smoothstep(1.0 - v_edge - aa, 1.0 - v_edge, r);
		rgb = mix(rgb, v_border.rgb, border * v_border.a);
		float wounded = v_wound.x;
		if (wounded > 0.0) {
			float low = 0.55 + 0.45 * v_local.y;
			float rim = smoothstep(1.0 - (0.3 + 0.35 * wounded), 1.0, r) * low;
			rgb = mix(rgb, BLOOD, clamp(rim, 0.0, 1.0) * wounded);
			rgb = mix(rgb, PALLOR * dot(rgb, LUMA), 0.35 * wounded);
		}
		float beating = v_wound.y;
		if (beating > 0.5) {
			float amount = beating < 1.5 ? u_pulse.x : u_pulse.y;
			rgb = mix(rgb, PULSE, smoothstep(1.0 - ${PULSE_REACH}, 1.0, r) * amount);
		}
	} else if (shape < 1.5) {
		cover = picture.a;
		if (cover <= 0.0) {
			discard;
		}
		rgb = picture.rgb;
	} else {
		float r = length(v_local);
		float aa = max(fwidth(r), 1e-5);
		cover = picture.a
			* (1.0 - smoothstep(1.0 - aa, 1.0, r))
			* smoothstep(${BLOOD_INNER}, 1.0, r);
		if (cover <= 0.0) {
			discard;
		}
		rgb = v_border.rgb * picture.r;
	}
	rgb = mix(rgb, vec3(dot(rgb, LUMA)), grey);
	outColor = vec4(rgb, cover * alpha);
}
`;
const names = ["u_clip", "u_sprites", "u_scale", "u_pulse"] as const;
export interface Drawn {
	id: string;
	kind: Pawn["kind"];
	name: string;
	image: string;
	x: number;
	y: number;
	z: number;
	size: Pawn["size"];
	width: number;
	height: number;
	rotation: number;
	hidden: boolean;
	health: HPBand | null;
}
export interface PawnPulse {
	slow: number;
	heart: number;
}
export interface PawnPass {
	build(pawns: readonly Drawn[], grid: Grid, sprites: SpriteCache, alpha?: number): void;
	draw(cam: Camera, deviceWidth: number, deviceHeight: number, dpr: number, pulse?: PawnPulse): void;
	beating(): boolean;
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
	gl.enableVertexAttribArray(5);
	gl.vertexAttribPointer(5, 4, gl.FLOAT, false, stride, 64);
	gl.vertexAttribDivisor(5, 1);
	gl.bindVertexArray(null);
	gl.bindBuffer(gl.ARRAY_BUFFER, null);
	let data = new Float32Array(64 * FLOATS_PER_INSTANCE);
	let count = 0;
	let texture: WebGLTexture | null = null;
	let pulsing = false;
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
		border: Rgb, borderAlpha: number,
		layer: number, shape: number, alpha: number, grey: number,
		kx: number, ky: number, uvW: number, uvH: number,
		cos: number, sin: number, wounded: number, beat: number,
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
		data[at + 16] = cos;
		data[at + 17] = sin;
		data[at + 18] = wounded;
		data[at + 19] = beat;
		count++;
	}
	return {
		build(pawns, grid, sprites, alpha = 1) {
			count = 0;
			pulsing = false;
			texture = sprites.texture();
			reserve(pawns.length * 3);
			if (order.length !== pawns.length) {
				order = pawns.map((_, i) => i);
			} else {
				for (let i = 0; i < pawns.length; i++) {
					order[i] = i;
				}
			}
			order.sort((a, b) => compareStack(pawns[a], pawns[b]));
			for (const index of order) {
				const pawn = pawns[index];
				const [halfW, halfH] = pawnExtents(pawn, grid.cellSize);
				const object = pawn.kind === "object";
				const dead = pawn.health === "dead";
				const opacity = alpha * (pawn.hidden ? HIDDEN_ALPHA : 1);
				const grey = pawn.hidden ? HIDDEN_GREY : dead ? 1 : 0;
				const wounded = object ? 0 : hurt(pawn.health);
				const beat = object ? BEAT_NONE : beats(pawn.health);
				if (beat !== BEAT_NONE) {
					pulsing = true;
				}
				const slot = sprites.sprite(pawn.image, 0) ?? sprites.initials(pawn.kind, pawn.name);
				const layer = slot ? slot.layer : -1;
				const [kx, ky] = slot
					? fitFactors(slot.w, slot.h, halfW, halfH, !object)
					: [1, 1];
				const angle = radians(pawn.rotation);
				const cos = pawn.rotation === 0 ? 1 : Math.cos(angle);
				const sin = pawn.rotation === 0 ? 0 : Math.sin(angle);
				push(
					pawn.x, pawn.y, halfW, halfH,
					KIND_COLORS[pawn.kind] ?? KIND_COLORS.npc,
					object ? 0 : 1,
					layer, object ? SHAPE_RECT : SHAPE_DISC, opacity, grey,
					kx, ky,
					slot ? slot.w / SPRITE_SIZE : 1,
					slot ? slot.h / SPRITE_SIZE : 1,
					cos, sin, wounded, beat,
				);
				if (!object && bleeds(pawn.health)) {
					const mark = seed(pawn.id);
					const stain = sprites.sprite(bloodSprite(mark), 1);
					if (stain) {
						const turn = radians(mark % 360);
						const [bx, by] = fitFactors(stain.w, stain.h, halfW, halfH, true);
						push(
							pawn.x, pawn.y, halfW, halfH,
							dead ? BLOOD_DRIED : BLOOD_FRESH, 0,
							stain.layer, SHAPE_STAIN, opacity * STAIN_ALPHA,
							pawn.hidden ? HIDDEN_GREY : 0,
							bx, by, stain.w / SPRITE_SIZE, stain.h / SPRITE_SIZE,
							Math.cos(turn), Math.sin(turn), 0, BEAT_NONE,
						);
					}
				}
				if (dead && !object) {
					const skull = sprites.glyph(SKULL);
					if (skull) {
						const size = halfW * SKULL_SCALE;
						const [sx, sy] = fitFactors(skull.w, skull.h, size, size, false);
						push(
							pawn.x, pawn.y, size, size,
							KIND_COLORS[pawn.kind] ?? KIND_COLORS.npc, 0,
							skull.layer, SHAPE_RECT, opacity, 0,
							sx, sy, skull.w / SPRITE_SIZE, skull.h / SPRITE_SIZE,
							1, 0, 0, BEAT_NONE,
						);
					}
				}
			}
		},
		beating: () => pulsing,
		draw(cam, deviceWidth, deviceHeight, dpr, pulse) {
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
			gl.uniform2f(at.u_pulse, pulse?.slow ?? 0, pulse?.heart ?? 0);
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
