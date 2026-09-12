import type { FrameContext } from "./frame-context.ts";
import type { FogShape } from "../protocol.ts";
import type { MaskRect } from "../model/polygon.ts";
import { blended } from "../gl/blend.ts";
import { createVertexStream } from "../gl/vertices.ts";
import { createProgram } from "../gl/program.ts";
import { fullscreenTriangle } from "../gl/fullscreen.ts";
import { maskRect, rectTriangles, triangulate } from "../model/polygon.ts";
import {
	coverFragmentSource,
	coverUniforms,
	coverVertexSource,
	shapeFragmentSource,
	shapeUniforms,
	shapeVertexSource,
} from "./shaders/fog.ts";
const MASK_SCALE = 0.25;
const MASK_MAX = 4096;
export interface FogPass {
	sync(
		shapes: readonly FogShape[], layerID: string,
		map: { width: number; height: number } | null,
		prefill: boolean, cell: number,
	): void;
	draw(frame: FrameContext, alpha: number): void;
	dispose(): void;
}
export function createFogPass(gl: WebGL2RenderingContext): FogPass {
	const shapeProgram = createProgram(gl, shapeVertexSource, shapeFragmentSource, shapeUniforms);
	const shapeAt = shapeProgram.at;
	const coverProgram = createProgram(gl, coverVertexSource, coverFragmentSource, coverUniforms);
	const coverAt = coverProgram.at;
	const cover = fullscreenTriangle(gl);
	const flush = () => gl.drawArrays(gl.TRIANGLES, 0, 3);
	const mask = createVertexStream(gl, 2, 1024);
	const texture = gl.createTexture();
	const frame = gl.createFramebuffer();
	let floorID = "";
	const rect: MaskRect = { x: 0, y: 0, width: 0, height: 0 };
	let sized = false;
	let prefilled = false;
	const drawn: string[] = [];
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
			gl.uniform1f(shapeAt.u_open, mode === "hide" ? 0 : 1);
			mask.upload(batch, batch.length);
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
			if (!restart && mine.length >= drawn.length) {
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
			shapeProgram.use();
			mask.bind();
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
		draw(frame, alpha) {
			if (alpha <= 0) {
				return;
			}
			gl.viewport(0, 0, frame.deviceWidth, frame.deviceHeight);
			coverProgram.use();
			gl.bindVertexArray(cover);
			gl.uniformMatrix3fv(coverAt.u_clipToWorld, false, frame.clipInverse);
			if (sized) {
				gl.uniform4f(coverAt.u_rect, rect.x, rect.y, rect.width, rect.height);
			} else {
				gl.uniform4f(coverAt.u_rect, 0, 0, 0, 0);
			}
			gl.uniform1f(coverAt.u_prefill, prefilled ? 1 : 0);
			gl.uniform4f(coverAt.u_color, frame.clear[0], frame.clear[1], frame.clear[2], alpha);
			gl.activeTexture(gl.TEXTURE0);
			gl.bindTexture(gl.TEXTURE_2D, texture);
			gl.uniform1i(coverAt.u_openness, 0);
			blended(gl, flush);
			gl.bindTexture(gl.TEXTURE_2D, null);
			gl.bindVertexArray(null);
		},
		dispose() {
			shapeProgram.dispose();
			coverProgram.dispose();
			gl.deleteVertexArray(cover);
			mask.dispose();
			gl.deleteTexture(texture);
			gl.deleteFramebuffer(frame);
		},
	};
}
