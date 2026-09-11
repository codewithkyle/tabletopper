import { createProgram, fullscreenTriangle, uniforms } from "./gl.ts";
import type { Grid } from "../protocol.ts";
import type { Camera } from "./camera.ts";
import { inverseClipMatrix } from "./camera.ts";
import { parseColor } from "../model/color.ts";
const vertexSource = `#version 300 es
layout(location = 0) in vec2 a_clip;
uniform mat3 u_clipToWorld;
out vec2 v_world;
void main() {
	v_world = (u_clipToWorld * vec3(a_clip, 1.0)).xy;
	gl_Position = vec4(a_clip, 0.0, 1.0);
}
`;
const fragmentSource = `#version 300 es
precision highp float;
in vec2 v_world;
uniform vec2 u_offset;
uniform float u_cell;
uniform vec4 u_color;
uniform float u_dash;
out vec4 outColor;
const float DASH_PERIOD = 0.25;
const float DASH_HALF = 0.3;
float dash(float along, float perPixel) {
	float s = abs(fract(along / DASH_PERIOD + 0.5) - 0.5);
	float w = max(perPixel / DASH_PERIOD, 1e-8);
	return 1.0 - smoothstep(DASH_HALF - w, DASH_HALF + w, s);
}
void main() {
	vec2 cells = (v_world - u_offset) / u_cell;
	vec2 perPixel = fwidth(cells);
	vec2 toEdge = abs(fract(cells - 0.5) - 0.5) / max(perPixel, vec2(1e-8));
	vec2 line = 1.0 - clamp(toEdge, 0.0, 1.0);
	if (u_dash > 0.5) {
		line.x *= dash(cells.y, perPixel.y);
		line.y *= dash(cells.x, perPixel.x);
	}
	float fade = smoothstep(2.0, 6.0, 1.0 / max(max(perPixel.x, perPixel.y), 1e-8));
	float alpha = u_color.a * max(line.x, line.y) * fade;
	if (alpha <= 0.0) {
		discard;
	}
	outColor = vec4(u_color.rgb, alpha);
}
`;
const names = ["u_clipToWorld", "u_offset", "u_cell", "u_color", "u_dash"] as const;
export interface GridPass {
	draw(cam: Camera, grid: Grid, deviceWidth: number, deviceHeight: number, dpr: number): void;
	dispose(): void;
}
export function createGridPass(gl: WebGL2RenderingContext): GridPass {
	const program = createProgram(gl, vertexSource, fragmentSource);
	const at = uniforms(gl, program, names);
	const vao = fullscreenTriangle(gl);
	const matrix = new Float32Array(9);
	const color = new Float32Array(4);
	return {
		draw(cam, grid, deviceWidth, deviceHeight, dpr) {
			if (grid.lines === "off" || grid.cellSize < 1) {
				return;
			}
			parseColor(grid.color, color);
			if (color[3] <= 0) {
				return;
			}
			gl.useProgram(program);
			gl.bindVertexArray(vao);
			gl.uniformMatrix3fv(at.u_clipToWorld, false, inverseClipMatrix(cam, deviceWidth, deviceHeight, dpr, matrix));
			gl.uniform2f(at.u_offset, wrap(grid.offsetX, grid.cellSize), wrap(grid.offsetY, grid.cellSize));
			gl.uniform1f(at.u_cell, grid.cellSize);
			gl.uniform4f(at.u_color, color[0], color[1], color[2], color[3]);
			gl.uniform1f(at.u_dash, grid.lines === "dashed" ? 1 : 0);
			gl.enable(gl.BLEND);
			gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
			gl.drawArrays(gl.TRIANGLES, 0, 3);
			gl.disable(gl.BLEND);
			gl.bindVertexArray(null);
		},
		dispose() {
			gl.deleteProgram(program);
			gl.deleteVertexArray(vao);
		},
	};
}
function wrap(value: number, size: number): number {
	return ((value % size) + size) % size;
}
