// Fog of war: the geometry, the gesture, and the one question the rest of the
// table asks it -- is this point hidden from the person looking at the screen.
//
// SHAPES ARE THE SOURCE OF TRUTH AND THE MASK IS A CACHE. That decision is
// internal/room/fog.go's and everything here follows from it: a rectangle is
// four integers on the wire, the texture it produces is a megabyte, and every
// client rasterises its own. So this module turns shapes into triangles and
// into an answer about a point, and render/fog-pass.ts turns the triangles into
// pixels.
//
// THE TRIANGULATION IS OURS RATHER THAN A DEPENDENCY. Ear clipping is sixty
// lines for the polygons this feature can produce -- a simple ring a GM clicked
// out, no holes, at most a thousand corners under FogPointsMax -- and the
// alternative was a second runtime dependency in a bundle that ships to every
// player at the table. path.ts already writes its own supercover rasteriser for
// the same reason.
//
// IT IS ALSO WHY THERE IS A BAIL-OUT IN IT. A GM CAN draw a polygon that
// crosses itself, ear clipping has no defined answer for one, and what must not
// happen is a loop that never ends inside a frame. See triangulate.
//
// CONCEALMENT IS CLIENT-SIDE AND IT IS PRESENTATION RATHER THAN SECRECY. Every
// shape and every pawn still reaches a player's browser, and the map itself is
// a URL they could open in another tab -- so what covered() buys is that the
// screen does not tell them, which is the whole of what fog has ever done at a
// physical table. Server-side projection stays deferred in the overview, and it
// would be half a measure while the image is fetchable.

import type { FogMode, FogShape, Grid, Pawn, Role, ShapeKind, State } from "./protocol.ts";
import type { Outgoing } from "./socket.ts";
import type { Modifiers } from "./render/input.ts";
import type { Outline, Segment } from "./pawns.ts";
import type { Point } from "./render/camera.ts";
import { snapAxis } from "./render/path.ts";
import { typing } from "./keys.ts";

// The two preview colours. Both are LIGHT, which is not a mistake about "a
// reveal is bright and a hide is dark": a preview is drawn over whatever the map
// happens to be and a dark outline on a dark dungeon is an outline nobody can
// aim with. What separates them is hue, which survives any background.
const REVEAL_COLOR: readonly [number, number, number] = [1.0, 0.82, 0.35];
const HIDE_COLOR: readonly [number, number, number] = [0.55, 0.83, 0.99];

// The preview's weights, in CSS pixels, matching the marquee's.
const PREVIEW_WIDTH = 2;
const PREVIEW_ALPHA = 0.95;

// A FOG CORNER SNAPS TO A CELL VERTEX AND NOT TO A CELL CENTRE, which is what
// the even footprint below buys: snapAxis with an even footprint drops the
// half-cell offset and lands on the lattice where the lines cross. A reveal is a
// room and rooms are drawn on the grid, so its corners belong where the grid's
// corners are.
const VERTEX_FOOTPRINT = 2;

// EARS_MAX bounds the ear-clipping loop. A ring of n corners needs n - 2 ears,
// and every pass of the loop either removes one or advances; this is the belt
// and braces that keeps a self-crossing ring from spinning inside a frame.
const EARS_MAX = 100_000;

// snapCorner puts one point on the grid's vertices, unless the grid does not
// snap or Alt is held. Alt is the bypass for the diagonal corridor that no
// lattice has a corner for.
export function snapCorner(grid: Grid, x: number, y: number, alt: boolean): [number, number] {
	if (alt) {
		return [Math.round(x), Math.round(y)];
	}

	return [
		snapAxis(grid.cellSize, grid.offsetX, VERTEX_FOOTPRINT, grid.snap, x),
		snapAxis(grid.cellSize, grid.offsetY, VERTEX_FOOTPRINT, grid.snap, y),
	];
}

// rectTriangles turns two opposite corners into the six vertices of two
// triangles, in map pixels. The corners are normalised on the way, so a
// rectangle dragged up and to the left is the same rectangle as one dragged down
// and to the right.
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

// triangulate is ear clipping over one closed ring, answering the flat triangle
// list in map pixels.
//
// THE RING'S WINDING IS MEASURED FIRST and the ear test is written in terms of
// it, so a ring clicked out clockwise and one clicked out anticlockwise both
// work -- which matters because nothing tells a GM which way to go round a room.
export function triangulate(ring: readonly number[], out: number[]): number[] {
	out.length = 0;

	const n = ring.length >> 1;
	if (n < 3) {
		return out;
	}

	// The remaining corners, as indices into the ring. Clipping an ear removes
	// one from here rather than rewriting the ring.
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

		// A WHOLE LAP WITH NO EAR IS A RING THAT IS NOT SIMPLE -- it crosses
		// itself, which a GM can draw and no triangulation can answer. The
		// remaining corners go out as a fan, which is wrong in the same way the
		// input was and is bounded, drawn, and over.
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

// signedArea is twice the ring's area, negative for a clockwise ring in a
// y-down space. Only its sign is read.
function signedArea(ring: readonly number[]): number {
	let sum = 0;
	for (let i = 0, n = ring.length >> 1; i < n; i++) {
		const j = (i + 1) % n;
		sum += ring[i * 2] * ring[j * 2 + 1] - ring[j * 2] * ring[i * 2 + 1];
	}

	return sum;
}

// isEar is the two halves of the ear test: the corner turns the same way the
// ring does, and no other remaining corner is inside the triangle it cuts off.
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

// bounds is a cache of every shape's bounding box, keyed by the shape object
// itself.
//
// IT IS WHAT KEEPS CONCEALMENT OFF THE CRITICAL PATH. covered() is asked once
// per pawn on every hover, every press and every marquee, and a polygon a GM
// clicked round a cave is fifty coordinates -- so without a box to reject
// against, one pointer move over a well-explored floor is the pawn count times
// the shape count times fifty. With one it is the pawn count times the shape
// count, in comparisons.
//
// A WeakMap AND NOT A FIELD, because a FogShape is the protocol's own type: it
// is generated from Go, it arrives off the socket, and it is not ours to hang a
// cache on. A shape that leaves the store takes its entry with it.
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

// insideShape is a point test against one shape, in map pixels. A rectangle is
// its two corners normalised; a polygon is the even-odd crossing count, which is
// the same rule the rasteriser's triangles produce for a simple ring and the
// only rule that is defined for one that crosses itself.
export function insideShape(shape: FogShape, x: number, y: number): boolean {
	const p = shape.points;
	const [minX, minY, maxX, maxY] = boxOf(shape);

	// The box first, and for a rectangle it is the whole answer.
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

// coveredBy walks one floor's shapes IN ORDER from the prefill and answers
// whether the point ends up hidden. Order is meaning here: a hide drawn over a
// reveal covers it again, which is why the last shape containing the point wins
// rather than the first.
export function coveredBy(
	shapes: readonly FogShape[], layerID: string, prefill: boolean, x: number, y: number,
): boolean {
	// WALKED BACKWARDS, WHICH IS THE SAME ANSWER AND NOT THE SAME COST. The
	// LAST shape containing the point decides, so the first one found going
	// backwards is that shape and there is nothing left to ask -- and a point
	// under the newest reveal, which is where a party standing in the room the
	// GM just opened is, costs one test rather than all of them.
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

// maskRect is the rectangle the mask texture covers, in map pixels: the map's
// own rectangle unioned with the bounding box of the floor's shapes, padded by
// a cell.
//
// THE UNION IS THE POINT AND THE MAP ALONE IS NOT ENOUGH. The cover is drawn
// over the whole viewport rather than over the image -- play leaves the map,
// which is why phase 4 made the grid infinite -- so a GM can clear the road out
// of town, on ground the picture does not reach. Sizing the mask to the image
// would drop that shape silently.
//
// A FLOOR WITH NEITHER ANSWERS NULL, which the shader reads as "sample nothing
// and use the prefill everywhere".
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

	// The pad is a cell so that a reveal drawn hard against the edge of the
	// rectangle keeps a texel of mask outside it to fade into, rather than
	// ending in the clamp.
	const pad = Math.max(cell, 1);
	out.x = minX - pad;
	out.y = minY - pad;
	out.width = maxX - minX + pad * 2;
	out.height = maxY - minY + pad * 2;

	return out;
}

// MaskRect is the mask texture's footprint in map pixels. It is its own type
// rather than camera.ts's Rect, which is a pair of corners: this one is fed
// straight to a shader as an origin and a size, and converting between the two
// in a uniform call is where a sign error would live.
export interface MaskRect {
	x: number;
	y: number;
	width: number;
	height: number;
}

// FogOptions is the second pill's state, asked for rather than pushed. See
// fog-tool.ts.
export interface FogOptions {
	shape: ShapeKind;
	mode: FogMode;
}

export interface FogDeps {
	state: State;
	role: Role;
	user: string;
	viewed: () => string;
	grid: () => Grid;
	send: (command: Outgoing) => void;
	invalidate: () => void;

	// fogging is whether the Fog tool is the one CHOSEN in the pill, which the
	// space bar does not change: shoving the map along a corridor must not put
	// a half-drawn polygon away. It is tools.ts's answer, read at a press and
	// on the way to a frame.
	fogging: () => boolean;

	// options is what the second pill says: rectangle or polygon, uncover or
	// cover. It is read at the moment a shape is FINISHED rather than when it
	// was started, which is deliberate -- a GM who clicks four corners and then
	// realises they meant to cover rather than uncover can say so before
	// closing the ring.
	options: () => FogOptions;
}

// Fog is what pawns.ts hands the table's gestures to while the Fog tool is
// chosen, plus the one question the renderer and the hit test ask of it.
export interface Fog {
	press(map: Point, mods: Modifiers): boolean;
	drag(map: Point, mods: Modifiers): void;
	release(map: Point, mods: Modifiers): void;

	// secondary answers whether the right button was SPENT here. A polygon in
	// hand closes on it -- which is the gesture every mapping tool has and the
	// one this GM asked for -- and a rectangle in hand is abandoned by it, the
	// way the right button abandons everything else on this table. With nothing
	// in hand it answers false and the press goes on to mean what it always
	// meant.
	secondary(): boolean;

	hover(map: Point | null): void;

	// key is Enter, Backspace and Ctrl+Z while the tool is chosen. It answers
	// whether the key was spent.
	key(e: KeyboardEvent): boolean;

	abandon(): boolean;

	outlines(out: Outline[]): Outline[];
	marks(out: Segment[]): Segment[];

	// covered is the concealment question, asked of a point in map pixels on the
	// floor being viewed. It is false for a floor whose fog is off and false for
	// every GM.
	covered(x: number, y: number): boolean;

	// concealed is covered() with the two exemptions applied: the GM sees
	// everything, and a player's own pawns are never hidden from them.
	concealed(pawn: Pawn): boolean;
}

type Gesture =
	| { kind: "rect"; x0: number; y0: number; x1: number; y1: number }
	| { kind: "poly"; points: number[] }
	| null;

export function createFog(deps: FogDeps): Fog {
	const state = deps.state;

	let gesture: Gesture = null;

	// pointer is where the rubber band ends: the last place the pointer was seen
	// over the table, in map pixels, or null when it has left.
	let pointer: Point | null = null;

	function layer(): { fogEnabled: boolean; fogPrefill: boolean } | null {
		const id = deps.viewed();
		for (const l of state.table.layers) {
			if (l.id === id) {
				return l;
			}
		}

		return null;
	}

	function corner(map: Point, mods: Modifiers): [number, number] {
		return snapCorner(deps.grid(), map.x, map.y, mods.alt);
	}

	function send(kind: ShapeKind, points: number[]): void {
		const id = deps.viewed();
		if (id === "" || points.length < 4) {
			return;
		}

		// THE MODE IS READ HERE AND NOT WHEN THE GESTURE BEGAN. See FogDeps.
		//
		// NOTHING IS SENT ABOUT THE FLOOR'S FLAGS. A floor whose fog is off is
		// woken by fog.add itself, in the command, which is one message rather
		// than three and one ordering rather than three. See room.FogAdd.
		deps.send({ type: "fog.add", layer: id, kind, mode: deps.options().mode, points });
	}

	function finishPolygon(): boolean {
		if (gesture?.kind !== "poly") {
			return false;
		}

		const points = gesture.points;
		gesture = null;
		deps.invalidate();

		// Two corners are a line and a line covers nothing. Dropping them is the
		// same answer Escape gives, which is what somebody who right-clicked
		// after two corners meant: this is not going to be a shape.
		if (points.length >= 6) {
			send("poly", points);
		}

		return true;
	}

	function undo(): void {
		const id = deps.viewed();

		// The store keeps the shapes in the order they arrived, so the newest on
		// this floor is the last one that names it.
		for (let i = state.fog.length - 1; i >= 0; i--) {
			if (state.fog[i].layerId === id) {
				deps.send({ type: "fog.remove", id: state.fog[i].id });

				return;
			}
		}
	}

	function previewColor(): readonly [number, number, number] {
		return deps.options().mode === "hide" ? HIDE_COLOR : REVEAL_COLOR;
	}

	function covered(x: number, y: number): boolean {
		const l = layer();
		if (!l || !l.fogEnabled) {
			return false;
		}

		return coveredBy(state.fog, deps.viewed(), l.fogPrefill, x, y);
	}

	return {
		press(map, mods) {
			if (!deps.fogging()) {
				return false;
			}

			const [x, y] = corner(map, mods);

			if (deps.options().shape === "poly") {
				if (gesture?.kind !== "poly") {
					gesture = { kind: "poly", points: [] };
				}

				// A CORNER ON TOP OF THE LAST ONE IS NOT A CORNER. Snapping
				// makes this common rather than rare: two clicks in the same
				// cell land on the same vertex, and a ring carrying the same
				// point twice has a zero-length edge in it.
				const points = gesture.points;
				const n = points.length;
				if (n < 2 || points[n - 2] !== x || points[n - 1] !== y) {
					points.push(x, y);
				}
			} else {
				gesture = { kind: "rect", x0: x, y0: y, x1: x, y1: y };
			}

			deps.invalidate();

			return true;
		},

		drag(map, mods) {
			if (gesture?.kind !== "rect") {
				return;
			}

			const [x, y] = corner(map, mods);
			gesture.x1 = x;
			gesture.y1 = y;
			deps.invalidate();
		},

		release(map, mods) {
			if (gesture?.kind !== "rect") {
				return;
			}

			const [x, y] = corner(map, mods);
			const { x0, y0 } = gesture;
			gesture = null;
			deps.invalidate();

			// A CLICK IS NOT A RECTANGLE. The test is on the SNAPPED corners
			// rather than on how far the hand moved, because that is what
			// decides whether the shape has any area: a drag across half a cell
			// snaps to nothing at all and would send a rectangle nobody could
			// see.
			if (x === x0 || y === y0) {
				return;
			}

			send("rect", [Math.min(x0, x), Math.min(y0, y), Math.max(x0, x), Math.max(y0, y)]);
		},

		secondary() {
			if (gesture?.kind === "poly") {
				return finishPolygon();
			}

			if (gesture?.kind === "rect") {
				gesture = null;
				deps.invalidate();

				return true;
			}

			return false;
		},

		hover(map) {
			pointer = map ? { x: map.x, y: map.y } : null;
		},

		key(e) {
			if (!deps.fogging() || typing(e.target)) {
				return false;
			}

			if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "z") {
				undo();

				return true;
			}

			// Every key below is a gesture's, so a modifier makes it somebody
			// else's -- Ctrl-Backspace is a word in a field, not a corner.
			if (e.ctrlKey || e.metaKey || e.altKey) {
				return false;
			}

			if (e.key === "Enter") {
				return finishPolygon();
			}

			if (e.key === "Backspace" && gesture?.kind === "poly") {
				gesture.points.length = Math.max(0, gesture.points.length - 2);
				if (gesture.points.length === 0) {
					gesture = null;
				}
				deps.invalidate();

				return true;
			}

			return false;
		},

		abandon() {
			if (!gesture) {
				return false;
			}

			gesture = null;
			deps.invalidate();

			return true;
		},

		outlines(out) {
			if (gesture?.kind !== "rect") {
				return out;
			}

			const halfW = Math.abs(gesture.x1 - gesture.x0) / 2;
			const halfH = Math.abs(gesture.y1 - gesture.y0) / 2;
			if (halfW <= 0 || halfH <= 0) {
				return out;
			}

			out.push({
				x: (gesture.x0 + gesture.x1) / 2,
				y: (gesture.y0 + gesture.y1) / 2,
				halfW, halfH,
				color: previewColor(),
				alpha: PREVIEW_ALPHA,
				thickness: PREVIEW_WIDTH,
				rect: true,
				rotation: 0,
			});

			return out;
		},

		marks(out) {
			if (gesture?.kind !== "poly" || gesture.points.length < 2) {
				return out;
			}

			const points = gesture.points;
			const color = previewColor();

			for (let i = 0; i + 3 < points.length; i += 2) {
				out.push({
					x0: points[i], y0: points[i + 1],
					x1: points[i + 2], y1: points[i + 3],
					color, alpha: PREVIEW_ALPHA, width: PREVIEW_WIDTH,
				});
			}

			// THE RUBBER BAND IS TWO SEGMENTS AND NOT ONE, which is the whole
			// of what makes the gesture legible: the ring a right click is
			// about to close is drawn before it closes, so the shape under the
			// pointer is the shape that lands. One line to the pointer and none
			// back to the start would show a chain rather than a room.
			if (pointer) {
				const last = points.length - 2;
				out.push({
					x0: points[last], y0: points[last + 1],
					x1: pointer.x, y1: pointer.y,
					color, alpha: PREVIEW_ALPHA * 0.7, width: PREVIEW_WIDTH,
				});

				if (points.length >= 4) {
					out.push({
						x0: pointer.x, y0: pointer.y,
						x1: points[0], y1: points[1],
						color, alpha: PREVIEW_ALPHA * 0.4, width: PREVIEW_WIDTH,
					});
				}
			}

			return out;
		},

		covered,

		concealed(pawn) {
			// THE GM IS NEVER CONCEALED FROM, and neither is anybody's own
			// character. A player who walks into an unlit room and watches
			// their own token disappear has been told the app is broken rather
			// than that the room is dark.
			if (deps.role === "gm" || (pawn.ownerId !== null && pawn.ownerId === deps.user)) {
				return false;
			}

			return covered(pawn.x, pawn.y);
		},
	};
}
