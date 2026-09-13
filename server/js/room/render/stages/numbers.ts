import type { Stage, StageFactory } from "./stage.ts";
import type { Rect, Rgb } from "../../model/types.ts";
import { cellCentre, cellExtents } from "../../model/grid.ts";
import { createPathPass } from "../path-pass.ts";
import { numberingFor } from "../../model/numbering.ts";
import { parseColor } from "../../model/color.ts";
import { visibleRect } from "../camera.ts";
const HEIGHT = 0.22;
const INSET = 0.06;
const FADE_FROM = 6;
const FADE_TO = 11;
export interface Ink {
	ink: Rgb;
	alpha: number;
}
const read = new Float32Array(4);
export function inkFor(color: string): Ink {
	parseColor(color, read);
	return { ink: [read[0], read[1], read[2]], alpha: read[3] };
}
function fade(pixels: number): number {
	if (pixels <= FADE_FROM) {
		return 0;
	}
	if (pixels >= FADE_TO) {
		return 1;
	}
	return (pixels - FADE_FROM) / (FADE_TO - FADE_FROM);
}
export const numbersStage: StageFactory = (gl, resources): Stage => {
	const pass = createPathPass(gl, resources.atlas);
	const seen: Rect = { x1: 0, y1: 0, x2: 0, y2: 0 };
	return {
		build(frame) {
			pass.begin(frame.worldPerCssPixel);
			const grid = frame.state.table.grid;
			const numbering = numberingFor(grid, frame.viewed?.map ?? frame.viewed?.gmMap);
			if (!numbering) {
				return;
			}
			const cell = Math.max(1, grid.cellSize);
			const height = cell * HEIGHT;
			const { ink, alpha } = inkFor(grid.color);
			const shown = alpha * fade(height / frame.worldPerCssPixel);
			if (shown <= 0) {
				return;
			}
			const top = cellExtents(grid)[1] - cell * INSET;
			visibleRect(frame.camera, frame.viewport, seen);
			numbering.each(seen.x1, seen.y1, seen.x2, seen.y2, (q, r, n) => {
				const [x, y] = cellCentre(grid, q, r);
				pass.caption(String(n), x, y - top, height, ink, shown);
			});
		},
		draw(frame) {
			pass.draw(frame);
		},
		dispose: () => pass.dispose(),
	};
};
