import type { Pawn, Grid, Size } from "../protocol.ts";
import type { Point } from "./types.ts";
import { snapPoint } from "./grid.ts";
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
export function snapTo(grid: Grid, pawn: Sized, x: number, y: number, out: Point): Point {
	const [sx, sy] = snapPawn(grid, pawn, Math.round(x), Math.round(y));
	out.x = sx;
	out.y = sy;
	return out;
}
export function snapPawn(grid: Grid, pawn: Sized, x: number, y: number): [number, number] {
	if (pawn.kind === "object") {
		return [x, y];
	}
	const f = footprintOf(pawn.size);
	return snapPoint(grid, f, f, x, y);
}
