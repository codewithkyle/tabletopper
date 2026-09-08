// The camera, and the two conversions every other module goes through to ask
// where something is.
//
// IT IS WORLD-CENTRED: (x, y) is the MAP PIXEL under the middle of the
// viewport, not the map pixel at its top-left corner. Both work, and this one
// is chosen because every operation the room actually performs is centred --
// fitting a map, zooming at the cursor, keeping the view over the map while it
// resizes. A corner-anchored camera makes each of those a subtraction of half a
// viewport that has to be got right in four places.
//
// COORDINATES ARE MAP PIXELS AND FLOATS. The protocol's integers are where
// things ARE; this is where the viewer is LOOKING, which is a continuous
// quantity -- a zoom of 0.37 anchored at a cursor lands the camera between two
// pixels and rounding it there would make zooming out and back in drift.
//
// SCREEN COORDINATES HERE ARE CSS PIXELS, never device pixels. The device
// pixel ratio enters in exactly one place, clipMatrix, because that is the only
// place that talks to the GPU. Pointer events arrive in CSS pixels and getting
// the two mixed up is a bug that only shows on a retina display.

export interface Camera {
	x: number;
	y: number;
	zoom: number;
}

// Viewport is the drawing area in CSS pixels.
export interface Viewport {
	width: number;
	height: number;
}

// Point is an out-parameter rather than a return value. These run per frame
// rather than per pawn, so the allocation would not actually matter -- but the
// callers that come in later phases are per pawn, and a signature that has to
// change then is a signature that gets used wrongly first.
export interface Point {
	x: number;
	y: number;
}

// Rect is a map-space rectangle: the map itself, or the part of it a viewport
// covers. x2 and y2 are exclusive, so width is x2 - x1.
export interface Rect {
	x1: number;
	y1: number;
	x2: number;
	y2: number;
}

// THE RANGE IS WIDER THAN THE OLD CLIENT'S 0.1 TO 2 because tiles made the
// whole-map view cheap. At 0.05 a 12000 pixel map is 600 pixels across, which
// is the overview a GM wants when moving the party between wings; at 4 a token
// is four screen pixels per map pixel, which is close enough to see what a
// texture actually contains.
export const ZOOM_MIN = 0.05;
export const ZOOM_MAX = 4;

// FIT_MARGIN leaves a tenth of the smaller axis as breathing room, so a fitted
// map does not touch the menu bar.
const FIT_MARGIN = 0.9;

export function newCamera(): Camera {
	return { x: 0, y: 0, zoom: 1 };
}

export function clampZoom(zoom: number): number {
	return Math.min(Math.max(zoom, ZOOM_MIN), ZOOM_MAX);
}

// worldToScreen maps a map pixel to a CSS pixel in the viewport.
export function worldToScreen(cam: Camera, vp: Viewport, wx: number, wy: number, out: Point): Point {
	out.x = (wx - cam.x) * cam.zoom + vp.width / 2;
	out.y = (wy - cam.y) * cam.zoom + vp.height / 2;

	return out;
}

// screenToWorld is the inverse, and the one the pointer goes through.
export function screenToWorld(cam: Camera, vp: Viewport, sx: number, sy: number, out: Point): Point {
	out.x = (sx - vp.width / 2) / cam.zoom + cam.x;
	out.y = (sy - vp.height / 2) / cam.zoom + cam.y;

	return out;
}

// visibleRect is the part of map space the viewport covers, which is what the
// tile pass intersects with the map to decide what to fetch.
export function visibleRect(cam: Camera, vp: Viewport, out: Rect): Rect {
	const halfW = vp.width / 2 / cam.zoom;
	const halfH = vp.height / 2 / cam.zoom;

	out.x1 = cam.x - halfW;
	out.y1 = cam.y - halfH;
	out.x2 = cam.x + halfW;
	out.y2 = cam.y + halfH;

	return out;
}

// panBy moves the camera by a drag measured in CSS pixels. The sign is
// inverted because dragging the map right moves the camera left.
export function panBy(cam: Camera, dx: number, dy: number): void {
	cam.x -= dx / cam.zoom;
	cam.y -= dy / cam.zoom;
}

// zoomAt multiplies the zoom while keeping the map pixel under (sx, sy) exactly
// where it is. That anchor is the whole feel of a map application: zooming at
// the cursor rather than at the centre is what lets somebody drill into a
// corridor without a pan between every step.
//
// IT IS WRITTEN AS "WHERE WAS THAT POINT, WHERE IS IT NOW, MOVE BACK BY THE
// DIFFERENCE" rather than as a closed-form expression, because the closed form
// has to be re-derived whenever the projection changes and this does not. The
// clamp is applied before the second lookup, so a zoom that hits the limit
// anchors correctly instead of drifting by the amount it was refused.
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

// zoomTo sets an absolute zoom about the viewport centre, which is what the
// View menu's 100% and 200% mean.
export function zoomTo(cam: Camera, vp: Viewport, zoom: number): void {
	zoomAt(cam, vp, vp.width / 2, vp.height / 2, zoom / cam.zoom);
}

// fit centres a map and picks the zoom that shows all of it.
export function fit(cam: Camera, vp: Viewport, width: number, height: number): void {
	if (width < 1 || height < 1 || vp.width < 1 || vp.height < 1) {
		return;
	}

	cam.zoom = clampZoom(Math.min(vp.width / width, vp.height / height) * FIT_MARGIN);
	cam.x = width / 2;
	cam.y = height / 2;
}

// clampToMap keeps the viewport centre over the map.
//
// THE RULE IS "THE MIDDLE OF THE SCREEN IS ALWAYS ON THE MAP", which is the
// same thing as saying the map can be pushed at most half a viewport off the
// edge. It is one comparison per axis and it has no configuration, and the
// alternative -- letting the map leave entirely and offering a Fit map item to
// get back -- is a state a person can reach by accident and cannot see their
// way out of.
//
// A MAP SMALLER THAN THE VIEWPORT IS LOCKED CENTRED on that axis instead.
// There is nothing to explore in a direction where everything is already
// visible, and a map that can be nudged into a corner at low zoom reads as the
// view having slipped rather than as freedom.
export function clampToMap(cam: Camera, vp: Viewport, width: number, height: number): void {
	if (width < 1 || height < 1) {
		return;
	}

	cam.x = clampAxis(cam.x, vp.width / cam.zoom, width);
	cam.y = clampAxis(cam.y, vp.height / cam.zoom, height);
}

function clampAxis(centre: number, viewport: number, size: number): number {
	if (viewport >= size) {
		return size / 2;
	}

	return Math.min(Math.max(centre, 0), size);
}

// clipMatrix is the camera as the vertex shaders take it: map pixels in, clip
// space out, as a mat3 in the column-major order uniformMatrix3fv wants.
//
// THIS IS THE ONLY FUNCTION THAT KNOWS ABOUT DEVICE PIXELS, and it takes the
// canvas's backing-store size rather than the CSS viewport for that reason. The
// y axis is negated because map space grows downwards and clip space grows up.
//
// The matrix is written into a caller-owned array so the frame loop uploads
// from the same Float32Array every frame.
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

// inverseClipMatrix goes the other way, clip space to map pixels, which is what
// the grid's vertex shader needs: it draws one triangle in clip space and has
// to know what map coordinate each of its corners lands on.
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
