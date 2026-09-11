import type { Camera } from "./camera.ts";
import { clipMatrix } from "./camera.ts";
import { createProgram, uniforms } from "./gl.ts";
import { radians } from "../model/shape.ts";
import type { Rgb } from "../model/types.ts";
const FLOATS_PER_INSTANCE = 14;
const vertexSource = `#version 300 es
layout(location = 0) in vec2 a_corner;
layout(location = 1) in vec4 a_rect;
layout(location = 2) in vec4 a_color;
layout(location = 3) in vec4 a_sheet;
layout(location = 4) in vec2 a_spin;
uniform mat3 u_clip;
out vec2 v_local;
flat out vec4 v_color;
flat out vec4 v_sheet;
void main() {
	v_local = a_corner * 2.0 - 1.0;
	v_color = a_color;
	v_sheet = a_sheet;
	vec2 offset = v_local * a_rect.zw;
	vec2 turned = vec2(
		offset.x * a_spin.x - offset.y * a_spin.y,
		offset.x * a_spin.y + offset.y * a_spin.x
	);
	vec2 world = a_rect.xy + turned;
	gl_Position = vec4((u_clip * vec3(world, 1.0)).xy, 0.0, 1.0);
}
`;
const fragmentSource = `#version 300 es
precision highp float;
precision highp sampler2DArray;
in vec2 v_local;
flat in vec4 v_color;
flat in vec4 v_sheet;
uniform sampler2DArray u_sprites;
out vec4 outColor;
void main() {
	vec2 t = (v_local * 0.5 + 0.5) * v_sheet.yz;
	vec4 picture = texture(u_sprites, vec3(t, v_sheet.x));
	float cover = picture.a * v_color.a;
	if (cover <= 0.0) {
		discard;
	}
	outColor = vec4(v_color.rgb * picture.r, cover);
}
`;
const names = ["u_clip", "u_sprites"] as const;
export interface DecalPass {
	begin(): void;
	add(
		x: number, y: number, halfW: number, halfH: number,
		color: Rgb, alpha: number,
		layer: number, uvW: number, uvH: number, rotation: number,
	): void;
	draw(cam: Camera, texture: WebGLTexture, deviceWidth: number, deviceHeight: number, dpr: number): void;
	dispose(): void;
}
export function createDecalPass(gl: WebGL2RenderingContext): DecalPass {
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
		add(x, y, halfW, halfH, color, alpha, layer, uvW, uvH, rotation) {
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
			data[at + 8] = layer;
			data[at + 9] = uvW;
			data[at + 10] = uvH;
			data[at + 11] = 0;
			const angle = radians(rotation);
			data[at + 12] = rotation === 0 ? 1 : Math.cos(angle);
			data[at + 13] = rotation === 0 ? 0 : Math.sin(angle);
			count++;
		},
		draw(cam, texture, deviceWidth, deviceHeight, dpr) {
			if (count === 0) {
				return;
			}
			gl.useProgram(program);
			gl.bindVertexArray(vao);
			gl.bindBuffer(gl.ARRAY_BUFFER, instances);
			gl.bufferData(gl.ARRAY_BUFFER, data.subarray(0, count * FLOATS_PER_INSTANCE), gl.DYNAMIC_DRAW);
			gl.activeTexture(gl.TEXTURE0);
			gl.bindTexture(gl.TEXTURE_2D_ARRAY, texture);
			gl.uniform1i(at.u_sprites, 0);
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
