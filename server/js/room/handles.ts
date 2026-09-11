import type { Placed } from "./render/path.ts";
import type { Point } from "./render/camera.ts";
import { pawnExtents, spin, unrotate } from "./render/path.ts";
export const HANDLE_HALF = 4;
export const HANDLE_GRAB = 10;
export const SPIN_GAP = 26;
export const SPIN_STEP = 15;
export const OBJECT_PIXELS_MAX = 8_192;
export interface Handle {
	lx: number;
	ly: number;
	turns: boolean;
	x: number;
	y: number;
	rotation: number;
}
const OFFSETS: readonly (readonly [number, number])[] = [
	[-1, -1], [1, -1], [1, 1], [-1, 1],
	[0, -1], [1, 0], [0, 1], [-1, 0],
];
const local: Point = { x: 0, y: 0 };
export function handlesFor(
	pawn: Placed,
	cellSize: number,
	mapPerPixel: number,
	out: Handle[],
): Handle[] {
	if (pawn.kind !== "object") {
		out.length = 0;
		return out;
	}
	const [halfW, halfH] = pawnExtents(pawn, cellSize);
	let count = 0;
	const put = (lx: number, ly: number, ox: number, oy: number, turns: boolean): void => {
		spin(pawn.rotation, ox, oy, local);
		const slot = out[count] ?? (out[count] = { lx: 0, ly: 0, turns: false, x: 0, y: 0, rotation: 0 });
		slot.lx = lx;
		slot.ly = ly;
		slot.turns = turns;
		slot.x = pawn.x + local.x;
		slot.y = pawn.y + local.y;
		slot.rotation = pawn.rotation;
		count++;
	};
	for (const [lx, ly] of OFFSETS) {
		put(lx, ly, lx * halfW, ly * halfH, false);
	}
	put(0, 1, 0, halfH + SPIN_GAP * mapPerPixel, true);
	out.length = count;
	return out;
}
export function handleAt(
	handles: readonly Handle[],
	x: number,
	y: number,
	mapPerPixel: number,
): Handle | null {
	const reach = HANDLE_GRAB * mapPerPixel;
	const limit = reach * reach;
	let best: Handle | null = null;
	let nearest = Infinity;
	for (const handle of handles) {
		const dx = x - handle.x;
		const dy = y - handle.y;
		const distance = dx * dx + dy * dy;
		if (distance <= limit && distance < nearest) {
			best = handle;
			nearest = distance;
		}
	}
	return best;
}
export function resized(
	pawn: Placed,
	handle: Handle,
	x: number,
	y: number,
	cellSize: number,
	lockAspect: boolean,
): [number, number] {
	const [halfW, halfH] = pawnExtents(pawn, cellSize);
	const fromW = halfW * 2;
	const fromH = halfH * 2;
	unrotate(pawn.rotation, x - pawn.x, y - pawn.y, local);
	let width = fromW;
	let height = fromH;
	if (handle.lx !== 0) {
		width = clamp(Math.round(Math.abs(local.x) * 2));
	}
	if (handle.ly !== 0) {
		height = clamp(Math.round(Math.abs(local.y) * 2));
	}
	if (lockAspect && handle.lx !== 0 && handle.ly !== 0) {
		const factor = Math.max(width / fromW, height / fromH);
		width = clamp(Math.round(fromW * factor));
		height = clamp(Math.round(fromH * factor));
	}
	return [width, height];
}
export function turned(pawn: Placed, x: number, y: number, step: number): number {
	const degrees = (Math.atan2(y - pawn.y, x - pawn.x) * 180) / Math.PI - 90;
	const stepped = step > 0 ? Math.round(degrees / step) * step : Math.round(degrees);
	return ((stepped % 360) + 360) % 360;
}
function clamp(pixels: number): number {
	return Math.min(Math.max(pixels, 1), OBJECT_PIXELS_MAX);
}
