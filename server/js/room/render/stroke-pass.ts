import type { FrameContext } from "./frame-context.ts";
import type { QuadBatch } from "../gl/quads.ts";
import type { Stroke } from "../protocol.ts";
import { blended } from "../gl/blend.ts";
import { createProgram } from "../gl/program.ts";
import { createQuadBatch } from "../gl/quads.ts";
import { parseColor } from "../model/color.ts";
import { strokeSegments } from "../model/stroke.ts";
import { fragmentSource, layout, uniforms, vertexSource } from "./shaders/stroke.ts";
const MIN_HALF_DEVICE = 0.75;
const tint = new Float32Array(4);
const segments: number[] = [];
export interface StrokePass {
	sync(strokes: readonly Stroke[], layerID: string): void;
	live(strokes: readonly Stroke[], layerID: string, own: Stroke | null): void;
	draw(frame: FrameContext): void;
	dispose(): void;
}
export function fillStrokes(batch: QuadBatch, strokes: readonly Stroke[]): void {
	batch.begin();
	for (const stroke of strokes) {
		segments.length = 0;
		strokeSegments(stroke, segments);
		if (segments.length === 0) {
			continue;
		}
		parseColor(stroke.color, tint);
		const half = Math.max(stroke.width, 1) / 2;
		for (let i = 0; i + 3 < segments.length; i += 4) {
			const at = batch.cursor();
			const data = batch.data;
			data[at] = segments[i];
			data[at + 1] = segments[i + 1];
			data[at + 2] = segments[i + 2];
			data[at + 3] = segments[i + 3];
			data[at + 4] = tint[0];
			data[at + 5] = tint[1];
			data[at + 6] = tint[2];
			data[at + 7] = tint[3];
			data[at + 8] = half;
		}
	}
}
export function createStrokePass(gl: WebGL2RenderingContext): StrokePass {
	const program = createProgram(gl, vertexSource, fragmentSource, uniforms);
	const done = createQuadBatch(gl, layout, 1024);
	const drawing = createQuadBatch(gl, layout, 64);
	let key = "";
	const finished: Stroke[] = [];
	const live: Stroke[] = [];
	const flush = () => {
		done.draw();
		drawing.draw();
	};
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
			fillStrokes(done, finished);
			done.upload();
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
			fillStrokes(drawing, live);
			drawing.upload();
		},
		draw(frame) {
			if (done.count === 0 && drawing.count === 0) {
				return;
			}
			program.use();
			gl.uniformMatrix3fv(program.at.u_clip, false, frame.clip);
			gl.uniform1f(program.at.u_minHalf, MIN_HALF_DEVICE * frame.worldPerDevicePixel);
			blended(gl, flush);
		},
		dispose() {
			program.dispose();
			done.dispose();
			drawing.dispose();
		},
	};
}
