import type { Camera } from "./camera.ts";
import { clipMatrix } from "./camera.ts";
import { createProgram, uniforms } from "./gl.ts";
import { radians } from "../model/shape.ts";
import type { Rgb } from "../model/types.ts";
const FLOATS_PER_INSTANCE = 14;
export const RING_ELLIPSE = 0;
export const RING_RECT = 1;
const vertexSource = `#version 300 es
layout(location = 0) in vec2 a_corner;
layout(location = 1) in vec4 a_rect;
layout(location = 2) in vec4 a_color;
layout(location = 3) in vec4 a_style;
layout(location = 4) in vec2 a_spin;
uniform mat3 u_clip;
uniform float u_scale;
out vec2 v_local;
flat out vec4 v_color;
flat out vec2 v_half;
flat out vec2 v_style;
void main() {
	v_local = a_corner * 2.0 - 1.0;
	v_color = a_color;
	v_half = a_rect.zw;
	v_style = vec2(a_style.x / max(u_scale, 1e-4), a_style.y);
	vec2 grown = a_rect.zw + v_style.x;
	vec2 offset = v_local * grown;
	vec2 turned = vec2(
		offset.x * a_spin.x - offset.y * a_spin.y,
		offset.x * a_spin.y + offset.y * a_spin.x
	);
	vec2 world = a_rect.xy + turned;
	gl_Position = vec4((u_clip * vec3(world, 1.0)).xy, 0.0, 1.0);
	v_local *= grown / max(a_rect.zw, vec2(1e-4));
}
`;
const fragmentSource = `#version 300 es
precision highp float;
in vec2 v_local;
flat in vec4 v_color;
flat in vec2 v_half;
flat in vec2 v_style;
out vec4 outColor;
void main() {
	float thickness = v_style.x;
	float inside;
	if (v_style.y < 0.5) {
		float r = length(v_local);
		inside = (1.0 - r) * v_half.x;
	} else {
		vec2 edge = (vec2(1.0) - abs(v_local)) * v_half;
		inside = min(edge.x, edge.y);
	}
	float aa = max(fwidth(inside), 1e-5);
	float alpha = smoothstep(-thickness * 0.5 - aa, -thickness * 0.5, inside)
		* (1.0 - smoothstep(thickness * 0.5, thickness * 0.5 + aa, inside));
	if (alpha <= 0.0) {
		discard;
	}
	outColor = vec4(v_color.rgb, v_color.a * alpha);
}
`;
const names = ["u_clip", "u_scale"] as const;
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
	for (let i = 0; i < 3; i++) {
		gl.enableVertexAttribArray(1 + i);
		gl.vertexAttribPointer(1 + i, 4, gl.FLOAT, false, stride, i * 16);
		gl.vertexAttribDivisor(1 + i, 1);
	}
	gl.enableVertexAttribArray(4);
	gl.vertexAttribPointer(4, 2, gl.FLOAT, false, stride, 48);
	gl.vertexAttribDivisor(4, 1);
	gl.bindVertexArray(null);
	gl.bindBuffer(gl.ARRAY_BUFFER, null);
	let data = new Float32Array(64 * FLOATS_PER_INSTANCE);
	let count = 0;
	const matrix = new Float32Array(9);
	return {
		begin() {
			count = 0;
		},
		add(x, y, halfW, halfH, color, alpha, thickness, shape, rotation = 0) {
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
			const at = count * FLOATS_PER_INSTANCE;
			data[at] = x;
			data[at + 1] = y;
			data[at + 2] = halfW;
			data[at + 3] = halfH;
			data[at + 4] = color[0];
			data[at + 5] = color[1];
			data[at + 6] = color[2];
			data[at + 7] = alpha;
			data[at + 8] = thickness;
			data[at + 9] = shape;
			data[at + 10] = 0;
			data[at + 11] = 0;
			const angle = radians(rotation);
			data[at + 12] = rotation === 0 ? 1 : Math.cos(angle);
			data[at + 13] = rotation === 0 ? 0 : Math.sin(angle);
			count++;
		},
		draw(cam, deviceWidth, deviceHeight, dpr) {
			if (count === 0) {
				return;
			}
			gl.useProgram(program);
			gl.bindVertexArray(vao);
			gl.bindBuffer(gl.ARRAY_BUFFER, instances);
			gl.bufferData(gl.ARRAY_BUFFER, data.subarray(0, count * FLOATS_PER_INSTANCE), gl.DYNAMIC_DRAW);
			gl.uniform1f(at.u_scale, cam.zoom * dpr);
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
		},
	};
}
