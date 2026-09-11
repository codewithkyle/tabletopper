import type { FrameContext } from "./frame-context.ts";
import type { Rgb } from "../model/types.ts";
import { SHAPED_QUAD, writeShaped } from "./shaped-quad.ts";
import { blended } from "../gl/blend.ts";
import { createProgram } from "../gl/program.ts";
import { createQuadBatch } from "../gl/quads.ts";
import { fragmentSource, uniforms, vertexSource } from "./shaders/aura.ts";
export const AURA_DISC = 0;
export const AURA_RECT = 1;
const AURA_PERIOD = 6000;
export interface AuraPass {
	begin(): void;
	add(
		x: number, y: number, halfW: number, halfH: number,
		color: Rgb, alpha: number,
		shape: number, rotation?: number,
	): void;
	draw(frame: FrameContext, turn: number): void;
	dispose(): void;
}
export function auraTurn(now: number): number {
	return (((now % AURA_PERIOD) + AURA_PERIOD) % AURA_PERIOD) / AURA_PERIOD;
}
export function createAuraPass(gl: WebGL2RenderingContext): AuraPass {
	const program = createProgram(gl, vertexSource, fragmentSource, uniforms);
	const batch = createQuadBatch(gl, SHAPED_QUAD, 16);
	const flush = () => batch.draw();
	return {
		begin() {
			batch.begin();
		},
		add(x, y, halfW, halfH, color, alpha, shape, rotation = 0) {
			writeShaped(batch, x, y, halfW, halfH, color, alpha, 0, shape, 0, 0, rotation);
		},
		draw(frame, turn) {
			if (batch.count === 0) {
				return;
			}
			batch.upload();
			program.use();
			gl.uniform1f(program.at.u_scale, frame.scale);
			gl.uniform1f(program.at.u_turn, turn);
			gl.uniformMatrix3fv(program.at.u_clip, false, frame.clip);
			blended(gl, flush);
		},
		dispose() {
			program.dispose();
			batch.dispose();
		},
	};
}
