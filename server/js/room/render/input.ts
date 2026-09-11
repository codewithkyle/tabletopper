import type { Camera, Viewport } from "./camera.ts";
import type { Point } from "../model/types.ts";
import { panBy, zoomAt } from "./camera.ts";
const ZOOM_STEP_MAX = Math.log(1.25);
const WHEEL_SCALE = 0.0015;
const PINCH_SCALE = 0.01;
const PIXELS_PER_LINE = 16;
const PIXELS_PER_PAGE = 400;
export interface Modifiers {
	shift: boolean;
	alt: boolean;
}
export interface Tool {
	press(map: Point, screen: Point, mods: Modifiers): boolean;
	drag(map: Point, screen: Point, mods: Modifiers): void;
	release(map: Point, screen: Point, mods: Modifiers): void;
	cancel(): void;
	secondary(map: Point, screen: Point): void;
	hover(map: Point | null): void;
	active(): boolean;
}
export interface Pending {
	panX: number;
	panY: number;
	zoom: number;
	zoomX: number;
	zoomY: number;
}
export function newPending(): Pending {
	return { panX: 0, panY: 0, zoom: 1, zoomX: 0, zoomY: 0 };
}
export function apply(pending: Pending, cam: Camera, vp: Viewport): boolean {
	const moved = pending.panX !== 0 || pending.panY !== 0 || pending.zoom !== 1;
	if (!moved) {
		return false;
	}
	if (pending.panX !== 0 || pending.panY !== 0) {
		panBy(cam, pending.panX, pending.panY);
	}
	if (pending.zoom !== 1) {
		zoomAt(cam, vp, pending.zoomX, pending.zoomY, pending.zoom);
	}
	pending.panX = 0;
	pending.panY = 0;
	pending.zoom = 1;
	return true;
}
export function wheelMultiplier(deltaY: number, deltaMode: number, pinch: boolean): number {
	let pixels = deltaY;
	if (deltaMode === 1) {
		pixels *= PIXELS_PER_LINE;
	} else if (deltaMode === 2) {
		pixels *= PIXELS_PER_PAGE;
	}
	const step = -pixels * (pinch ? PINCH_SCALE : WHEEL_SCALE);
	return Math.exp(Math.min(Math.max(step, -ZOOM_STEP_MAX), ZOOM_STEP_MAX));
}
interface Tracked {
	x: number;
	y: number;
}
export interface Input {
	pending: Pending;
	dragging(): boolean;
	stop(): void;
}
export type Project = (x: number, y: number, out: Point) => Point;
const PAN_BUTTONS = new Set([0, 1]);
export function wireInput(
	canvas: HTMLCanvasElement,
	invalidate: () => void,
	project: Project | null = null,
	tool: Tool | null = null,
): Input {
	const pending = newPending();
	const pointers = new Map<number, Tracked>();
	let claimed: number | null = null;
	const map: Point = { x: 0, y: 0 };
	const screen: Point = { x: 0, y: 0 };
	function toMap(tracked: Tracked): Point {
		screen.x = tracked.x;
		screen.y = tracked.y;
		if (project) {
			project(tracked.x, tracked.y, map);
		} else {
			map.x = tracked.x;
			map.y = tracked.y;
		}
		return map;
	}
	function mods(e: PointerEvent): Modifiers {
		return { shift: e.shiftKey, alt: e.altKey };
	}
	let bounds = { left: 0, top: 0 };
	function measure(): void {
		const rect = canvas.getBoundingClientRect();
		bounds = { left: rect.left, top: rect.top };
	}
	function at(e: MouseEvent): Tracked {
		return { x: e.clientX - bounds.left, y: e.clientY - bounds.top };
	}
	function onPointerDown(e: PointerEvent): void {
		if (!PAN_BUTTONS.has(e.button)) {
			return;
		}
		e.preventDefault();
		measure();
		canvas.setPointerCapture(e.pointerId);
		const tracked = at(e);
		if (e.button === 0 && tool && claimed === null) {
			const took = tool.press(toMap(tracked), screen, mods(e));
			claimed = e.pointerId;
			invalidate();
			if (took) {
				return;
			}
		}
		pointers.set(e.pointerId, tracked);
	}
	function onPointerMove(e: PointerEvent): void {
		const now = at(e);
		if (tool) {
			if (e.pointerId === claimed) {
				tool.drag(toMap(now), screen, mods(e));
				invalidate();
			} else if (claimed === null && pointers.size === 0) {
				tool.hover(toMap(now));
				invalidate();
			}
		}
		const tracked = pointers.get(e.pointerId);
		if (!tracked) {
			return;
		}
		if (pointers.size === 1) {
			pending.panX += now.x - tracked.x;
			pending.panY += now.y - tracked.y;
			tracked.x = now.x;
			tracked.y = now.y;
			invalidate();
			return;
		}
		const [a, b] = firstTwo(pointers);
		if (!a || !b) {
			return;
		}
		const beforeDistance = distance(a, b);
		const beforeX = (a.x + b.x) / 2;
		const beforeY = (a.y + b.y) / 2;
		tracked.x = now.x;
		tracked.y = now.y;
		const afterDistance = distance(a, b);
		const afterX = (a.x + b.x) / 2;
		const afterY = (a.y + b.y) / 2;
		pending.panX += afterX - beforeX;
		pending.panY += afterY - beforeY;
		if (beforeDistance > 1 && afterDistance > 1) {
			pending.zoom *= afterDistance / beforeDistance;
			pending.zoomX = afterX;
			pending.zoomY = afterY;
		}
		invalidate();
	}
	function onPointerUp(e: PointerEvent): void {
		if (canvas.hasPointerCapture(e.pointerId)) {
			canvas.releasePointerCapture(e.pointerId);
		}
		pointers.delete(e.pointerId);
		if (e.pointerId !== claimed || !tool) {
			return;
		}
		claimed = null;
		if (e.type === "pointercancel") {
			tool.cancel();
		} else {
			tool.release(toMap(at(e)), screen, mods(e));
		}
		invalidate();
	}
	function onContextMenu(e: MouseEvent): void {
		e.preventDefault();
		if (!tool) {
			return;
		}
		measure();
		tool.secondary(toMap(at(e)), screen);
		invalidate();
	}
	function onPointerEnter(): void {
		measure();
	}
	function onPointerLeave(e: PointerEvent): void {
		if (!tool || claimed !== null || pointers.size > 0) {
			return;
		}
		if (e.relatedTarget instanceof Node && canvas.parentElement?.contains(e.relatedTarget)) {
			return;
		}
		tool.hover(null);
		invalidate();
	}
	function onWheel(e: WheelEvent): void {
		e.preventDefault();
		const point = at(e);
		pending.zoom *= wheelMultiplier(e.deltaY, e.deltaMode, e.ctrlKey);
		pending.zoomX = point.x;
		pending.zoomY = point.y;
		invalidate();
	}
	canvas.addEventListener("pointerdown", onPointerDown);
	canvas.addEventListener("pointermove", onPointerMove);
	canvas.addEventListener("pointerup", onPointerUp);
	canvas.addEventListener("pointercancel", onPointerUp);
	canvas.addEventListener("pointerenter", onPointerEnter);
	canvas.addEventListener("pointerleave", onPointerLeave);
	canvas.addEventListener("contextmenu", onContextMenu);
	window.addEventListener("resize", measure);
	measure();
	canvas.addEventListener("wheel", onWheel, { passive: false });
	return {
		pending,
		dragging: () => pointers.size > 0 || claimed !== null || (tool?.active() ?? false),
		stop() {
			canvas.removeEventListener("pointerdown", onPointerDown);
			canvas.removeEventListener("pointermove", onPointerMove);
			canvas.removeEventListener("pointerup", onPointerUp);
			canvas.removeEventListener("pointercancel", onPointerUp);
			canvas.removeEventListener("pointerenter", onPointerEnter);
			canvas.removeEventListener("pointerleave", onPointerLeave);
			canvas.removeEventListener("contextmenu", onContextMenu);
			window.removeEventListener("resize", measure);
			canvas.removeEventListener("wheel", onWheel);
			pointers.clear();
		},
	};
}
function firstTwo(pointers: Map<number, Tracked>): [Tracked | undefined, Tracked | undefined] {
	const iterator = pointers.values();
	return [iterator.next().value, iterator.next().value];
}
function distance(a: Tracked, b: Tracked): number {
	return Math.hypot(a.x - b.x, a.y - b.y);
}
