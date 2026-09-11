import type { FrameContext } from "./frame-context.ts";
import type { Program } from "../gl/program.ts";
import type { Rgb } from "../model/types.ts";
import { SHAPED_QUAD, writeShaped } from "./shaped-quad.ts";
import { blended } from "../gl/blend.ts";
import { createProgram } from "../gl/program.ts";
import { createQuadBatch } from "../gl/quads.ts";
import { fragmentSource, uniforms, vertexSource } from "./shaders/ring.ts";
export const RING_ELLIPSE = 0;
export const RING_RECT = 1;
export interface RingPass {
	begin(): void;
	add(
		x: number, y: number, halfW: number, halfH: number,
		color: Rgb, alpha: number,
		thickness: number, shape: number, rotation?: number,
	): void;
	draw(frame: FrameContext): void;
	dispose(): void;
}
export type RingProgram = Program<(typeof uniforms)[number]>;
export function createRingProgram(gl: WebGL2RenderingContext): RingProgram {
	return createProgram(gl, vertexSource, fragmentSource, uniforms);
}
export function createRingPass(gl: WebGL2RenderingContext, program: RingProgram): RingPass {
	const batch = createQuadBatch(gl, SHAPED_QUAD, 64);
	const flush = () => batch.draw();
	return {
		begin() {
			batch.begin();
		},
		add(x, y, halfW, halfH, color, alpha, thickness, shape, rotation = 0) {
			writeShaped(batch, x, y, halfW, halfH, color, alpha, thickness, shape, 0, 0, rotation);
		},
		draw(frame) {
			if (batch.count === 0) {
				return;
			}
			batch.upload();
			program.use();
			gl.uniform1f(program.at.u_scale, frame.scale);
			gl.uniformMatrix3fv(program.at.u_clip, false, frame.clip);
			blended(gl, flush);
		},
		dispose() {
			batch.dispose();
		},
	};
}
