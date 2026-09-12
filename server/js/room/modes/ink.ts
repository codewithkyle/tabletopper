import type { DrawOptions } from "./draw.ts";
import type { Grid } from "../protocol.ts";
import type { Outline, Overlay } from "../model/overlay.ts";
import type { Point, Rgb } from "../model/types.ts";
import { blankLabel, blankOutline, blankSegment, pool } from "../model/overlay.ts";
import { coneCorners, measure } from "../model/stroke.ts";
import { parseColor } from "../model/color.ts";
const PREVIEW_WIDTH = 2;
const ERASE_COLOR: Rgb = [0.98, 0.98, 0.99];
const ERASE_WIDTH = 1.5;
export type Shaped = "rect" | "circle" | "cone";
export interface Sketch {
	kind: Shaped;
	x0: number;
	y0: number;
	x1: number;
	y1: number;
}
export interface Ink {
	reset(): void;
	show(out: Overlay, sketch: Sketch | null, pointer: Point | null, reach: number): void;
}
export interface InkDeps {
	grid: () => Grid;
	options: () => DrawOptions;
}
export function newInk(deps: InkDeps): Ink {
	const segments = pool(blankSegment);
	const labels = pool(blankLabel);
	const corners: number[] = [];
	const tint = new Float32Array(4);
	const color: [number, number, number] = [1, 1, 1];
	const ring: Outline = blankOutline();
	ring.color = ERASE_COLOR;
	ring.alpha = 0.9;
	ring.thickness = ERASE_WIDTH;
	const preview: Outline = blankOutline();
	preview.color = color;
	preview.alpha = 0.95;
	preview.thickness = PREVIEW_WIDTH;
	function paint(): void {
		parseColor(deps.options().color, tint);
		color[0] = tint[0];
		color[1] = tint[1];
		color[2] = tint[2];
	}
	function outline(sketch: Sketch | null, pointer: Point | null, reach: number): Outline | null {
		if (sketch) {
			if (sketch.kind === "cone") {
				return null;
			}
			paint();
			if (sketch.kind === "rect") {
				const halfW = Math.abs(sketch.x1 - sketch.x0) / 2;
				const halfH = Math.abs(sketch.y1 - sketch.y0) / 2;
				if (halfW <= 0 || halfH <= 0) {
					return null;
				}
				preview.x = (sketch.x0 + sketch.x1) / 2;
				preview.y = (sketch.y0 + sketch.y1) / 2;
				preview.halfW = halfW;
				preview.halfH = halfH;
				preview.rect = true;
				return preview;
			}
			const r = Math.hypot(sketch.x1 - sketch.x0, sketch.y1 - sketch.y0);
			if (r <= 0) {
				return null;
			}
			preview.x = sketch.x0;
			preview.y = sketch.y0;
			preview.halfW = r;
			preview.halfH = r;
			preview.rect = false;
			return preview;
		}
		if (deps.options().mode !== "erase" || !pointer) {
			return null;
		}
		ring.x = pointer.x;
		ring.y = pointer.y;
		ring.halfW = reach;
		ring.halfH = reach;
		return ring;
	}
	function cone(out: Overlay, sketch: Sketch): void {
		coneCorners(sketch.x0, sketch.y0, sketch.x1, sketch.y1, corners);
		if (corners.length === 0) {
			return;
		}
		paint();
		for (let i = 0; i < 3; i++) {
			const j = (i + 1) % 3;
			const slot = segments.take();
			slot.x0 = corners[i * 2];
			slot.y0 = corners[i * 2 + 1];
			slot.x1 = corners[j * 2];
			slot.y1 = corners[j * 2 + 1];
			slot.color = color;
			slot.alpha = 0.95;
			slot.width = PREVIEW_WIDTH;
			out.segments.push(slot);
		}
	}
	function distances(out: Overlay, sketch: Sketch): void {
		measure(
			sketch.kind,
			[sketch.x0, sketch.y0, sketch.x1, sketch.y1],
			deps.grid(), deps.options().color,
			(text, x, y, tinted) => {
				const slot = labels.take();
				parseColor(tinted, tint);
				slot.text = text;
				slot.x = x;
				slot.y = y;
				slot.color[0] = tint[0];
				slot.color[1] = tint[1];
				slot.color[2] = tint[2];
				slot.alpha = 1;
				out.labels.push(slot);
			},
		);
	}
	return {
		reset() {
			segments.reset();
			labels.reset();
		},
		show(out, sketch, pointer, reach) {
			const shape = outline(sketch, pointer, reach);
			if (shape) {
				out.outlines.push(shape);
			}
			if (!sketch) {
				return;
			}
			if (sketch.kind === "cone") {
				cone(out, sketch);
			}
			distances(out, sketch);
		},
	};
}
