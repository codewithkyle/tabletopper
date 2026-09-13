import type { FrameContext } from "./frame-context.ts";
import type { Grid } from "../protocol.ts";
import { blended } from "../gl/blend.ts";
import { counted } from "../gl/counters.ts";
import { createProgram } from "../gl/program.ts";
import { fullscreenTriangle } from "../gl/fullscreen.ts";
import { parseColor } from "../model/color.ts";
import { fragmentSource, uniforms, vertexSource } from "./shaders/grid.ts";
export interface GridPass {
	draw(frame: FrameContext, grid: Grid): void;
	dispose(): void;
}
export function createGridPass(gl: WebGL2RenderingContext): GridPass {
	const program = createProgram(gl, vertexSource, fragmentSource, uniforms);
	const vao = fullscreenTriangle(gl);
	const color = new Float32Array(4);
	const flush = () => {
		gl.drawArrays(gl.TRIANGLES, 0, 3);
		counted(1);
	};
	return {
		draw(frame, grid) {
			if (grid.lines === "off" || grid.cellSize < 1) {
				return;
			}
			parseColor(grid.color, color);
			if (color[3] <= 0) {
				return;
			}
			program.use();
			gl.bindVertexArray(vao);
			gl.uniformMatrix3fv(program.at.u_clipToWorld, false, frame.clipInverse);
			const [ox, oy] = origin(grid);
			gl.uniform2f(program.at.u_offset, ox, oy);
			gl.uniform1f(program.at.u_cell, grid.cellSize);
			gl.uniform4f(program.at.u_color, color[0], color[1], color[2], color[3]);
			gl.uniform1f(program.at.u_dash, grid.lines === "dashed" ? 1 : 0);
			gl.uniform1f(program.at.u_type, TYPES[grid.type] ?? 0);
			blended(gl, flush);
			gl.bindVertexArray(null);
		},
		dispose() {
			program.dispose();
			gl.deleteVertexArray(vao);
		},
	};
}
const SQRT3 = Math.sqrt(3);
const TYPES: Record<string, number> = { square: 0, hexPointy: 1, hexFlat: 2 };
function origin(grid: Grid): [number, number] {
	const cell = grid.cellSize;
	if (grid.type === "square") {
		return [wrap(grid.offsetX, cell), wrap(grid.offsetY, cell)];
	}
	const x = grid.offsetX + cell / 2;
	const y = grid.offsetY + cell / 2;
	if (grid.type === "hexFlat") {
		return [wrap(x, cell * SQRT3), wrap(y, cell)];
	}
	return [wrap(x, cell), wrap(y, cell * SQRT3)];
}
function wrap(value: number, size: number): number {
	return ((value % size) + size) % size;
}
