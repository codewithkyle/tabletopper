import type { Camera } from "./camera.ts";
import type { FogShape } from "../protocol.ts";
import type { MaskRect } from "../fog.ts";
import { createProgram, fullscreenTriangle, uniforms } from "./gl.ts";
import { inverseClipMatrix } from "./camera.ts";
import { maskRect, rectTriangles, triangulate } from "../fog.ts";
const MASK_SCALE = 0.25;
const MASK_MAX = 4096;
const shapeVertexSource = `#version 300 es
layout(location = 0) in vec2 a_map;
uniform vec4 u_rect;
void main() {
	vec2 t = (a_map - u_rect.xy) / u_rect.zw;
	gl_Position = vec4(t * 2.0 - 1.0, 0.0, 1.0);
}
`;
const shapeFragmentSource = `#version 300 es
precision highp float;
uniform float u_open;
out vec4 outColor;
void main() {
	outColor = vec4(u_open, 0.0, 0.0, 1.0);
}
`;
const coverVertexSource = `#version 300 es
layout(location = 0) in vec2 a_clip;
uniform mat3 u_clipToWorld;
out vec2 v_world;
void main() {
	v_world = (u_clipToWorld * vec3(a_clip, 1.0)).xy;
	gl_Position = vec4(a_clip, 0.0, 1.0);
}
`;
const coverFragmentSource = `#version 300 es
precision highp float;
in vec2 v_world;
uniform vec4 u_rect;
uniform sampler2D u_openness;
uniform float u_prefill;
uniform vec4 u_color;
out vec4 outColor;
void main() {
	float open = 1.0 - u_prefill;
	if (u_rect.z > 0.0 && u_rect.w > 0.0) {
		vec2 t = (v_world - u_rect.xy) / u_rect.zw;
		if (t.x >= 0.0 && t.x <= 1.0 && t.y >= 0.0 && t.y <= 1.0) {
			open = texture(u_openness, t).r;
		}
	}
	float alpha = u_color.a * (1.0 - smoothstep(0.4, 0.6, open));
	if (alpha <= 0.0) {
		discard;
	}
	outColor = vec4(u_color.rgb, alpha);
}
`;
const shapeNames = ["u_rect", "u_open"] as const;
const coverNames = ["u_clipToWorld", "u_rect", "u_openness", "u_prefill", "u_color"] as const;
export interface FogPass {
	sync(
		shapes: readonly FogShape[], layerID: string,
		map: { width: number; height: number } | null,
		prefill: boolean, cell: number,
	): void;
	draw(
		cam: Camera, deviceWidth: number, deviceHeight: number, dpr: number,
		color: { r: number; g: number; b: number }, alpha: number,
	): void;
	dispose(): void;
}
export function createFogPass(gl: WebGL2RenderingContext): FogPass {
	const shapeProgram = createProgram(gl, shapeVertexSource, shapeFragmentSource);
	const shapeAt = uniforms(gl, shapeProgram, shapeNames);
	const coverProgram = createProgram(gl, coverVertexSource, coverFragmentSource);
	const coverAt = uniforms(gl, coverProgram, coverNames);
	const cover = fullscreenTriangle(gl);
	const matrix = new Float32Array(9);
	const shapeVao = gl.createVertexArray();
	const shapeBuffer = gl.createBuffer();
	gl.bindVertexArray(shapeVao);
	gl.bindBuffer(gl.ARRAY_BUFFER, shapeBuffer);
	gl.enableVertexAttribArray(0);
	gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
	gl.bindVertexArray(null);
	gl.bindBuffer(gl.ARRAY_BUFFER, null);
	let vertices = new Float32Array(1024);
	const texture = gl.createTexture();
	const frame = gl.createFramebuffer();
	let floorID = "";
	const rect: MaskRect = { x: 0, y: 0, width: 0, height: 0 };
	let sized = false;
	let prefilled = false;
	const drawn: string[] = [];
	let signature = "";
	const measured: MaskRect = { x: 0, y: 0, width: 0, height: 0 };
	let width = 0;
	let height = 0;
	const mine: FogShape[] = [];
	const triangles: number[] = [];
	const batch: number[] = [];
	function allocate(w: number, h: number): void {
		if (w === width && h === height) {
			return;
		}
		width = w;
		height = h;
		gl.bindTexture(gl.TEXTURE_2D, texture);
		gl.texImage2D(gl.TEXTURE_2D, 0, gl.R8, w, h, 0, gl.RED, gl.UNSIGNED_BYTE, null);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
		gl.bindFramebuffer(gl.FRAMEBUFFER, frame);
		gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, texture, 0);
		gl.bindFramebuffer(gl.FRAMEBUFFER, null);
		gl.bindTexture(gl.TEXTURE_2D, null);
	}
	function paint(shapes: readonly FogShape[], from: number): void {
		let at = from;
		while (at < shapes.length) {
			const mode = shapes[at].mode;
			batch.length = 0;
			while (at < shapes.length && shapes[at].mode === mode) {
				const shape = shapes[at];
				if (shape.kind === "rect") {
					rectTriangles(shape.points, triangles);
				} else {
					triangulate(shape.points, triangles);
				}
				for (const v of triangles) {
					batch.push(v);
				}
				at++;
			}
			if (batch.length === 0) {
				continue;
			}
			if (vertices.length < batch.length) {
				let size = vertices.length;
				while (size < batch.length) {
					size *= 2;
				}
				vertices = new Float32Array(size);
			}
			vertices.set(batch);
			gl.uniform1f(shapeAt.u_open, mode === "hide" ? 0 : 1);
			gl.bindBuffer(gl.ARRAY_BUFFER, shapeBuffer);
			gl.bufferData(gl.ARRAY_BUFFER, vertices.subarray(0, batch.length), gl.DYNAMIC_DRAW);
			gl.drawArrays(gl.TRIANGLES, 0, batch.length / 2);
		}
	}
	return {
		sync(shapes, layerID, map, prefill, cell) {
			mine.length = 0;
			for (const shape of shapes) {
				if (shape.layerId === layerID) {
					mine.push(shape);
				}
			}
			const now = layerID + "/" + String(prefill) + "/" + mine.length
				+ "/" + (mine.length > 0 ? mine[mine.length - 1].id : "")
				+ "/" + (map ? map.width + "x" + map.height : "")
				+ "/" + cell;
			if (now === signature) {
				return;
			}
			signature = now;
			const next = maskRect(map, mine, layerID, cell, measured);
			if (!next) {
				sized = false;
				floorID = layerID;
				prefilled = prefill;
				drawn.length = 0;
				return;
			}
			const moved = !sized
				|| next.x !== rect.x || next.y !== rect.y
				|| next.width !== rect.width || next.height !== rect.height;
			const restart = moved || layerID !== floorID || prefill !== prefilled;
			let from = 0;
			if (!restart && mine.length > drawn.length) {
				from = drawn.length;
				for (let i = 0; i < drawn.length; i++) {
					if (mine[i].id !== drawn[i]) {
						from = 0;
						break;
					}
				}
			}
			rect.x = next.x;
			rect.y = next.y;
			rect.width = next.width;
			rect.height = next.height;
			sized = true;
			floorID = layerID;
			prefilled = prefill;
			const scale = Math.min(
				MASK_SCALE,
				MASK_MAX / Math.max(rect.width, rect.height, 1),
			);
			allocate(
				Math.max(1, Math.min(MASK_MAX, Math.round(rect.width * scale))),
				Math.max(1, Math.min(MASK_MAX, Math.round(rect.height * scale))),
			);
			gl.bindFramebuffer(gl.FRAMEBUFFER, frame);
			gl.viewport(0, 0, width, height);
			gl.disable(gl.BLEND);
			gl.useProgram(shapeProgram);
			gl.bindVertexArray(shapeVao);
			gl.uniform4f(shapeAt.u_rect, rect.x, rect.y, rect.width, rect.height);
			if (from === 0) {
				const open = prefill ? 0 : 1;
				gl.clearColor(open, 0, 0, 1);
				gl.clear(gl.COLOR_BUFFER_BIT);
			}
			paint(mine, from);
			drawn.length = 0;
			for (const shape of mine) {
				drawn.push(shape.id);
			}
			gl.bindVertexArray(null);
			gl.bindBuffer(gl.ARRAY_BUFFER, null);
			gl.bindFramebuffer(gl.FRAMEBUFFER, null);
		},
		draw(cam, deviceWidth, deviceHeight, dpr, color, alpha) {
			if (alpha <= 0) {
				return;
			}
			gl.viewport(0, 0, deviceWidth, deviceHeight);
			gl.useProgram(coverProgram);
			gl.bindVertexArray(cover);
			gl.uniformMatrix3fv(coverAt.u_clipToWorld, false, inverseClipMatrix(cam, deviceWidth, deviceHeight, dpr, matrix));
			if (sized) {
				gl.uniform4f(coverAt.u_rect, rect.x, rect.y, rect.width, rect.height);
			} else {
				gl.uniform4f(coverAt.u_rect, 0, 0, 0, 0);
			}
			gl.uniform1f(coverAt.u_prefill, prefilled ? 1 : 0);
			gl.uniform4f(coverAt.u_color, color.r, color.g, color.b, alpha);
			gl.activeTexture(gl.TEXTURE0);
			gl.bindTexture(gl.TEXTURE_2D, texture);
			gl.uniform1i(coverAt.u_openness, 0);
			gl.enable(gl.BLEND);
			gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
			gl.drawArrays(gl.TRIANGLES, 0, 3);
			gl.disable(gl.BLEND);
			gl.bindTexture(gl.TEXTURE_2D, null);
			gl.bindVertexArray(null);
		},
		dispose() {
			gl.deleteProgram(shapeProgram);
			gl.deleteProgram(coverProgram);
			gl.deleteVertexArray(cover);
			gl.deleteVertexArray(shapeVao);
			gl.deleteBuffer(shapeBuffer);
			gl.deleteTexture(texture);
			gl.deleteFramebuffer(frame);
		},
	};
}
