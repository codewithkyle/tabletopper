import type { Grid } from "../protocol.ts";
export const PATH_CELLS_MAX = 512;
const SQRT3 = Math.sqrt(3);
const NUDGE = 1e-6;
export function roundAway(value: number): number {
	return (value < 0 ? -Math.round(-value) : Math.round(value)) + 0;
}
export function isHex(grid: Grid): boolean {
	return grid.type === "hexPointy" || grid.type === "hexFlat";
}
function sizeOf(grid: Grid): number {
	return Math.max(1, grid.cellSize);
}
function originOf(grid: Grid): [number, number] {
	const s = sizeOf(grid);
	return [grid.offsetX + s / 2, grid.offsetY + s / 2];
}
export function hexCentreAt(grid: Grid, q: number, r: number): [number, number] {
	const s = sizeOf(grid);
	const [ox, oy] = originOf(grid);
	if (grid.type === "hexFlat") {
		return [ox + s * (SQRT3 / 2) * q, oy + s * (q / 2 + r)];
	}
	return [ox + s * (q + r / 2), oy + s * (SQRT3 / 2) * r];
}
export function hexCentre(grid: Grid, q: number, r: number): [number, number] {
	const [x, y] = hexCentreAt(grid, q, r);
	return [roundAway(x), roundAway(y)];
}
export function hexFractional(grid: Grid, x: number, y: number): [number, number] {
	const s = sizeOf(grid);
	const [ox, oy] = originOf(grid);
	const px = x - ox;
	const py = y - oy;
	if (grid.type === "hexFlat") {
		return [(2 * px) / (s * SQRT3), py / s - px / (s * SQRT3)];
	}
	return [px / s - py / (s * SQRT3), (2 * py) / (s * SQRT3)];
}
export function hexRound(fq: number, fr: number): [number, number] {
	const x = fq;
	const z = fr;
	const y = -x - z;
	let rx = roundAway(x);
	let ry = roundAway(y);
	let rz = roundAway(z);
	const dx = Math.abs(rx - x);
	const dy = Math.abs(ry - y);
	const dz = Math.abs(rz - z);
	if (dx > dy && dx > dz) {
		rx = -ry - rz;
	} else if (dy > dz) {
		ry = -rx - rz;
	} else {
		rz = -rx - ry;
	}
	return [rx + 0, rz + 0];
}
export function hexAt(grid: Grid, x: number, y: number): [number, number] {
	const [fq, fr] = hexFractional(grid, x, y);
	return hexRound(fq, fr);
}
export function hexDistance(q0: number, r0: number, q1: number, r1: number): number {
	const dq = q1 - q0;
	const dr = r1 - r0;
	return (Math.abs(dq) + Math.abs(dq + dr) + Math.abs(dr)) / 2;
}
export function hexLine(q0: number, r0: number, q1: number, r1: number, out: number[]): number[] {
	out.length = 0;
	const steps = hexDistance(q0, r0, q1, r1);
	if (steps === 0) {
		out.push(q0, r0);
		return out;
	}
	for (let i = 0; i <= steps && out.length < PATH_CELLS_MAX * 2; i++) {
		const t = i / steps;
		const fq = q0 + NUDGE + (q1 - q0) * t;
		const fr = r0 + NUDGE + (r1 - r0) * t;
		const [q, r] = hexRound(fq, fr);
		out.push(q, r);
	}
	return out;
}
export function hexCorners(grid: Grid, q: number, r: number, out: number[]): number[] {
	out.length = 0;
	const [cx, cy] = hexCentreAt(grid, q, r);
	const radius = sizeOf(grid) / SQRT3;
	const start = grid.type === "hexFlat" ? 0 : -30;
	for (let i = 0; i < 6; i++) {
		const a = ((start + 60 * i) * Math.PI) / 180;
		out.push(roundAway(cx + radius * Math.cos(a)), roundAway(cy + radius * Math.sin(a)));
	}
	return out;
}
