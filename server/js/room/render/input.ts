// Pointer, wheel and touch, turned into camera movement.
//
// NO WORK HAPPENS IN A HANDLER. That is the first performance rule in the
// overview and this is the module it exists for: a wheel event can arrive
// dozens of times between two frames and a pointermove arrives at the mouse's
// polling rate, which on a gaming mouse is a thousand times a second. A handler
// that rendered would render nine frames the display never shows for every one
// it does.
//
// So a handler does two things: it adds what just happened to a small mutable
// accumulator, and it asks for a frame. The frame calls apply() once, which
// collapses however many events arrived into one pan and one zoom, and the
// accumulator is reset. A fast mouse and a slow one cost the same.
//
// EVERY LISTENER IS ON THE CANVAS, which is what keeps the windows working
// without a single line about them. A window is a sibling element stacked above
// the canvas, so a pointer that goes down on a title bar targets the window and
// never reaches here -- which is the rule in CLAUDE.md that anything
// hit-testing the table must ignore events inside a window, satisfied by
// listening in the right place rather than by testing for it.

import type { Camera, Viewport } from "./camera.ts";
import { panBy, zoomAt } from "./camera.ts";

// ZOOM_STEP_MAX is how much one event may change the zoom: 25 percent.
//
// IT IS CLAMPED AS AN EXPONENT rather than as a multiplier, so the limit is
// exactly symmetric -- 1.25 in and 1/1.25 out. Clamping the multiplier to a
// range like [0.75, 1.25] instead makes a notch out larger than a notch in, and
// a wheel rolled down and back up does not return to where it started.
const ZOOM_STEP_MAX = Math.log(1.25);

// WHEEL_SCALE turns a delta in pixels into an exponent. A conventional mouse
// notch is 100 pixels, which lands at about 14 percent per notch.
const WHEEL_SCALE = 0.0015;

// PINCH_SCALE is the same for a trackpad, which reports a pinch as a wheel
// event with ctrlKey set and a delta an order of magnitude smaller. Without a
// separate constant a trackpad pinch barely moves.
const PINCH_SCALE = 0.01;

// A wheel event in line mode reports lines rather than pixels; the browsers
// that still do this mean about this many.
const PIXELS_PER_LINE = 16;
const PIXELS_PER_PAGE = 400;

// Pending is everything that happened since the last frame, collapsed.
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

// apply folds the accumulator into the camera and empties it. It answers
// whether anything actually moved, which is what the frame reports back to the
// loop as "something changed".
//
// PAN FIRST, THEN ZOOM. During a pinch both are non-zero and describe the same
// movement of the same two fingers: the drag happened at the zoom the frame
// started at, so it is applied at that zoom, and the zoom is then anchored at
// where the fingers ended up.
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

// wheelMultiplier reads one wheel event as a zoom factor.
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

// A tracked pointer and where it was last seen, in CSS pixels relative to the
// canvas. Two of these at once is a pinch.
interface Tracked {
	x: number;
	y: number;
}

export interface Input {
	pending: Pending;

	// dragging is true while a button or a finger is down, and it is what keeps
	// the frame loop alive through a drag that has paused rather than ended.
	dragging(): boolean;

	stop(): void;
}

// PAN_BUTTONS is the primary and the middle button. The primary one will
// eventually belong to whichever tool is selected in the pill and pan only
// under Move; there is no tool that reads the table yet, so it pans always.
const PAN_BUTTONS = new Set([0, 1]);

export function wireInput(canvas: HTMLCanvasElement, invalidate: () => void): Input {
	const pending = newPending();
	const pointers = new Map<number, Tracked>();

	function at(e: PointerEvent | WheelEvent): Tracked {
		const rect = canvas.getBoundingClientRect();

		return { x: e.clientX - rect.left, y: e.clientY - rect.top };
	}

	function onPointerDown(e: PointerEvent): void {
		if (!PAN_BUTTONS.has(e.button)) {
			return;
		}

		// The middle button's default is the scroll-anywhere puck on Windows
		// and Linux, which appears over the table and stays there.
		e.preventDefault();

		canvas.setPointerCapture(e.pointerId);
		pointers.set(e.pointerId, at(e));
	}

	function onPointerMove(e: PointerEvent): void {
		const tracked = pointers.get(e.pointerId);
		if (!tracked) {
			return;
		}

		const now = at(e);

		if (pointers.size === 1) {
			pending.panX += now.x - tracked.x;
			pending.panY += now.y - tracked.y;
			tracked.x = now.x;
			tracked.y = now.y;
			invalidate();

			return;
		}

		// Two or more: a pinch, measured between the first two pointers. The
		// midpoint's movement is the pan and the change in separation is the
		// zoom, both read BEFORE and AFTER this one pointer moves, so a pinch
		// with one finger still works out.
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
		if (!pointers.delete(e.pointerId)) {
			return;
		}
		if (canvas.hasPointerCapture(e.pointerId)) {
			canvas.releasePointerCapture(e.pointerId);
		}
	}

	function onWheel(e: WheelEvent): void {
		// Without this the page scrolls, and on a trackpad a two finger pinch
		// zooms the whole browser instead of the map.
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

	// passive: false is what makes preventDefault above legal. A wheel listener
	// is passive by default in every browser, and a passive listener that calls
	// preventDefault is ignored with a console warning.
	canvas.addEventListener("wheel", onWheel, { passive: false });

	return {
		pending,
		dragging: () => pointers.size > 0,
		stop() {
			canvas.removeEventListener("pointerdown", onPointerDown);
			canvas.removeEventListener("pointermove", onPointerMove);
			canvas.removeEventListener("pointerup", onPointerUp);
			canvas.removeEventListener("pointercancel", onPointerUp);
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
