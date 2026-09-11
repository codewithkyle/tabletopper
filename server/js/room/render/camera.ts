import type { Point, Rect } from "../model/types.ts";
export interface Camera {
	x: number;
	y: number;
	zoom: number;
}
export interface Viewport {
	width: number;
	height: number;
}
export const ZOOM_MIN = 0.05;
export const ZOOM_MAX = 4;
const FIT_MARGIN = 0.9;
export function newCamera(): Camera {
	return { x: 0, y: 0, zoom: 1 };
}
export function clampZoom(zoom: number): number {
	return Math.min(Math.max(zoom, ZOOM_MIN), ZOOM_MAX);
}
export function worldToScreen(cam: Camera, vp: Viewport, wx: number, wy: number, out: Point): Point {
	out.x = (wx - cam.x) * cam.zoom + vp.width / 2;
	out.y = (wy - cam.y) * cam.zoom + vp.height / 2;
	return out;
}
export function screenToWorld(cam: Camera, vp: Viewport, sx: number, sy: number, out: Point): Point {
	out.x = (sx - vp.width / 2) / cam.zoom + cam.x;
	out.y = (sy - vp.height / 2) / cam.zoom + cam.y;
	return out;
}
export function visibleRect(cam: Camera, vp: Viewport, out: Rect): Rect {
	const halfW = vp.width / 2 / cam.zoom;
	const halfH = vp.height / 2 / cam.zoom;
	out.x1 = cam.x - halfW;
	out.y1 = cam.y - halfH;
	out.x2 = cam.x + halfW;
	out.y2 = cam.y + halfH;
	return out;
}
export function panBy(cam: Camera, dx: number, dy: number): void {
	cam.x -= dx / cam.zoom;
	cam.y -= dy / cam.zoom;
}
const anchorBefore: Point = { x: 0, y: 0 };
const anchorAfter: Point = { x: 0, y: 0 };
export function zoomAt(cam: Camera, vp: Viewport, sx: number, sy: number, multiplier: number): void {
	const zoom = clampZoom(cam.zoom * multiplier);
	if (zoom === cam.zoom) {
		return;
	}
	screenToWorld(cam, vp, sx, sy, anchorBefore);
	cam.zoom = zoom;
	screenToWorld(cam, vp, sx, sy, anchorAfter);
	cam.x += anchorBefore.x - anchorAfter.x;
	cam.y += anchorBefore.y - anchorAfter.y;
}
export function zoomTo(cam: Camera, vp: Viewport, zoom: number): void {
	zoomAt(cam, vp, vp.width / 2, vp.height / 2, zoom / cam.zoom);
}
export function fit(cam: Camera, vp: Viewport, width: number, height: number): void {
	if (width < 1 || height < 1 || vp.width < 1 || vp.height < 1) {
		return;
	}
	cam.zoom = clampZoom(Math.min(vp.width / width, vp.height / height) * FIT_MARGIN);
	cam.x = width / 2;
	cam.y = height / 2;
}
const FOCUS_MARGIN = 0.8;
export function focusTarget(cam: Camera, vp: Viewport, rect: Rect, out: Camera): Camera {
	const width = Math.max(rect.x2 - rect.x1, 1);
	const height = Math.max(rect.y2 - rect.y1, 1);
	const holds = clampZoom(Math.min(vp.width / width, vp.height / height) * FOCUS_MARGIN);
	out.x = (rect.x1 + rect.x2) / 2;
	out.y = (rect.y1 + rect.y2) / 2;
	out.zoom = Math.min(cam.zoom, holds);
	return out;
}
export function clampToMap(cam: Camera, vp: Viewport, width: number, height: number): void {
	if (width < 1 || height < 1) {
		return;
	}
	cam.x = clampAxis(cam.x, vp.width / cam.zoom, width);
	cam.y = clampAxis(cam.y, vp.height / cam.zoom, height);
}
function clampAxis(centre: number, viewport: number, size: number): number {
	const keep = Math.min(viewport, size) / 2;
	return Math.min(Math.max(centre, keep - viewport / 2), size - keep + viewport / 2);
}
export function clipMatrix(cam: Camera, deviceWidth: number, deviceHeight: number, dpr: number, out: Float32Array): Float32Array {
	const sx = (2 * cam.zoom * dpr) / Math.max(deviceWidth, 1);
	const sy = (-2 * cam.zoom * dpr) / Math.max(deviceHeight, 1);
	out[0] = sx;
	out[1] = 0;
	out[2] = 0;
	out[3] = 0;
	out[4] = sy;
	out[5] = 0;
	out[6] = -sx * cam.x;
	out[7] = -sy * cam.y;
	out[8] = 1;
	return out;
}
export function inverseClipMatrix(cam: Camera, deviceWidth: number, deviceHeight: number, dpr: number, out: Float32Array): Float32Array {
	const sx = (2 * cam.zoom * dpr) / Math.max(deviceWidth, 1);
	const sy = (-2 * cam.zoom * dpr) / Math.max(deviceHeight, 1);
	out[0] = 1 / sx;
	out[1] = 0;
	out[2] = 0;
	out[3] = 0;
	out[4] = 1 / sy;
	out[5] = 0;
	out[6] = cam.x;
	out[7] = cam.y;
	out[8] = 1;
	return out;
}
