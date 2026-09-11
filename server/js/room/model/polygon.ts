import type { FogShape, Layer, Pawn, Role } from "../protocol.ts";
const EARS_MAX = 100_000;
export function rectTriangles(points: readonly number[], out: number[]): number[] {
	out.length = 0;
	if (points.length < 4) {
		return out;
	}
	const x0 = Math.min(points[0], points[2]);
	const y0 = Math.min(points[1], points[3]);
	const x1 = Math.max(points[0], points[2]);
	const y1 = Math.max(points[1], points[3]);
	out.push(x0, y0, x1, y0, x1, y1);
	out.push(x0, y0, x1, y1, x0, y1);
	return out;
}
export function triangulate(ring: readonly number[], out: number[]): number[] {
	out.length = 0;
	const n = ring.length >> 1;
	if (n < 3) {
		return out;
	}
	const left: number[] = [];
	for (let i = 0; i < n; i++) {
		left.push(i);
	}
	const clockwise = signedArea(ring) < 0;
	let guard = 0;
	let at = 0;
	while (left.length > 3 && guard++ < EARS_MAX) {
		const count = left.length;
		const prev = left[(at + count - 1) % count];
		const here = left[at % count];
		const next = left[(at + 1) % count];
		if (isEar(ring, left, prev, here, next, clockwise)) {
			out.push(ring[prev * 2], ring[prev * 2 + 1]);
			out.push(ring[here * 2], ring[here * 2 + 1]);
			out.push(ring[next * 2], ring[next * 2 + 1]);
			left.splice(at % count, 1);
			at = 0;
			continue;
		}
		at++;
		if (at > count) {
			break;
		}
	}
	for (let i = 1; i + 1 < left.length; i++) {
		out.push(ring[left[0] * 2], ring[left[0] * 2 + 1]);
		out.push(ring[left[i] * 2], ring[left[i] * 2 + 1]);
		out.push(ring[left[i + 1] * 2], ring[left[i + 1] * 2 + 1]);
	}
	return out;
}
function signedArea(ring: readonly number[]): number {
	let sum = 0;
	for (let i = 0, n = ring.length >> 1; i < n; i++) {
		const j = (i + 1) % n;
		sum += ring[i * 2] * ring[j * 2 + 1] - ring[j * 2] * ring[i * 2 + 1];
	}
	return sum;
}
function isEar(
	ring: readonly number[], left: readonly number[],
	prev: number, here: number, next: number, clockwise: boolean,
): boolean {
	const ax = ring[prev * 2], ay = ring[prev * 2 + 1];
	const bx = ring[here * 2], by = ring[here * 2 + 1];
	const cx = ring[next * 2], cy = ring[next * 2 + 1];
	const turn = cross(ax, ay, bx, by, cx, cy);
	if (clockwise ? turn >= 0 : turn <= 0) {
		return false;
	}
	for (const i of left) {
		if (i === prev || i === here || i === next) {
			continue;
		}
		if (inTriangle(ring[i * 2], ring[i * 2 + 1], ax, ay, bx, by, cx, cy)) {
			return false;
		}
	}
	return true;
}
function cross(ax: number, ay: number, bx: number, by: number, cx: number, cy: number): number {
	return (bx - ax) * (cy - ay) - (by - ay) * (cx - ax);
}
function inTriangle(
	px: number, py: number,
	ax: number, ay: number, bx: number, by: number, cx: number, cy: number,
): boolean {
	const d1 = cross(ax, ay, bx, by, px, py);
	const d2 = cross(bx, by, cx, cy, px, py);
	const d3 = cross(cx, cy, ax, ay, px, py);
	return !((d1 < 0 || d2 < 0 || d3 < 0) && (d1 > 0 || d2 > 0 || d3 > 0));
}
const bounds = new WeakMap<FogShape, [number, number, number, number]>();
function boxOf(shape: FogShape): [number, number, number, number] {
	const found = bounds.get(shape);
	if (found) {
		return found;
	}
	let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
	for (let i = 0; i + 1 < shape.points.length; i += 2) {
		const x = shape.points[i];
		const y = shape.points[i + 1];
		if (x < minX) minX = x;
		if (x > maxX) maxX = x;
		if (y < minY) minY = y;
		if (y > maxY) maxY = y;
	}
	const box: [number, number, number, number] = [minX, minY, maxX, maxY];
	bounds.set(shape, box);
	return box;
}
export function insideShape(shape: FogShape, x: number, y: number): boolean {
	const p = shape.points;
	const [minX, minY, maxX, maxY] = boxOf(shape);
	if (x < minX || x > maxX || y < minY || y > maxY) {
		return false;
	}
	if (shape.kind === "rect") {
		return p.length >= 4;
	}
	let inside = false;
	for (let i = 0, n = p.length >> 1, j = n - 1; i < n; j = i++) {
		const xi = p[i * 2], yi = p[i * 2 + 1];
		const xj = p[j * 2], yj = p[j * 2 + 1];
		if ((yi > y) !== (yj > y) && x < ((xj - xi) * (y - yi)) / (yj - yi) + xi) {
			inside = !inside;
		}
	}
	return inside;
}
export function coveredBy(
	shapes: readonly FogShape[], layerID: string, prefill: boolean, x: number, y: number,
): boolean {
	for (let i = shapes.length - 1; i >= 0; i--) {
		const shape = shapes[i];
		if (shape.layerId !== layerID) {
			continue;
		}
		if (insideShape(shape, x, y)) {
			return shape.mode === "hide";
		}
	}
	return prefill;
}
export function concealed(
	pawn: Pick<Pawn, "x" | "y" | "ownerId">,
	shapes: readonly FogShape[],
	viewed: Layer | null,
	role: Role,
	user: string,
): boolean {
	if (role === "gm" || (pawn.ownerId !== null && pawn.ownerId === user)) {
		return false;
	}
	if (!viewed || !viewed.fogEnabled) {
		return false;
	}
	return coveredBy(shapes, viewed.id, viewed.fogPrefill, pawn.x, pawn.y);
}
export function maskRect(
	map: { width: number; height: number } | null,
	shapes: readonly FogShape[], layerID: string, cell: number,
	out: MaskRect,
): MaskRect | null {
	let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
	if (map && map.width > 0 && map.height > 0) {
		minX = 0;
		minY = 0;
		maxX = map.width;
		maxY = map.height;
	}
	for (const shape of shapes) {
		if (shape.layerId !== layerID) {
			continue;
		}
		for (let i = 0; i + 1 < shape.points.length; i += 2) {
			const x = shape.points[i];
			const y = shape.points[i + 1];
			if (x < minX) minX = x;
			if (x > maxX) maxX = x;
			if (y < minY) minY = y;
			if (y > maxY) maxY = y;
		}
	}
	if (!(maxX > minX) || !(maxY > minY)) {
		return null;
	}
	const pad = Math.max(cell, 1);
	out.x = minX - pad;
	out.y = minY - pad;
	out.width = maxX - minX + pad * 2;
	out.height = maxY - minY + pad * 2;
	return out;
}
export interface MaskRect {
	x: number;
	y: number;
	width: number;
	height: number;
}
