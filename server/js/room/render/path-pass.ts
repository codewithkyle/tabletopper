// The ruler: the cells a drag passes through, the line across them, and the
// distance in feet.
//
// ONE PROGRAM FOR THREE THINGS, because all three are quads. A highlighted cell
// is an axis-aligned rectangle, the line is the same rectangle rotated, and a
// character of the label is that rectangle with a piece of the glyph atlas on
// it -- so an instance is an origin and two axis vectors, which covers every
// one of them without a branch, and the whole ruler is one draw call.
//
// EVERYTHING IS WORLD SPACE BY THE TIME IT REACHES THE BUFFER. A line two CSS
// pixels wide and a label thirteen CSS pixels tall have to stay that size at
// every zoom, and the conversion happens once per frame in begin() rather than
// per fragment in a shader: the camera is settled before any pass runs, so the
// scale is a number the caller already has.
//
// TWO OF THESE ARE CREATED, NOT ONE. The cells go UNDER the pawns -- a
// highlight is the floor, and a pawn standing on it must not be tinted -- and
// the line and the label go over them. Two instances of this pass with two
// buffers is how that is expressed; one pass with an internal split would be
// the same two buffers with a worse name.

import type { Camera } from "./camera.ts";
import type { GlyphAtlas } from "./glyphs.ts";
import { clipMatrix } from "./camera.ts";
import { createProgram, uniforms } from "./gl.ts";

// FLOATS_PER_INSTANCE: the origin and first axis, the second axis and the
// textured flag, the colour, and the atlas rectangle. Four vec4s.
const FLOATS_PER_INSTANCE = 16;

// LABEL_PIXELS is the label's height in CSS pixels, and LABEL_LIFT is how far
// above its anchor it sits. Both are fixed sizes on the screen rather than on
// the table: a ruler that shrank as you zoomed out would stop being readable
// exactly when the distance got long enough to need reading.
const LABEL_PIXELS = 13;
const LABEL_LIFT = 10;

const vertexSource = `#version 300 es
layout(location = 0) in vec2 a_corner;
layout(location = 1) in vec4 a_rect;
layout(location = 2) in vec4 a_axis;
layout(location = 3) in vec4 a_color;
layout(location = 4) in vec4 a_uv;

uniform mat3 u_clip;

out vec2 v_uv;
flat out vec4 v_color;
flat out float v_textured;

void main() {
	vec2 world = a_rect.xy + a_corner.x * a_rect.zw + a_corner.y * a_axis.xy;

	v_uv = mix(a_uv.xy, a_uv.zw, a_corner);
	v_color = a_color;
	v_textured = a_axis.z;

	gl_Position = vec4((u_clip * vec3(world, 1.0)).xy, 0.0, 1.0);
}
`;

const fragmentSource = `#version 300 es
precision highp float;

in vec2 v_uv;
flat in vec4 v_color;
flat in float v_textured;

uniform sampler2D u_atlas;

out vec4 outColor;

void main() {
	float alpha = v_color.a;

	// The atlas is white on transparency, so its alpha IS the character's
	// coverage and the colour is whatever the caller asked for. That is what
	// lets one atlas serve a label per player, each in their own colour.
	if (v_textured > 0.5) {
		alpha *= texture(u_atlas, v_uv).a;
	}

	if (alpha <= 0.0) {
		discard;
	}

	outColor = vec4(v_color.rgb, alpha);
}
`;

const names = ["u_clip", "u_atlas"] as const;

export interface PathPass {
	// begin empties the buffer and records how many map pixels one CSS pixel
	// is, which is what turns a line's width and a label's height into world
	// units. Everything added afterwards is in map pixels.
	begin(worldPerPixel: number): void;

	// cell fills one square of the grid.
	cell(x: number, y: number, size: number, color: readonly [number, number, number], alpha: number): void;

	// line is one segment of the ruler, its width in CSS pixels.
	line(
		x0: number, y0: number, x1: number, y1: number,
		width: number, color: readonly [number, number, number], alpha: number,
	): void;

	// label draws text centred above a map point. It draws only the characters
	// the atlas holds; see glyphs.ts for why that is the whole vocabulary.
	label(text: string, x: number, y: number, color: readonly [number, number, number], alpha: number): void;

	draw(cam: Camera, deviceWidth: number, deviceHeight: number, dpr: number): void;
	dispose(): void;
}

export function createPathPass(gl: WebGL2RenderingContext, atlas: GlyphAtlas | null): PathPass {
	const program = createProgram(gl, vertexSource, fragmentSource);
	const at = uniforms(gl, program, names);

	const vao = gl.createVertexArray();
	gl.bindVertexArray(vao);

	const corners = gl.createBuffer();
	gl.bindBuffer(gl.ARRAY_BUFFER, corners);
	gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([0, 0, 1, 0, 0, 1, 1, 1]), gl.STATIC_DRAW);
	gl.enableVertexAttribArray(0);
	gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);

	const instances = gl.createBuffer();
	gl.bindBuffer(gl.ARRAY_BUFFER, instances);
	const stride = FLOATS_PER_INSTANCE * 4;
	for (let i = 0; i < 4; i++) {
		gl.enableVertexAttribArray(1 + i);
		gl.vertexAttribPointer(1 + i, 4, gl.FLOAT, false, stride, i * 16);
		gl.vertexAttribDivisor(1 + i, 1);
	}

	gl.bindVertexArray(null);
	gl.bindBuffer(gl.ARRAY_BUFFER, null);

	let data = new Float32Array(128 * FLOATS_PER_INSTANCE);
	let count = 0;
	let scale = 1;

	const matrix = new Float32Array(9);

	// A one pixel white texture stands in when there is no atlas, so the
	// textured branch is never taken against an unbound sampler -- which some
	// drivers answer with black and others with a warning.
	const blank = gl.createTexture();
	gl.bindTexture(gl.TEXTURE_2D, blank);
	gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, new Uint8Array([255, 255, 255, 255]));
	gl.bindTexture(gl.TEXTURE_2D, null);

	function push(
		ox: number, oy: number, axx: number, axy: number,
		ayx: number, ayy: number, textured: number,
		color: readonly [number, number, number], alpha: number,
		u0: number, v0: number, u1: number, v1: number,
	): void {
		const floats = (count + 1) * FLOATS_PER_INSTANCE;
		if (floats > data.length) {
			let size = data.length;
			while (size < floats) {
				size *= 2;
			}

			const grown = new Float32Array(size);
			grown.set(data);
			data = grown;
		}

		const i = count * FLOATS_PER_INSTANCE;

		data[i] = ox;
		data[i + 1] = oy;
		data[i + 2] = axx;
		data[i + 3] = axy;

		data[i + 4] = ayx;
		data[i + 5] = ayy;
		data[i + 6] = textured;
		data[i + 7] = 0;

		data[i + 8] = color[0];
		data[i + 9] = color[1];
		data[i + 10] = color[2];
		data[i + 11] = alpha;

		data[i + 12] = u0;
		data[i + 13] = v0;
		data[i + 14] = u1;
		data[i + 15] = v1;

		count++;
	}

	return {
		begin(worldPerPixel) {
			count = 0;
			scale = worldPerPixel;
		},

		cell(x, y, size, color, alpha) {
			push(x, y, size, 0, 0, size, 0, color, alpha, 0, 0, 0, 0);
		},

		line(x0, y0, x1, y1, width, color, alpha) {
			const dx = x1 - x0;
			const dy = y1 - y0;
			const length = Math.hypot(dx, dy);
			if (length <= 0) {
				return;
			}

			// The quad runs along the segment and is half its width either
			// side, so the line is centred on the path rather than hanging off
			// one edge of it.
			const half = (width * scale) / 2;
			const nx = (-dy / length) * half;
			const ny = (dx / length) * half;

			push(x0 - nx, y0 - ny, dx, dy, nx * 2, ny * 2, 0, color, alpha, 0, 0, 0, 0);
		},

		label(text, x, y, color, alpha) {
			if (!atlas) {
				return;
			}

			const height = LABEL_PIXELS * scale;
			const width = atlas.measure(text) * height;

			// Centred on the anchor and lifted above it, both in world units
			// derived from CSS pixels -- so the label sits the same distance
			// off the pawn at every zoom.
			let pen = x - width / 2;
			const top = y - LABEL_LIFT * scale - height;

			for (const char of text) {
				const glyph = atlas.get(char);
				if (!glyph) {
					continue;
				}

				if (glyph.width > 0) {
					const w = glyph.width * height;
					push(pen, top, w, 0, 0, height, 1, color, alpha, glyph.u0, glyph.v0, glyph.u1, glyph.v1);
				}

				pen += glyph.advance * height;
			}
		},

		draw(cam, deviceWidth, deviceHeight, dpr) {
			if (count === 0) {
				return;
			}

			gl.useProgram(program);
			gl.bindVertexArray(vao);
			gl.bindBuffer(gl.ARRAY_BUFFER, instances);
			gl.bufferData(gl.ARRAY_BUFFER, data.subarray(0, count * FLOATS_PER_INSTANCE), gl.DYNAMIC_DRAW);

			gl.activeTexture(gl.TEXTURE0);
			gl.bindTexture(gl.TEXTURE_2D, atlas ? atlas.texture : blank);
			gl.uniform1i(at.u_atlas, 0);
			gl.uniformMatrix3fv(at.u_clip, false, clipMatrix(cam, deviceWidth, deviceHeight, dpr, matrix));

			gl.enable(gl.BLEND);
			gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
			gl.drawArraysInstanced(gl.TRIANGLE_STRIP, 0, 4, count);
			gl.disable(gl.BLEND);

			gl.bindVertexArray(null);
			gl.bindBuffer(gl.ARRAY_BUFFER, null);
		},

		dispose() {
			gl.deleteProgram(program);
			gl.deleteVertexArray(vao);
			gl.deleteBuffer(corners);
			gl.deleteBuffer(instances);
			gl.deleteTexture(blank);
		},
	};
}
