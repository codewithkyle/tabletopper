import type { Camera } from "./camera.ts";
import type { Grid } from "../protocol.ts";
import { blended } from "../gl/blend.ts";
import { createProgram } from "../gl/program.ts";
import { fullscreenTriangle } from "../gl/fullscreen.ts";
import { inverseClipMatrix } from "./camera.ts";
import { parseColor } from "../model/color.ts";
import { fragmentSource, uniforms, vertexSource } from "./shaders/grid.ts";
export interface GridPass {
	draw(cam: Camera, grid: Grid, deviceWidth: number, deviceHeight: number, dpr: number): void;
	dispose(): void;
}
export function createGridPass(gl: WebGL2RenderingContext): GridPass {
	const program = createProgram(gl, vertexSource, fragmentSource, uniforms);
	const vao = fullscreenTriangle(gl);
	const matrix = new Float32Array(9);
	const color = new Float32Array(4);
	const flush = () => gl.drawArrays(gl.TRIANGLES, 0, 3);
	return {
		draw(cam, grid, deviceWidth, deviceHeight, dpr) {
			if (grid.lines === "off" || grid.cellSize < 1) {
				return;
			}
			parseColor(grid.color, color);
			if (color[3] <= 0) {
				return;
			}
			program.use();
			gl.bindVertexArray(vao);
			gl.uniformMatrix3fv(program.at.u_clipToWorld, false, inverseClipMatrix(cam, deviceWidth, deviceHeight, dpr, matrix));
			gl.uniform2f(program.at.u_offset, wrap(grid.offsetX, grid.cellSize), wrap(grid.offsetY, grid.cellSize));
			gl.uniform1f(program.at.u_cell, grid.cellSize);
			gl.uniform4f(program.at.u_color, color[0], color[1], color[2], color[3]);
			gl.uniform1f(program.at.u_dash, grid.lines === "dashed" ? 1 : 0);
			blended(gl, flush);
			gl.bindVertexArray(null);
		},
		dispose() {
			program.dispose();
			gl.deleteVertexArray(vao);
		},
	};
}
function wrap(value: number, size: number): number {
	return ((value % size) + size) % size;
}
