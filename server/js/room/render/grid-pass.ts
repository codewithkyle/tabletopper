// The grid, drawn as ONE TRIANGLE AND NO GEOMETRY.
//
// The obvious implementation is a line list: two vertices per grid line,
// rebuilt whenever the camera or the cell size changes. It is also the wrong
// one. A 12000 by 9000 map at 64 pixel cells is 328 lines, so the buffer has to
// be regenerated on every pan, the lines are one PHYSICAL pixel wide only if
// the driver's line rasterisation agrees with you, and lines are the one
// primitive whose behaviour genuinely differs between drivers.
//
// So the fragment shader is asked the question instead: for the map pixel under
// THIS device pixel, how far is the nearest cell edge? The answer is a distance
// in device pixels via fwidth, which is exactly the derivative the hardware
// already computes for texture sampling. The line is one device pixel wide at
// every zoom by construction, the cost is one triangle, and nothing is rebuilt
// when anything changes.

import { createProgram, fullscreenTriangle, uniforms } from "./gl.ts";
import type { Grid } from "../protocol.ts";
import type { Camera, Rect } from "./camera.ts";
import { inverseClipMatrix } from "./camera.ts";

const vertexSource = `#version 300 es
layout(location = 0) in vec2 a_clip;

uniform mat3 u_clipToWorld;

out vec2 v_world;

void main() {
	v_world = (u_clipToWorld * vec3(a_clip, 1.0)).xy;
	gl_Position = vec4(a_clip, 0.0, 1.0);
}
`;

// u_rect is the map rectangle the grid is confined to. Outside it the fragment
// is discarded, so a grid never runs off the edge of the map it belongs to --
// and a layer with no map passes a rectangle big enough to never clip, which is
// how a GM choosing a cell size before choosing a map still sees what they are
// choosing.
//
// THE FADE IS NOT DECORATION. Below about two device pixels per cell the lines
// are closer together than the pixels that would draw them, and what comes out
// is moire rather than a grid. Fading between two and six pixels means zooming
// out to the whole map dissolves the grid instead of painting a slab of colour
// over it.
const fragmentSource = `#version 300 es
precision highp float;

in vec2 v_world;

uniform vec4 u_rect;
uniform vec2 u_offset;
uniform float u_cell;
uniform vec4 u_color;

out vec4 outColor;

void main() {
	if (v_world.x < u_rect.x || v_world.y < u_rect.y || v_world.x > u_rect.z || v_world.y > u_rect.w) {
		discard;
	}

	vec2 cells = (v_world - u_offset) / u_cell;
	vec2 perPixel = fwidth(cells);
	vec2 toEdge = abs(fract(cells - 0.5) - 0.5) / max(perPixel, vec2(1e-8));

	float line = 1.0 - clamp(min(toEdge.x, toEdge.y), 0.0, 1.0);
	float fade = smoothstep(2.0, 6.0, 1.0 / max(max(perPixel.x, perPixel.y), 1e-8));

	float alpha = u_color.a * line * fade;
	if (alpha <= 0.0) {
		discard;
	}

	outColor = vec4(u_color.rgb, alpha);
}
`;

const names = ["u_clipToWorld", "u_rect", "u_offset", "u_cell", "u_color"] as const;

// NO_MAP is the rectangle a layer without one is clipped to: far enough out
// that no reachable camera position finds its edge, and small enough that it
// stays exact in a 32 bit float.
const NO_MAP: Rect = { x1: -1e7, y1: -1e7, x2: 1e7, y2: 1e7 };

export interface GridPass {
	draw(cam: Camera, grid: Grid, map: Rect | null, deviceWidth: number, deviceHeight: number, dpr: number): void;
	dispose(): void;
}

export function createGridPass(gl: WebGL2RenderingContext): GridPass {
	const program = createProgram(gl, vertexSource, fragmentSource);
	const at = uniforms(gl, program, names);
	const vao = fullscreenTriangle(gl);

	const matrix = new Float32Array(9);
	const color = new Float32Array(4);

	return {
		draw(cam, grid, map, deviceWidth, deviceHeight, dpr) {
			if (!grid.visible || grid.cellSize < 1) {
				return;
			}

			parseColor(grid.color, color);
			if (color[3] <= 0) {
				return;
			}

			const rect = map ?? NO_MAP;

			gl.useProgram(program);
			gl.bindVertexArray(vao);

			gl.uniformMatrix3fv(at.u_clipToWorld, false, inverseClipMatrix(cam, deviceWidth, deviceHeight, dpr, matrix));
			gl.uniform4f(at.u_rect, rect.x1, rect.y1, rect.x2, rect.y2);

			// The offset is reduced into one cell here rather than in the
			// shader. The state deliberately keeps it unreduced -- a GM nudging
			// past a cell boundary must not jump back -- and a large value
			// spends float precision the grid needs for fract.
			gl.uniform2f(at.u_offset, wrap(grid.offsetX, grid.cellSize), wrap(grid.offsetY, grid.cellSize));
			gl.uniform1f(at.u_cell, grid.cellSize);
			gl.uniform4f(at.u_color, color[0], color[1], color[2], color[3]);

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

// wrap is a modulo that answers in [0, size) for negative inputs too, which the
// % operator does not.
function wrap(value: number, size: number): number {
	return ((value % size) + size) % size;
}

// parseColor reads the grid's colour into a normalised RGBA.
//
// THE PROTOCOL ACCEPTS SIX OR EIGHT HEX DIGITS and validates that in Go, so
// this is a reader rather than a validator: anything it does not recognise
// becomes opaque black, which is the state's own default and is visible. A grid
// that silently vanished because of a bad colour would be reported as the grid
// being broken.
export function parseColor(value: string, out: Float32Array): Float32Array {
	out[0] = 0;
	out[1] = 0;
	out[2] = 0;
	out[3] = 1;

	const hex = value.startsWith("#") ? value.slice(1) : value;
	if ((hex.length !== 6 && hex.length !== 8) || !/^[0-9a-fA-F]+$/.test(hex)) {
		return out;
	}

	out[0] = parseInt(hex.slice(0, 2), 16) / 255;
	out[1] = parseInt(hex.slice(2, 4), 16) / 255;
	out[2] = parseInt(hex.slice(4, 6), 16) / 255;
	if (hex.length === 8) {
		out[3] = parseInt(hex.slice(6, 8), 16) / 255;
	}

	return out;
}
