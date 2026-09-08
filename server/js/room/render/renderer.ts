// The renderer: it owns the canvas, the camera and the passes, and it is the
// only module here that knows what a room is.
//
// IT READS THE STORE AND NEVER WRITES IT. main.ts reduces an event into the
// state and then tells this to draw again; there is one copy of the room on the
// client and it belongs to the reducer. Everything below is a function of that
// state plus a camera, which is why nothing here has to be kept in step with
// anything -- there is no second copy to drift.
//
// WHAT IS HERE IS THE GRID AND THE CAMERA. Tiles, pawns, fog and strokes are
// later phases and each is one more pass called from drawFrame, in that order,
// against the same camera matrix.

import type { Camera, Rect, Viewport } from "./camera.ts";
import type { MapRef, State } from "../protocol.ts";
import { clampToMap, fit, newCamera, zoomAt, zoomTo } from "./camera.ts";
import { createContext } from "./gl.ts";
import { createGridPass } from "./grid-pass.ts";
import { startFrames } from "./frame.ts";
import { apply, wireInput } from "./input.ts";

// VIEW_ZOOM_STEP is what the View menu's Zoom in and Zoom out move by. It is
// larger than a wheel notch because a menu item is a deliberate act and
// reaching the menu again for a second helping is expensive.
const VIEW_ZOOM_STEP = 1.5;

export interface Renderer {
	// invalidate asks for a frame. main.ts calls it when the reducer has
	// changed something the canvas draws.
	invalidate(): void;

	// timings is the benchmark's readout: CPU milliseconds inside the render.
	timings(): { average: number; p95: number; samples: number };
	resetTimings(): void;

	stop(): void;
}

// mountRenderer answers null when there is nothing to draw on, which is a
// browser without WebGL2 or a page that rendered no canvas. Neither is an
// error: the room's menus, windows and player list are ordinary HTML and go on
// working, and the caller has nothing to do about it.
export function mountRenderer(mount: HTMLElement, state: State): Renderer | null {
	const found = mount.querySelector("[data-tabletop-canvas]");
	if (!(found instanceof HTMLCanvasElement)) {
		return null;
	}

	// The two re-declarations are what carry the narrowing above into the
	// closures below: TypeScript does not keep a control-flow narrowing across
	// a function boundary, and every frame is a function boundary.
	const canvas: HTMLCanvasElement = found;

	const context = createContext(canvas);
	if (!context) {
		// The message is a hidden element the page rendered, not markup built
		// here: server/js is not a Tailwind source, so a class name written in
		// this file would never be emitted into the stylesheet.
		mount.querySelector("[data-tabletop-unsupported]")?.removeAttribute("hidden");
		canvas.hidden = true;

		return null;
	}

	const gl: WebGL2RenderingContext = context;
	const grid = createGridPass(gl);

	const camera: Camera = newCamera();
	const viewport: Viewport = { width: 1, height: 1 };
	let dpr = window.devicePixelRatio || 1;

	const mapRect: Rect = { x1: 0, y1: 0, x2: 0, y2: 0 };
	const clear = { r: 0, g: 0, b: 0 };

	// lastMap is how "the map changed" is noticed without a subscription. The
	// key is the asset and its tiling generation, so a re-tiled map counts as a
	// new one -- which it is, at new URLs, with possibly new dimensions.
	let lastMap = "";

	const input = wireInput(canvas, () => frames.invalidate());

	const frames = startFrames({
		mount,
		canvas,
		render: drawFrame,
		resized() {
			// THE ONLY DOM READ IN THE WHOLE LOOP, and it is here rather than
			// in drawFrame because a getBoundingClientRect inside a frame
			// forces layout on every frame. Size changes are rare; frames are
			// not.
			dpr = window.devicePixelRatio || 1;
			const rect = mount.getBoundingClientRect();
			viewport.width = Math.max(1, rect.width);
			viewport.height = Math.max(1, rect.height);
		},
	});

	readClearColor();
	window.addEventListener("theme:change", readClearColor);
	window.addEventListener("room:view", onViewCommand as EventListener);

	frames.invalidate();

	function drawFrame(): boolean {
		const map = activeMap(state);

		// A map arriving, or being swapped for another, frames itself. Doing it
		// on the first frame that sees the map rather than on the event means
		// it happens once the viewport is known, so a room opened in a
		// background tab still fits correctly when it is looked at.
		const key = map ? `${map.assetId}:${map.gen}` : "";
		if (key !== lastMap) {
			lastMap = key;
			if (map) {
				fit(camera, viewport, map.width, map.height);
			}
		}

		const moved = apply(input.pending, camera, viewport);
		if (moved && map) {
			clampToMap(camera, viewport, map.width, map.height);
		}

		gl.viewport(0, 0, canvas.width, canvas.height);
		gl.clearColor(clear.r, clear.g, clear.b, 1);
		gl.clear(gl.COLOR_BUFFER_BIT);

		grid.draw(camera, state.table.grid, mapRectOf(map), canvas.width, canvas.height, dpr);

		// The loop keeps running while a button or a finger is down, so a drag
		// that pauses does not settle into a stale frame, and stops the moment
		// it is released.
		return input.dragging();
	}

	function mapRectOf(map: MapRef | null): Rect | null {
		if (!map) {
			return null;
		}

		mapRect.x1 = 0;
		mapRect.y1 = 0;
		mapRect.x2 = map.width;
		mapRect.y2 = map.height;

		return mapRect;
	}

	function onViewCommand(e: CustomEvent<{ action?: string }>): void {
		const map = activeMap(state);

		switch (e.detail?.action) {
			case "zoom-in":
				zoomAt(camera, viewport, viewport.width / 2, viewport.height / 2, VIEW_ZOOM_STEP);
				break;
			case "zoom-out":
				zoomAt(camera, viewport, viewport.width / 2, viewport.height / 2, 1 / VIEW_ZOOM_STEP);
				break;
			case "zoom-1":
				zoomTo(camera, viewport, 1);
				break;
			case "zoom-2":
				zoomTo(camera, viewport, 2);
				break;
			case "fit":
				if (map) {
					fit(camera, viewport, map.width, map.height);
				}
				break;
			default:
				return;
		}

		if (map) {
			clampToMap(camera, viewport, map.width, map.height);
		}

		frames.invalidate();
	}

	// THE CLEAR COLOUR IS THE PAGE'S OWN, read from the element the canvas
	// covers, so the table and the chrome around it are the same shade and a
	// theme change moves both. It is resolved through a one pixel 2D canvas
	// rather than parsed, because a computed background-color is whatever
	// syntax the browser chose to serialise it as -- rgb(), colour with a
	// space, or the oklch() the themes are actually written in -- and canvas
	// resolves every one of them to the sRGB bytes that are wanted here.
	function readClearColor(): void {
		const probe = document.createElement("canvas").getContext("2d", { willReadFrequently: true });
		if (!probe) {
			return;
		}

		probe.fillStyle = "#000000";
		probe.fillStyle = window.getComputedStyle(mount).backgroundColor;
		probe.fillRect(0, 0, 1, 1);

		const pixel = probe.getImageData(0, 0, 1, 1).data;
		clear.r = pixel[0] / 255;
		clear.g = pixel[1] / 255;
		clear.b = pixel[2] / 255;

		frames.invalidate();
	}

	return {
		invalidate: frames.invalidate,
		timings: frames.timings,
		resetTimings: frames.resetTimings,

		stop() {
			window.removeEventListener("theme:change", readClearColor);
			window.removeEventListener("room:view", onViewCommand as EventListener);
			input.stop();
			frames.stop();
			grid.dispose();
		},
	};
}

// activeMap is the map the canvas is drawing.
//
// IT IS THE ACTIVE LAYER'S FOR EVERYONE TODAY. The GM's ability to view a layer
// other than the active one is a piece of client state with a crossfade
// attached, and it arrives with the tiles that make the crossfade mean
// something; until then there is one answer and both audiences get it.
function activeMap(state: State): MapRef | null {
	for (const layer of state.table.layers) {
		if (layer.id === state.table.activeLayer) {
			return layer.map;
		}
	}

	return null;
}
