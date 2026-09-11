import type { Camera } from "./camera.ts";
import type { Stroke } from "../protocol.ts";
import { clipMatrix } from "./camera.ts";
import { createProgram, uniforms } from "./gl.ts";
import { parseColor } from "../model/color.ts";
import { strokeSegments } from "../model/stroke.ts";
const FLOATS_PER_INSTANCE = 9;
const MIN_HALF_DEVICE = 0.75;
const vertexSource = `#version 300 es
layout(location = 0) in vec2 a_corner;
layout(location = 1) in vec4 a_seg;
layout(location = 2) in vec4 a_color;
layout(location = 3) in float a_half;
uniform mat3 u_clip;
uniform float u_minHalf;
out vec2 v_local;
flat out vec4 v_color;
flat out float v_half;
flat out float v_len;
void main() {
	float radius = max(a_half, u_minHalf);
	vec2 p0 = a_seg.xy;
	vec2 delta = a_seg.zw - p0;
	float len = length(delta);
	vec2 dir = len > 1e-4 ? delta / len : vec2(1.0, 0.0);
	vec2 nor = vec2(-dir.y, dir.x);
	float lx = mix(-radius, len + radius, a_corner.x);
	float ly = mix(-radius, radius, a_corner.y);
	v_local = vec2(lx, ly);
	v_color = a_color;
	v_half = radius;
	v_len = len;
	vec2 world = p0 + dir * lx + nor * ly;
	gl_Position = vec4((u_clip * vec3(world, 1.0)).xy, 0.0, 1.0);
}
`;
const fragmentSource = `#version 300 es
precision highp float;
in vec2 v_local;
flat in vec4 v_color;
flat in float v_half;
flat in float v_len;
out vec4 outColor;
void main() {
	float t = clamp(v_local.x, 0.0, v_len);
	float dist = length(v_local - vec2(t, 0.0));
	float aa = max(fwidth(dist), 1e-5);
	float alpha = v_color.a * (1.0 - smoothstep(v_half - aa, v_half + aa, dist));
	if (alpha <= 0.0) {
		discard;
	}
	outColor = vec4(v_color.rgb, alpha);
}
`;
const names = ["u_clip", "u_minHalf"] as const;
export interface StrokePass {
	sync(strokes: readonly Stroke[], layerID: string): void;
	live(strokes: readonly Stroke[], layerID: string, own: Stroke | null): void;
	draw(cam: Camera, deviceWidth: number, deviceHeight: number, dpr: number): void;
	dispose(): void;
}
interface Batch {
	vao: WebGLVertexArrayObject;
	buffer: WebGLBuffer;
	data: Float32Array;
	count: number;
}
export function createStrokePass(gl: WebGL2RenderingContext): StrokePass {
	const program = createProgram(gl, vertexSource, fragmentSource);
	const at = uniforms(gl, program, names);
	const corners = gl.createBuffer();
	gl.bindBuffer(gl.ARRAY_BUFFER, corners);
	gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([0, 0, 1, 0, 0, 1, 1, 1]), gl.STATIC_DRAW);
	gl.bindBuffer(gl.ARRAY_BUFFER, null);
	const done = newBatch(gl, corners, 1024);
	const drawing = newBatch(gl, corners, 64);
	let key = "";
	const color = new Float32Array(4);
	const segments: number[] = [];
	const matrix = new Float32Array(9);
	function build(batch: Batch, strokes: readonly Stroke[]): void {
		batch.count = 0;
		for (const stroke of strokes) {
			segments.length = 0;
			strokeSegments(stroke, segments);
			if (segments.length === 0) {
				continue;
			}
			parseColor(stroke.color, color);
			const half = Math.max(stroke.width, 1) / 2;
			for (let i = 0; i + 3 < segments.length; i += 4) {
				push(batch, segments[i], segments[i + 1], segments[i + 2], segments[i + 3], half);
			}
		}
	}
	function push(batch: Batch, x0: number, y0: number, x1: number, y1: number, half: number): void {
		const floats = (batch.count + 1) * FLOATS_PER_INSTANCE;
		if (floats > batch.data.length) {
			let size = batch.data.length;
			while (size < floats) {
				size *= 2;
			}
			const grown = new Float32Array(size);
			grown.set(batch.data);
			batch.data = grown;
		}
		const i = batch.count * FLOATS_PER_INSTANCE;
		batch.data[i] = x0;
		batch.data[i + 1] = y0;
		batch.data[i + 2] = x1;
		batch.data[i + 3] = y1;
		batch.data[i + 4] = color[0];
		batch.data[i + 5] = color[1];
		batch.data[i + 6] = color[2];
		batch.data[i + 7] = color[3];
		batch.data[i + 8] = half;
		batch.count++;
	}
	function upload(batch: Batch): void {
		gl.bindBuffer(gl.ARRAY_BUFFER, batch.buffer);
		gl.bufferData(gl.ARRAY_BUFFER, batch.data.subarray(0, batch.count * FLOATS_PER_INSTANCE), gl.DYNAMIC_DRAW);
		gl.bindBuffer(gl.ARRAY_BUFFER, null);
	}
	function run(batch: Batch): void {
		if (batch.count === 0) {
			return;
		}
		gl.bindVertexArray(batch.vao);
		gl.drawArraysInstanced(gl.TRIANGLE_STRIP, 0, 4, batch.count);
	}
	const finished: Stroke[] = [];
	const live: Stroke[] = [];
	return {
		sync(strokes, layerID) {
			finished.length = 0;
			let newest = "";
			for (const stroke of strokes) {
				if (stroke.layerId !== layerID || !stroke.done) {
					continue;
				}
				finished.push(stroke);
				newest = stroke.id;
			}
			const next = layerID + "|" + finished.length + "|" + newest;
			if (next === key) {
				return;
			}
			key = next;
			build(done, finished);
			upload(done);
		},
		live(strokes, layerID, own) {
			live.length = 0;
			for (const stroke of strokes) {
				if (stroke.layerId !== layerID || stroke.done || stroke.id === own?.id) {
					continue;
				}
				live.push(stroke);
			}
			if (own && own.layerId === layerID) {
				live.push(own);
			}
			if (live.length === 0) {
				drawing.count = 0;
				return;
			}
			build(drawing, live);
			upload(drawing);
		},
		draw(cam, deviceWidth, deviceHeight, dpr) {
			if (done.count === 0 && drawing.count === 0) {
				return;
			}
			gl.useProgram(program);
			gl.uniformMatrix3fv(at.u_clip, false, clipMatrix(cam, deviceWidth, deviceHeight, dpr, matrix));
			gl.uniform1f(at.u_minHalf, MIN_HALF_DEVICE / Math.max(cam.zoom * dpr, 1e-4));
			gl.enable(gl.BLEND);
			gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
			run(done);
			run(drawing);
			gl.disable(gl.BLEND);
			gl.bindVertexArray(null);
		},
		dispose() {
			gl.deleteProgram(program);
			gl.deleteVertexArray(done.vao);
			gl.deleteVertexArray(drawing.vao);
			gl.deleteBuffer(done.buffer);
			gl.deleteBuffer(drawing.buffer);
			gl.deleteBuffer(corners);
		},
	};
}
function newBatch(gl: WebGL2RenderingContext, corners: WebGLBuffer, instances: number): Batch {
	const vao = gl.createVertexArray();
	gl.bindVertexArray(vao);
	gl.bindBuffer(gl.ARRAY_BUFFER, corners);
	gl.enableVertexAttribArray(0);
	gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
	const buffer = gl.createBuffer();
	gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
	const stride = FLOATS_PER_INSTANCE * 4;
	gl.enableVertexAttribArray(1);
	gl.vertexAttribPointer(1, 4, gl.FLOAT, false, stride, 0);
	gl.vertexAttribDivisor(1, 1);
	gl.enableVertexAttribArray(2);
	gl.vertexAttribPointer(2, 4, gl.FLOAT, false, stride, 16);
	gl.vertexAttribDivisor(2, 1);
	gl.enableVertexAttribArray(3);
	gl.vertexAttribPointer(3, 1, gl.FLOAT, false, stride, 32);
	gl.vertexAttribDivisor(3, 1);
	gl.bindVertexArray(null);
	gl.bindBuffer(gl.ARRAY_BUFFER, null);
	return { vao, buffer, data: new Float32Array(instances * FLOATS_PER_INSTANCE), count: 0 };
}
