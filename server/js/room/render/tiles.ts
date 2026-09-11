import type { MapRef } from "../protocol.ts";
import type { Rect } from "../model/types.ts";
import { levelPixels, levelTiles } from "./pyramid.ts";
export const LAYERS = 96;
export function levelFor(zoom: number, dpr: number, maxZoom: number): number {
	const scale = zoom * dpr;
	if (!(scale > 0)) {
		return maxZoom;
	}
	return Math.min(Math.max(Math.round(Math.log2(1 / scale)), 0), maxZoom);
}
export function levelScale(size: number, z: number): number {
	const pixels = levelPixels(size, z);
	return pixels > 0 ? size / pixels : 1;
}
export interface TileRange {
	x0: number;
	y0: number;
	x1: number;
	y1: number;
}
export function newRange(): TileRange {
	return { x0: 0, y0: 0, x1: -1, y1: -1 };
}
export function rangeCount(r: TileRange): number {
	return Math.max(0, r.x1 - r.x0 + 1) * Math.max(0, r.y1 - r.y0 + 1);
}
export function visibleRange(map: MapRef, z: number, view: Rect, out: TileRange): TileRange {
	out.x0 = 0;
	out.y0 = 0;
	out.x1 = -1;
	out.y1 = -1;
	const across = levelTiles(map.width, map.tileSize, z);
	const down = levelTiles(map.height, map.tileSize, z);
	if (across < 1 || down < 1) {
		return out;
	}
	const spanX = map.tileSize * levelScale(map.width, z);
	const spanY = map.tileSize * levelScale(map.height, z);
	const left = Math.max(view.x1, 0);
	const right = Math.min(view.x2, map.width);
	const top = Math.max(view.y1, 0);
	const bottom = Math.min(view.y2, map.height);
	if (right <= left || bottom <= top) {
		return out;
	}
	out.x0 = clamp(Math.floor(left / spanX), 0, across - 1);
	out.x1 = clamp(Math.ceil(right / spanX) - 1, 0, across - 1);
	out.y0 = clamp(Math.floor(top / spanY), 0, down - 1);
	out.y1 = clamp(Math.ceil(bottom / spanY) - 1, 0, down - 1);
	return out;
}
export function tileRect(map: MapRef, z: number, x: number, y: number, out: Rect): Rect {
	const scaleX = levelScale(map.width, z);
	const scaleY = levelScale(map.height, z);
	const pixelsX = levelPixels(map.width, z);
	const pixelsY = levelPixels(map.height, z);
	out.x1 = x * map.tileSize * scaleX;
	out.y1 = y * map.tileSize * scaleY;
	out.x2 = Math.min((x + 1) * map.tileSize, pixelsX) * scaleX;
	out.y2 = Math.min((y + 1) * map.tileSize, pixelsY) * scaleY;
	return out;
}
export function uvFor(map: MapRef, z: number, x: number, y: number, rect: Rect, out: Rect): Rect {
	const scaleX = levelScale(map.width, z);
	const scaleY = levelScale(map.height, z);
	out.x1 = (rect.x1 / scaleX - x * map.tileSize) / map.tileSize;
	out.y1 = (rect.y1 / scaleY - y * map.tileSize) / map.tileSize;
	out.x2 = (rect.x2 / scaleX - x * map.tileSize) / map.tileSize;
	out.y2 = (rect.y2 / scaleY - y * map.tileSize) / map.tileSize;
	return out;
}
export function tileKey(map: MapRef, z: number, x: number, y: number): string {
	return `${map.assetId}:${map.gen}:${z}:${x}:${y}`;
}
export function tileURL(map: MapRef, z: number, x: number, y: number): string {
	return `/assets/maps/${map.assetId}/tiles/${map.gen}/${z}/${x}_${y}.webp`;
}
function clamp(v: number, low: number, high: number): number {
	return Math.min(Math.max(v, low), high);
}
