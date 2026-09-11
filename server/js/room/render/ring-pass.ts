import type { Camera } from "./camera.ts";
import type { Rgb } from "../model/types.ts";
import { SHAPED_QUAD, writeShaped } from "./shaped-quad.ts";
import { blended } from "../gl/blend.ts";
import { clipMatrix } from "./camera.ts";
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
	draw(cam: Camera, deviceWidth: number, deviceHeight: number, dpr: number): void;
	dispose(): void;
}
export function createRingPass(gl: WebGL2RenderingContext): RingPass {
	const program = createProgram(gl, vertexSource, fragmentSource, uniforms);
	const batch = createQuadBatch(gl, SHAPED_QUAD, 64);
	const matrix = new Float32Array(9);
	const flush = () => batch.draw();
	return {
		begin() {
			batch.begin();
		},
		add(x, y, halfW, halfH, color, alpha, thickness, shape, rotation = 0) {
			writeShaped(batch, x, y, halfW, halfH, color, alpha, thickness, shape, 0, 0, rotation);
		},
		draw(cam, deviceWidth, deviceHeight, dpr) {
			if (batch.count === 0) {
				return;
			}
			batch.upload();
			program.use();
			gl.uniform1f(program.at.u_scale, cam.zoom * dpr);
			gl.uniformMatrix3fv(program.at.u_clip, false, clipMatrix(cam, deviceWidth, deviceHeight, dpr, matrix));
			blended(gl, flush);
		},
		dispose() {
			program.dispose();
			batch.dispose();
		},
	};
}
