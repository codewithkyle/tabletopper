import type { Grid, MapRef } from "../protocol.ts";
import { cellAt, cellCentre } from "./grid.ts";
const PAD = 1;
const CELLS_MAX = 250_000;
export interface Numbering {
	of(q: number, r: number): number | null;
	each(x1: number, y1: number, x2: number, y2: number, visit: (q: number, r: number, n: number) => void): void;
	total(): number;
}
export function offsetOf(grid: Grid, q: number, r: number): [number, number] {
	if (grid.type === "hexPointy") {
		return [q + (r - (r & 1)) / 2, r];
	}
	if (grid.type === "hexFlat") {
		return [q, r + (q - (q & 1)) / 2];
	}
	return [q, r];
}
export function axialOf(grid: Grid, col: number, row: number): [number, number] {
	if (grid.type === "hexPointy") {
		return [col - (row - (row & 1)) / 2, row];
	}
	if (grid.type === "hexFlat") {
		return [col, row - (col - (col & 1)) / 2];
	}
	return [col, row];
}
function span(grid: Grid, x1: number, y1: number, x2: number, y2: number): [number, number, number, number] {
	let colMin = Infinity;
	let colMax = -Infinity;
	let rowMin = Infinity;
	let rowMax = -Infinity;
	for (const [x, y] of [[x1, y1], [x2, y1], [x1, y2], [x2, y2]]) {
		const [q, r] = cellAt(grid, x, y);
		const [col, row] = offsetOf(grid, q, r);
		colMin = Math.min(colMin, col);
		colMax = Math.max(colMax, col);
		rowMin = Math.min(rowMin, row);
		rowMax = Math.max(rowMax, row);
	}
	return [colMin - PAD, colMax + PAD, rowMin - PAD, rowMax + PAD];
}
const empty: Numbering = { of: () => null, each: () => {}, total: () => 0 };
export function newNumbering(grid: Grid, width: number, height: number): Numbering {
	if (width <= 0 || height <= 0) {
		return empty;
	}
	const [colMin, colMax, rowMin, rowMax] = span(grid, 0, 0, width, height);
	const cols = colMax - colMin + 1;
	const rows = rowMax - rowMin + 1;
	if (cols * rows > CELLS_MAX) {
		return empty;
	}
	const numbers = new Int32Array(cols * rows);
	let count = 0;
	for (let c = 0; c < cols; c++) {
		for (let w = 0; w < rows; w++) {
			const [q, r] = axialOf(grid, colMin + c, rowMin + w);
			const [x, y] = cellCentre(grid, q, r);
			if (x < 0 || x > width || y < 0 || y > height) {
				continue;
			}
			numbers[c * rows + w] = ++count;
		}
	}
	function at(col: number, row: number): number | null {
		const c = col - colMin;
		const w = row - rowMin;
		if (c < 0 || c >= cols || w < 0 || w >= rows) {
			return null;
		}
		return numbers[c * rows + w] || null;
	}
	return {
		of(q, r) {
			const [col, row] = offsetOf(grid, q, r);
			return at(col, row);
		},
		each(x1, y1, x2, y2, visit) {
			if (count === 0) {
				return;
			}
			const [fromCol, toCol, fromRow, toRow] = span(grid, x1, y1, x2, y2);
			for (let col = Math.max(fromCol, colMin); col <= Math.min(toCol, colMax); col++) {
				for (let row = Math.max(fromRow, rowMin); row <= Math.min(toRow, rowMax); row++) {
					const n = at(col, row);
					if (n === null) {
						continue;
					}
					const [q, r] = axialOf(grid, col, row);
					visit(q, r, n);
				}
			}
		},
		total: () => count,
	};
}
let held: { key: string; numbering: Numbering } | null = null;
export function numberingFor(grid: Grid, map: MapRef | null | undefined): Numbering | null {
	if (!grid.numbered || !map) {
		return null;
	}
	const key = [grid.type, grid.cellSize, grid.offsetX, grid.offsetY, map.width, map.height].join(":");
	if (held?.key !== key) {
		held = { key, numbering: newNumbering(grid, map.width, map.height) };
	}
	return held.numbering;
}
export function cellName(grid: Grid, map: MapRef | null | undefined, q: number, r: number): string {
	const n = numberingFor(grid, map)?.of(q, r);
	return n ? String(n) : `${q}, ${r}`;
}
