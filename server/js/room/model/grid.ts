import type { Grid } from "../protocol.ts";
const PATH_CELLS_MAX = 512;
function round(value: number): number {
	return value < 0 ? -Math.round(-value) : Math.round(value);
}
export function snapAxis(cell: number, offset: number, footprint: number, mode: Grid["snap"], value: number): number {
	if (mode === "off" || cell < 1) {
		return value;
	}
	let step = cell;
	let half = 0;
	if (mode === "halfCells") {
		step = cell / 2;
	} else if (footprint % 2 === 1) {
		half = cell / 2;
	}
	const k = round((value - offset - half) / step);
	return round(k * step + offset + half);
}
export function snapPoint(grid: Grid, footprintW: number, footprintH: number, x: number, y: number): [number, number] {
	return [
		snapAxis(grid.cellSize, grid.offsetX, footprintW, grid.snap, x),
		snapAxis(grid.cellSize, grid.offsetY, footprintH, grid.snap, y),
	];
}
export function cellAt(grid: Grid, x: number, y: number): [number, number] {
	const cell = Math.max(1, grid.cellSize);
	return [Math.floor((x - grid.offsetX) / cell), Math.floor((y - grid.offsetY) / cell)];
}
export function cellCentre(grid: Grid, cx: number, cy: number): [number, number] {
	const cell = Math.max(1, grid.cellSize);
	return [grid.offsetX + (cx + 0.5) * cell, grid.offsetY + (cy + 0.5) * cell];
}
export function supercover(x0: number, y0: number, x1: number, y1: number, out: number[]): number[] {
	out.length = 0;
	out.push(x0, y0);
	if (x0 === x1 && y0 === y1) {
		return out;
	}
	const dx = x1 - x0;
	const dy = y1 - y0;
	const stepX = Math.sign(dx);
	const stepY = Math.sign(dy);
	const deltaX = dx === 0 ? Infinity : Math.abs(1 / dx);
	const deltaY = dy === 0 ? Infinity : Math.abs(1 / dy);
	let tX = dx === 0 ? Infinity : deltaX / 2;
	let tY = dy === 0 ? Infinity : deltaY / 2;
	let x = x0;
	let y = y0;
	const epsilon = 1e-9;
	while ((x !== x1 || y !== y1) && out.length < PATH_CELLS_MAX * 2) {
		if (tX < tY - epsilon) {
			x += stepX;
			tX += deltaX;
		} else if (tY < tX - epsilon) {
			y += stepY;
			tY += deltaY;
		} else {
			x += stepX;
			y += stepY;
			tX += deltaX;
			tY += deltaY;
		}
		out.push(x, y);
	}
	return out;
}
export function cellsMoved(dx: number, dy: number, diagonals: Grid["diagonals"]): number {
	const across = Math.abs(dx);
	const down = Math.abs(dy);
	const straight = Math.max(across, down) - Math.min(across, down);
	const diagonal = Math.min(across, down);
	if (diagonals === "alternating") {
		return straight + diagonal + Math.floor(diagonal / 2);
	}
	return straight + diagonal;
}
export function feetMoved(dx: number, dy: number, grid: Grid): number {
	return cellsMoved(dx, dy, grid.diagonals) * Math.max(0, grid.feetPerCell);
}
export function feetBetween(dx: number, dy: number, grid: Grid): number {
	const cell = Math.max(1, grid.cellSize);
	return (Math.hypot(dx, dy) / cell) * Math.max(0, grid.feetPerCell);
}
export function distanceLabel(feet: number): string {
	return `${Math.round(feet)} ft.`;
}
