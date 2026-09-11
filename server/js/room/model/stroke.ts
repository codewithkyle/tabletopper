import type { Grid, Stroke, StrokeKind } from "../protocol.ts";
import { distanceLabel, feetBetween } from "./grid.ts";
export function strokeSegments(stroke: { kind: StrokeKind; points: readonly number[] }, out: number[]): number[] {
	const p = stroke.points;
	switch (stroke.kind) {
		case "free": {
			if (p.length < 2) {
				return out;
			}
			if (p.length === 2) {
				out.push(p[0], p[1], p[0], p[1]);
				return out;
			}
			for (let i = 0; i + 3 < p.length; i += 2) {
				out.push(p[i], p[i + 1], p[i + 2], p[i + 3]);
			}
			return out;
		}
		case "rect": {
			if (p.length < 4) {
				return out;
			}
			const x0 = Math.min(p[0], p[2]);
			const y0 = Math.min(p[1], p[3]);
			const x1 = Math.max(p[0], p[2]);
			const y1 = Math.max(p[1], p[3]);
			out.push(x0, y0, x1, y0);
			out.push(x1, y0, x1, y1);
			out.push(x1, y1, x0, y1);
			out.push(x0, y1, x0, y0);
			return out;
		}
		case "circle": {
			if (p.length < 4) {
				return out;
			}
			const r = Math.hypot(p[2] - p[0], p[3] - p[1]);
			if (r <= 0) {
				return out;
			}
			const n = circleSegments(r);
			let px = p[0] + r;
			let py = p[1];
			for (let i = 1; i <= n; i++) {
				const a = (i / n) * Math.PI * 2;
				const x = p[0] + Math.cos(a) * r;
				const y = p[1] + Math.sin(a) * r;
				out.push(px, py, x, y);
				px = x;
				py = y;
			}
			return out;
		}
		case "cone": {
			if (p.length < 4) {
				return out;
			}
			const corners = coneCorners(p[0], p[1], p[2], p[3], []);
			if (corners.length === 0) {
				return out;
			}
			out.push(corners[0], corners[1], corners[2], corners[3]);
			out.push(corners[2], corners[3], corners[4], corners[5]);
			out.push(corners[4], corners[5], corners[0], corners[1]);
			return out;
		}
		default:
			return out;
	}
}
export function coneCorners(
	ax: number, ay: number, bx: number, by: number, out: number[],
): number[] {
	out.length = 0;
	const dx = bx - ax;
	const dy = by - ay;
	const length = Math.hypot(dx, dy);
	if (length <= 0) {
		return out;
	}
	const half = length / 2;
	const nx = (-dy / length) * half;
	const ny = (dx / length) * half;
	out.push(ax, ay);
	out.push(bx + nx, by + ny);
	out.push(bx - nx, by - ny);
	return out;
}
export function circleSegments(radius: number): number {
	return Math.max(24, Math.min(512, Math.ceil((Math.PI * 2 * radius) / 4)));
}
const bounds = new WeakMap<Stroke, [number, number, number, number]>();
function boxOf(stroke: Stroke): [number, number, number, number] {
	const found = bounds.get(stroke);
	if (found) {
		return found;
	}
	let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
	for (let i = 0; i + 1 < stroke.points.length; i += 2) {
		const x = stroke.points[i];
		const y = stroke.points[i + 1];
		if (x < minX) minX = x;
		if (x > maxX) maxX = x;
		if (y < minY) minY = y;
		if (y > maxY) maxY = y;
	}
	const box: [number, number, number, number] = [minX, minY, maxX, maxY];
	bounds.set(stroke, box);
	return box;
}
export function strokeHit(stroke: Stroke, x: number, y: number, radius: number, out: number[]): boolean {
	const reach = radius + Math.max(stroke.width, 1) / 2;
	const [minX, minY, maxX, maxY] = boxOf(stroke);
	if (x < minX - reach || x > maxX + reach || y < minY - reach || y > maxY + reach) {
		return false;
	}
	out.length = 0;
	strokeSegments(stroke, out);
	const limit = reach * reach;
	for (let i = 0; i + 3 < out.length; i += 4) {
		if (segmentDistanceSquared(x, y, out[i], out[i + 1], out[i + 2], out[i + 3]) <= limit) {
			return true;
		}
	}
	return false;
}
function segmentDistanceSquared(
	px: number, py: number, x0: number, y0: number, x1: number, y1: number,
): number {
	const dx = x1 - x0;
	const dy = y1 - y0;
	const length = dx * dx + dy * dy;
	let t = 0;
	if (length > 0) {
		t = Math.min(1, Math.max(0, ((px - x0) * dx + (py - y0) * dy) / length));
	}
	const nx = px - (x0 + t * dx);
	const ny = py - (y0 + t * dy);
	return nx * nx + ny * ny;
}
export function measure(
	kind: StrokeKind, points: readonly number[], grid: Grid, color: string,
	add: (text: string, x: number, y: number, color: string) => void,
): void {
	if (points.length < 4) {
		return;
	}
	if (kind === "circle") {
		const dx = points[2] - points[0];
		const dy = points[3] - points[1];
		const r = Math.hypot(dx, dy);
		if (r <= 0) {
			return;
		}
		add(distanceLabel(feetBetween(dx, dy, grid)), points[0], points[1] - r, color);
		return;
	}
	if (kind === "cone") {
		const dx = points[2] - points[0];
		const dy = points[3] - points[1];
		if (dx === 0 && dy === 0) {
			return;
		}
		const corners = coneCorners(points[0], points[1], points[2], points[3], []);
		if (corners.length === 0) {
			return;
		}
		let minX = Infinity, maxX = -Infinity, minY = Infinity;
		for (let i = 0; i + 1 < corners.length; i += 2) {
			minX = Math.min(minX, corners[i]);
			maxX = Math.max(maxX, corners[i]);
			minY = Math.min(minY, corners[i + 1]);
		}
		add(distanceLabel(feetBetween(dx, dy, grid)), (minX + maxX) / 2, minY, color);
		return;
	}
	if (kind !== "rect") {
		return;
	}
	const x0 = Math.min(points[0], points[2]);
	const y0 = Math.min(points[1], points[3]);
	const x1 = Math.max(points[0], points[2]);
	const y1 = Math.max(points[1], points[3]);
	if (x1 > x0) {
		add(distanceLabel(feetBetween(x1 - x0, 0, grid)), (x0 + x1) / 2, y0, color);
	}
	if (y1 > y0) {
		add(distanceLabel(feetBetween(0, y1 - y0, grid)), x0, (y0 + y1) / 2, color);
	}
}
