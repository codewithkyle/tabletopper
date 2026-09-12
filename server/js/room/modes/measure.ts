import type { Grid } from "../protocol.ts";
import type { Point } from "../model/types.ts";
import type { Tool } from "../render/input.ts";
import { SELF_COLOR } from "../model/color.ts";
import { blankOutline } from "../model/overlay.ts";
import { lineRuler, newPens } from "./ruler.ts";
const MEASURE_POINT = 4;
const MEASURE_WIDTH = 2;
export interface MeasureDeps {
	grid: () => Grid;
	scale: () => number;
	invalidate: () => void;
}
export function createMeasure(deps: MeasureDeps): Tool {
	let measured: { from: Point; to: Point } | null = null;
	const pens = newPens();
	const point = blankOutline();
	point.color = SELF_COLOR;
	point.alpha = 0.95;
	point.thickness = MEASURE_WIDTH;
	function aim(map: Point): void {
		if (!measured) {
			return;
		}
		measured.to.x = map.x;
		measured.to.y = map.y;
		deps.invalidate();
	}
	function abandon(): boolean {
		if (!measured) {
			return false;
		}
		measured = null;
		deps.invalidate();
		return true;
	}
	return {
		press(map) {
			measured = measured
				? null
				: { from: { x: map.x, y: map.y }, to: { x: map.x, y: map.y } };
			return true;
		},
		drag(map) {
			aim(map);
		},
		release() {},
		cancel() {},
		secondary: abandon,
		hover(map) {
			if (map) {
				aim(map);
			}
		},
		key: () => false,
		abandon,
		active: () => false,
		contribute(out) {
			pens.reset();
			if (!measured) {
				return;
			}
			const radius = MEASURE_POINT * deps.scale();
			point.x = measured.from.x;
			point.y = measured.from.y;
			point.halfW = radius;
			point.halfH = radius;
			out.outlines.push(point);
			lineRuler(
				out, pens, deps.grid(),
				measured.from.x, measured.from.y, measured.to.x, measured.to.y, SELF_COLOR,
			);
		},
	};
}
