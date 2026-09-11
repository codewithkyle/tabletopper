import type { Grid, Pawn, Size } from "../protocol.ts";
import type { Point } from "./camera.ts";
export const PATH_CELLS_MAX = 512;
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
export type Sized = Pick<Pawn, "kind" | "size" | "width" | "height" | "rotation">;
export type Placed = Sized & Pick<Pawn, "x" | "y">;
export const TINY_SCALE = 0.5;
export function footprintOf(size: Size): number {
	switch (size) {
		case "large":
			return 2;
		case "huge":
			return 3;
		case "gargantuan":
			return 4;
		default:
			return 1;
	}
}
export function pawnExtents(pawn: Sized, cellSize: number): [number, number] {
	if (pawn.kind === "object") {
		return [Math.max(1, pawn.width) / 2, Math.max(1, pawn.height) / 2];
	}
	const cell = Math.max(1, cellSize);
	const f = footprintOf(pawn.size);
	const scale = pawn.size === "tiny" ? TINY_SCALE : 1;
	return [(f * cell * scale) / 2, (f * cell * scale) / 2];
}
export function boundsOf(pawn: Sized, cellSize: number): [number, number] {
	const [halfW, halfH] = pawnExtents(pawn, cellSize);
	if (pawn.rotation === 0 || pawn.kind !== "object") {
		return [halfW, halfH];
	}
	const a = radians(pawn.rotation);
	const cos = Math.abs(Math.cos(a));
	const sin = Math.abs(Math.sin(a));
	return [halfW * cos + halfH * sin, halfW * sin + halfH * cos];
}
export function radians(degrees: number): number {
	return (degrees * Math.PI) / 180;
}
export function unrotate(degrees: number, dx: number, dy: number, out: Point): Point {
	if (degrees === 0) {
		out.x = dx;
		out.y = dy;
		return out;
	}
	const a = radians(degrees);
	const cos = Math.cos(a);
	const sin = Math.sin(a);
	out.x = dx * cos + dy * sin;
	out.y = dy * cos - dx * sin;
	return out;
}
export function spin(degrees: number, lx: number, ly: number, out: Point): Point {
	if (degrees === 0) {
		out.x = lx;
		out.y = ly;
		return out;
	}
	const a = radians(degrees);
	const cos = Math.cos(a);
	const sin = Math.sin(a);
	out.x = lx * cos - ly * sin;
	out.y = lx * sin + ly * cos;
	return out;
}
const local: Point = { x: 0, y: 0 };
export function containsPoint(pawn: Placed, x: number, y: number, cellSize: number): boolean {
	const [halfW, halfH] = pawnExtents(pawn, cellSize);
	if (pawn.kind !== "object") {
		const dx = x - pawn.x;
		const dy = y - pawn.y;
		return dx * dx + dy * dy <= halfW * halfW;
	}
	unrotate(pawn.rotation, x - pawn.x, y - pawn.y, local);
	return Math.abs(local.x) <= halfW && Math.abs(local.y) <= halfH;
}
export function snapsToGrid(pawn: Sized, grid: Grid): boolean {
	return grid.snap !== "off" && pawn.kind !== "object";
}
export function snapPawn(grid: Grid, pawn: Sized, x: number, y: number): [number, number] {
	if (pawn.kind === "object") {
		return [x, y];
	}
	const f = footprintOf(pawn.size);
	return snapPoint(grid, f, f, x, y);
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
