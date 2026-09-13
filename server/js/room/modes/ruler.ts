import type { Cell, Label, Overlay, Pool, Segment } from "../model/overlay.ts";
import type { Grid } from "../protocol.ts";
import type { Rgb } from "../model/types.ts";
import { blankCell, blankLabel, blankSegment, pool } from "../model/overlay.ts";
import {
	cellAt,
	cellCentre,
	cellPath,
	distanceLabel,
	feetBetween,
	feetMoved,
} from "../model/grid.ts";
import { isHex } from "../model/hex.ts";
const RULER_WIDTH = 2;
const RULER_ALPHA = 0.9;
const CELL_ALPHA = 0.16;
const crossed: number[] = [];
export interface Pens {
	cells: Pool<Cell>;
	segments: Pool<Segment>;
	labels: Pool<Label>;
	reset(): void;
}
export function newPens(): Pens {
	const cells = pool(blankCell);
	const segments = pool(blankSegment);
	const labels = pool(blankLabel);
	return {
		cells,
		segments,
		labels,
		reset() {
			cells.reset();
			segments.reset();
			labels.reset();
		},
	};
}
export function walkRuler(
	out: Overlay, pens: Pens, grid: Grid,
	fromX: number, fromY: number, toX: number, toY: number, color: Rgb,
): void {
	const a = cellAt(grid, fromX, fromY);
	const b = cellAt(grid, toX, toY);
	const size = Math.max(1, grid.cellSize);
	cellPath(grid, a[0], a[1], b[0], b[1], crossed);
	for (let i = 0; i + 1 < crossed.length; i += 2) {
		const [cx, cy] = cellCentre(grid, crossed[i], crossed[i + 1]);
		const cell = pens.cells.take();
		cell.x = cx - size / 2;
		cell.y = cy - size / 2;
		cell.size = size;
		cell.type = grid.type;
		cell.color = color;
		cell.alpha = CELL_ALPHA;
		out.cells.push(cell);
	}
	const start = cellCentre(grid, a[0], a[1]);
	const end = cellCentre(grid, b[0], b[1]);
	const feet = feetMoved(b[0] - a[0], b[1] - a[1], grid);
	rule(out, pens, start[0], start[1], end[0], end[1], distanceLabel(feet, grid), color);
}
export function lineRuler(
	out: Overlay, pens: Pens, grid: Grid,
	fromX: number, fromY: number, toX: number, toY: number, color: Rgb,
): void {
	if (isHex(grid)) {
		walkRuler(out, pens, grid, fromX, fromY, toX, toY, color);
		return;
	}
	const feet = feetBetween(toX - fromX, toY - fromY, grid);
	rule(out, pens, fromX, fromY, toX, toY, distanceLabel(feet, grid), color);
}
function rule(
	out: Overlay, pens: Pens,
	x0: number, y0: number, x1: number, y1: number, text: string, color: Rgb,
): void {
	const line = pens.segments.take();
	line.x0 = x0;
	line.y0 = y0;
	line.x1 = x1;
	line.y1 = y1;
	line.color = color;
	line.alpha = RULER_ALPHA;
	line.width = RULER_WIDTH;
	out.segments.push(line);
	const label = pens.labels.take();
	label.text = text;
	label.x = (x0 + x1) / 2;
	label.y = (y0 + y1) / 2;
	label.color[0] = color[0];
	label.color[1] = color[1];
	label.color[2] = color[2];
	label.alpha = 1;
	out.labels.push(label);
}
