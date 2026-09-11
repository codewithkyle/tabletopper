import type { Camera } from "./camera.ts";
import type { Rgb } from "../model/types.ts";
import { clipMatrix } from "./camera.ts";
import { createProgram, uniforms } from "./gl.ts";
import { radians } from "../model/shape.ts";
const FLOATS_PER_INSTANCE = 14;
export const AURA_DISC = 0;
export const AURA_RECT = 1;
const AURA_PERIOD = 6000;
const AURA_PAD = 4;
const AURA_NEAR = 4;
const AURA_FAR = 16;
const AURA_NEAR_A = 0.7;
const AURA_FAR_A = 0.3;
const AURA_REACH = AURA_PAD + AURA_FAR + 2;
const AURA_TAIL = 0.6;
const vertexSource = `#version 300 es
#define REACH ${AURA_REACH}.0
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
	vec2 corner = a_corner * 2.0 - 1.0;
	v_color = a_color;
	v_half = a_rect.zw;
	float reach = REACH / max(u_scale, 1e-4);
	v_style = vec2(reach, a_style.y);
	v_local = (a_rect.zw + reach) * corner;
	vec2 turned = vec2(
		v_local.x * a_spin.x - v_local.y * a_spin.y,
		v_local.x * a_spin.y + v_local.y * a_spin.x
	);
	gl_Position = vec4((u_clip * vec3(a_rect.xy + turned, 1.0)).xy, 0.0, 1.0);
}
`;
const fragmentSource = `#version 300 es
precision highp float;
#define PI 3.141592653589793
#define TAU 6.283185307179586
#define PAD ${AURA_PAD}.0
#define NEAR ${AURA_NEAR}.0
#define FAR ${AURA_FAR}.0
#define NEAR_A ${AURA_NEAR_A}
#define FAR_A ${AURA_FAR_A}
#define TAIL ${AURA_TAIL}
in vec2 v_local;
flat in vec4 v_color;
flat in vec2 v_half;
flat in vec2 v_style;
uniform float u_scale;
uniform float u_turn;
out vec4 outColor;
float ramp(float q) {
	return clamp((q - (1.0 - TAIL)) / TAIL, 0.0, 1.0);
}
float dual(float q, float soft) {
	if (soft < 1e-4) {
		return ramp(q);
	}
	float sum = 0.0;
	for (int i = -2; i <= 2; i++) {
		sum += ramp(fract(q + float(i) * soft * 0.6));
	}
	return sum * 0.2;
}
void main() {
	float reach = v_style.x;
	float outside;
	if (v_style.y < 0.5) {
		outside = length(v_local) - v_half.x;
	} else {
		vec2 q = abs(v_local) - v_half;
		outside = length(max(q, 0.0)) + min(max(q.x, q.y), 0.0);
	}
	float aa = max(fwidth(outside), 1e-5);
	float scale = 1.0 / max(u_scale, 1e-4);
	float pad = PAD * scale;
	float near = NEAR * scale;
	float far = FAR * scale;
	float hole = smoothstep(-aa, 0.0, outside);
	if (hole <= 0.0 || outside > reach) {
		discard;
	}
	float turn = fract(atan(v_local.x, -v_local.y) / TAU - u_turn);
	float q = fract(turn * 2.0);
	float r = max(length(v_local), 1e-3);
	float softNear = clamp(near / (PI * r), 0.0, 0.5);
	float softFar = clamp(far / (PI * r), 0.0, 0.5);
	float band = hole * (1.0 - smoothstep(pad, pad + aa, outside));
	float crisp = band * dual(q, 0.0);
	float glowNear = hole * NEAR_A * (1.0 - smoothstep(pad - near, pad + near, outside)) * dual(q, softNear);
	float glowFar = hole * FAR_A * (1.0 - smoothstep(pad - far, pad + far, outside)) * dual(q, softFar);
	float alpha = 1.0 - (1.0 - crisp) * (1.0 - glowNear) * (1.0 - glowFar);
	alpha *= v_color.a;
	if (alpha <= 0.0) {
		discard;
	}
	outColor = vec4(v_color.rgb, alpha);
}
`;
const names = ["u_clip", "u_scale", "u_turn"] as const;
export interface AuraPass {
	begin(): void;
	add(
		x: number, y: number, halfW: number, halfH: number,
		color: Rgb, alpha: number,
		shape: number, rotation?: number,
	): void;
	draw(cam: Camera, deviceWidth: number, deviceHeight: number, dpr: number, turn: number): void;
	dispose(): void;
}
export function auraTurn(now: number): number {
	return (((now % AURA_PERIOD) + AURA_PERIOD) % AURA_PERIOD) / AURA_PERIOD;
}
export function createAuraPass(gl: WebGL2RenderingContext): AuraPass {
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
	let data = new Float32Array(16 * FLOATS_PER_INSTANCE);
	let count = 0;
	const matrix = new Float32Array(9);
	return {
		begin() {
			count = 0;
		},
		add(x, y, halfW, halfH, color, alpha, shape, rotation = 0) {
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
			data[at + 8] = 0;
			data[at + 9] = shape;
			data[at + 10] = 0;
			data[at + 11] = 0;
			const angle = radians(rotation);
			data[at + 12] = rotation === 0 ? 1 : Math.cos(angle);
			data[at + 13] = rotation === 0 ? 0 : Math.sin(angle);
			count++;
		},
		draw(cam, deviceWidth, deviceHeight, dpr, turn) {
			if (count === 0) {
				return;
			}
			gl.useProgram(program);
			gl.bindVertexArray(vao);
			gl.bindBuffer(gl.ARRAY_BUFFER, instances);
			gl.bufferData(gl.ARRAY_BUFFER, data.subarray(0, count * FLOATS_PER_INSTANCE), gl.DYNAMIC_DRAW);
			gl.uniform1f(at.u_scale, cam.zoom * dpr);
			gl.uniform1f(at.u_turn, turn);
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
