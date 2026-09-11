import type { FrameContext } from "./frame-context.ts";
import type { Grid, HPBand, Pawn } from "../protocol.ts";
import type { Attribute, QuadBatch } from "../gl/quads.ts";
import type { Program } from "../gl/program.ts";
import type { SpriteCache } from "./sprites.ts";
import type { Rgb } from "../model/types.ts";
import { SKULL, SPRITE_SIZE } from "./sprites.ts";
import { BLOOD_DRIED, BLOOD_FRESH, KIND_COLORS } from "../model/color.ts";
import { BEAT_NONE, beats, bleeds, bloodSprite, hurt, seed } from "../model/health.ts";
import { blended } from "../gl/blend.ts";
import { createProgram } from "../gl/program.ts";
import { createQuadBatch } from "../gl/quads.ts";
import { pawnExtents, radians } from "../model/shape.ts";
import { compareStack } from "../model/stack.ts";
import { fragmentSource, uniforms, vertexSource } from "./shaders/pawn.ts";
export const HIDDEN_ALPHA = 0.6;
const HIDDEN_GREY = 0.7;
const SKULL_SCALE = 0.7;
const SHAPE_DISC = 0;
const SHAPE_RECT = 1;
const SHAPE_STAIN = 2;
const STAIN_ALPHA = 0.62;
const PAWN_QUAD: readonly Attribute[] = [{ size: 4 }, { size: 4 }, { size: 4 }, { size: 4 }, { size: 4 }];
export type PawnProgram = Program<(typeof uniforms)[number]>;
export function createPawnProgram(gl: WebGL2RenderingContext): PawnProgram {
	return createProgram(gl, vertexSource, fragmentSource, uniforms);
}
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
	draw(frame: FrameContext, pulse?: PawnPulse): void;
	beating(): boolean;
	dispose(): void;
}
function push(
	batch: QuadBatch,
	x: number, y: number, halfW: number, halfH: number,
	border: Rgb, borderAlpha: number,
	layer: number, shape: number, alpha: number, grey: number,
	kx: number, ky: number, uvW: number, uvH: number,
	cos: number, sin: number, wounded: number, beat: number,
): void {
	const at = batch.cursor();
	const data = batch.data;
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
}
export function createPawnPass(gl: WebGL2RenderingContext, program: PawnProgram): PawnPass {
	const batch = createQuadBatch(gl, PAWN_QUAD, 64);
	let texture: WebGLTexture | null = null;
	let pulsing = false;
	let order: number[] = [];
	const flush = () => batch.draw();
	return {
		build(pawns, grid, sprites, alpha = 1) {
			batch.begin();
			pulsing = false;
			texture = sprites.texture();
			batch.reserve(pawns.length * 3);
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
					batch,
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
							batch,
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
							batch,
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
		draw(frame, pulse) {
			if (batch.count === 0 || !texture) {
				return;
			}
			batch.upload();
			program.use();
			gl.activeTexture(gl.TEXTURE0);
			gl.bindTexture(gl.TEXTURE_2D_ARRAY, texture);
			gl.uniform1i(program.at.u_sprites, 0);
			gl.uniform1f(program.at.u_scale, frame.scale);
			gl.uniform2f(program.at.u_pulse, pulse?.slow ?? 0, pulse?.heart ?? 0);
			gl.uniformMatrix3fv(program.at.u_clip, false, frame.clip);
			blended(gl, flush);
		},
		dispose() {
			batch.dispose();
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
