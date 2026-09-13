import type { FrameContext } from "./frame-context.ts";
import type { Grid } from "../protocol.ts";
import type { Laid } from "./stages/terrain.ts";
import type { PawnProgram } from "./pawn-pass.ts";
import type { Rgb } from "../model/types.ts";
import type { SpriteCache } from "./sprites.ts";
import { BEAT_NONE } from "../model/health.ts";
import { PAWN_QUAD, SHAPE_HEX, SHAPE_HEX_FLAT, SHAPE_RECT, fitFactors, pushQuad } from "./pawn-pass.ts";
import { SPRITE_SIZE } from "./sprites.ts";
import { blended } from "../gl/blend.ts";
import { cellExtents } from "../model/grid.ts";
import { createQuadBatch } from "../gl/quads.ts";
import { radians } from "../model/shape.ts";
const UNBORDERED: Rgb = [0, 0, 0];
export interface TerrainPass {
	build(laid: readonly Laid[], grid: Grid, sprites: SpriteCache, alpha?: number): void;
	draw(frame: FrameContext): void;
	drawn(): number;
	dispose(): void;
}
export function maskOf(grid: Grid): number {
	if (grid.type === "hexPointy") {
		return SHAPE_HEX;
	}
	if (grid.type === "hexFlat") {
		return SHAPE_HEX_FLAT;
	}
	return SHAPE_RECT;
}
export function createTerrainPass(gl: WebGL2RenderingContext, program: PawnProgram): TerrainPass {
	const batch = createQuadBatch(gl, PAWN_QUAD, 64);
	let texture: WebGLTexture | null = null;
	const flush = (): void => batch.draw();
	return {
		build(laid, grid, sprites, alpha = 1) {
			batch.begin();
			texture = sprites.texture();
			batch.reserve(laid.length);
			const [halfW, halfH] = cellExtents(grid);
			const mask = maskOf(grid);
			for (const tile of laid) {
				const slot = sprites.sprite(tile.image, 0);
				if (!slot) {
					continue;
				}
				const [kx, ky] = fitFactors(slot.w, slot.h, halfW, halfH, true);
				const turn = radians(tile.rotation);
				pushQuad(
					batch,
					tile.x, tile.y, halfW, halfH,
					UNBORDERED, 0,
					slot.layer, mask, alpha, 0,
					kx, ky,
					slot.w / SPRITE_SIZE, slot.h / SPRITE_SIZE,
					Math.cos(turn), Math.sin(turn), 0, BEAT_NONE,
				);
			}
		},
		draw(frame) {
			if (batch.count === 0 || !texture) {
				return;
			}
			batch.upload();
			program.use();
			gl.activeTexture(gl.TEXTURE0);
			gl.bindTexture(gl.TEXTURE_2D_ARRAY, texture);
			gl.uniform1i(program.at.u_sprites, 0);
			gl.uniform1f(program.at.u_scale, frame.scale);
			gl.uniform2f(program.at.u_pulse, 0, 0);
			gl.uniformMatrix3fv(program.at.u_clip, false, frame.clip);
			blended(gl, flush);
		},
		drawn: () => batch.count,
		dispose() {
			batch.dispose();
		},
	};
}
