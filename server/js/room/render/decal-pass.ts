import type { FrameContext } from "./frame-context.ts";
import type { Rgb } from "../model/types.ts";
import { SHAPED_QUAD, writeShaped } from "./shaped-quad.ts";
import { blended } from "../gl/blend.ts";
import { createProgram } from "../gl/program.ts";
import { createQuadBatch } from "../gl/quads.ts";
import { fragmentSource, uniforms, vertexSource } from "./shaders/decal.ts";
export interface DecalPass {
	begin(): void;
	add(
		x: number, y: number, halfW: number, halfH: number,
		color: Rgb, alpha: number,
		layer: number, uvW: number, uvH: number, rotation: number,
	): void;
	draw(frame: FrameContext, texture: WebGLTexture): void;
	dispose(): void;
}
export function createDecalPass(gl: WebGL2RenderingContext): DecalPass {
	const program = createProgram(gl, vertexSource, fragmentSource, uniforms);
	const batch = createQuadBatch(gl, SHAPED_QUAD, 64);
	const flush = () => batch.draw();
	return {
		begin() {
			batch.begin();
		},
		add(x, y, halfW, halfH, color, alpha, layer, uvW, uvH, rotation) {
			writeShaped(batch, x, y, halfW, halfH, color, alpha, layer, uvW, uvH, 0, rotation);
		},
		draw(frame, texture) {
			if (batch.count === 0) {
				return;
			}
			batch.upload();
			program.use();
			gl.activeTexture(gl.TEXTURE0);
			gl.bindTexture(gl.TEXTURE_2D_ARRAY, texture);
			gl.uniform1i(program.at.u_sprites, 0);
			gl.uniformMatrix3fv(program.at.u_clip, false, frame.clip);
			blended(gl, flush);
		},
		dispose() {
			program.dispose();
			batch.dispose();
		},
	};
}
