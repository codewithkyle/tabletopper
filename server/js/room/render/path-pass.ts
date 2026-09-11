import type { Camera } from "./camera.ts";
import type { GlyphAtlas } from "./glyphs.ts";
import { clipMatrix } from "./camera.ts";
import { createProgram, uniforms } from "./gl.ts";
const FLOATS_PER_INSTANCE = 16;
const LABEL_PIXELS = 13;
const LABEL_LIFT = 10;
const HALO_COLOR: readonly [number, number, number] = [0.04, 0.04, 0.06];
const HALO_PIXELS = 1.25;
const HALO_RING: readonly (readonly [number, number])[] = [
	[1, 0],
	[0.7071, 0.7071],
	[0, 1],
	[-0.7071, 0.7071],
	[-1, 0],
	[-0.7071, -0.7071],
	[0, -1],
	[0.7071, -0.7071],
];
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
	begin(worldPerPixel: number): void;
	cell(x: number, y: number, size: number, color: readonly [number, number, number], alpha: number): void;
	line(
		x0: number, y0: number, x1: number, y1: number,
		width: number, color: readonly [number, number, number], alpha: number,
	): void;
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
	function segment(
		x: number, y: number, dx: number, dy: number,
		width: number, color: readonly [number, number, number], alpha: number,
	): void {
		const half = width / 2 / Math.hypot(dx, dy);
		const nx = -dy * half;
		const ny = dx * half;
		push(x - nx, y - ny, dx, dy, nx * 2, ny * 2, 0, color, alpha, 0, 0, 0, 0);
	}
	function run(
		text: string, left: number, top: number, height: number,
		color: readonly [number, number, number], alpha: number,
	): void {
		if (!atlas) {
			return;
		}
		let pen = left;
		for (const char of text) {
			const glyph = atlas.get(char);
			if (!glyph) {
				continue;
			}
			if (glyph.width > 0) {
				push(pen, top, glyph.width * height, 0, 0, height, 1, color, alpha, glyph.u0, glyph.v0, glyph.u1, glyph.v1);
			}
			pen += glyph.advance * height;
		}
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
			const grow = HALO_PIXELS * scale;
			const ux = (dx / length) * grow;
			const uy = (dy / length) * grow;
			segment(
				x0 - ux, y0 - uy, dx + ux * 2, dy + uy * 2,
				width * scale + grow * 2, HALO_COLOR, alpha,
			);
			segment(x0, y0, dx, dy, width * scale, color, alpha);
		},
		label(text, x, y, color, alpha) {
			if (!atlas) {
				return;
			}
			const height = LABEL_PIXELS * scale;
			const width = atlas.measure(text) * height;
			const left = x - width / 2;
			const top = y - LABEL_LIFT * scale - height;
			const reach = HALO_PIXELS * scale;
			for (const [ox, oy] of HALO_RING) {
				run(text, left + ox * reach, top + oy * reach, height, HALO_COLOR, alpha);
			}
			run(text, left, top, height, color, alpha);
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
