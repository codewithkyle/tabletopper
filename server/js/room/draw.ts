// Drawing on the table: the geometry of a stroke, and the gestures that make
// one.
//
// STROKES ARE THE SOURCE OF TRUTH AND THE SEGMENTS ARE A CACHE. That is
// internal/room/stroke.go's decision and everything here follows from it: what
// the room holds is a polyline, or a shape as two points, and what the GPU
// wants is a list of segments -- so this module turns the first into the second
// and render/stroke-pass.ts turns the second into pixels. Nothing is stored
// expanded, which is what lets a shape be re-measured at any time.
//
// THE GESTURE CONTRACT IS fog.ts's, deliberately. press, drag, release,
// secondary, abandon and key are the same six pawns.ts already routes, so a
// second tool on this table is a second implementation of a shape that works
// rather than a second way of taking a pointer.
//
// AND THE ONE PLACE IT PARTS COMPANY WITH THE FOG IS THAT A LINE IS ALREADY ON
// EVERY OTHER SCREEN BEFORE IT IS FINISHED. Fog is sent when a shape is done;
// a pen sends the first point immediately and chunks of the rest at roughly ten
// hertz, because watching somebody's line form is most of what drawing together
// is. So abandoning is not "send nothing" here -- see abandon.

import type { Grid, Role, State, Stroke, StrokeKind } from "./protocol.ts";
import type { Outgoing } from "./socket.ts";
import type { Modifiers } from "./render/input.ts";
import type { Label, Outline, Segment } from "./pawns.ts";
import type { Point } from "./render/camera.ts";
import { distanceLabel, feetBetween } from "./render/path.ts";
import { parseColor } from "./render/grid-pass.ts";
import { typing } from "./keys.ts";
import { ulid } from "./ulid.ts";

// CHUNK_MS and CHUNK_POINTS are when a run of points goes out: ten times a
// second, or every sixty-four points, whichever comes first.
//
// SIXTY-FOUR POINTS IS A HUNDRED AND TWENTY-EIGHT NUMBERS, well inside
// StrokeChunkMax of 512, so the count is what a message costs rather than what
// the server will take. The interval is what everybody else's screen sees; a
// line that arrived once per second would read as somebody drawing in stages.
const CHUNK_MS = 100;
const CHUNK_POINTS = 64;

// DEFAULT_WIDTH is what the pen draws with on a page that renders no options
// pill at all -- a closed room. Everywhere else the width is read out of the
// slider the template rendered, so that the control and the pen cannot open on
// two different numbers; see draw-tool.ts.
export const DEFAULT_WIDTH = 4;

// ERASE_RADIUS is how close the pointer has to come to a line to take it out,
// in CSS pixels. It is a SCREEN measurement and not a map one because it is
// about aiming: what a hand can hit is a few pixels on the display, whatever
// the map underneath is scaled to.
const ERASE_RADIUS = 6;

// PREVIEW_WIDTH is how heavy the outline of a shape being dragged is, in CSS
// pixels. It matches the marquee's and the fog's, because all three are the
// same thing: a box that follows the pointer and is gone when the button comes
// up.
const PREVIEW_WIDTH = 2;

// ERASE_COLOR and ERASE_WIDTH draw the cursor ring. It is pale for the reason
// the fog's previews are -- a dark ring over a dark dungeon cannot be aimed
// with -- and it is the only thing that makes an invisible hit radius usable.
const ERASE_COLOR: readonly [number, number, number] = [0.98, 0.98, 0.99];
const ERASE_WIDTH = 1.5;

// DrawOptions is what the tool draws with. It is asked for rather than pushed,
// and it is read when a stroke STARTS rather than when it finishes -- which is
// the opposite of the fog's rule and is right for the opposite reason: a fog
// shape is one message sent at the end, and a stroke's colour is already on
// everybody's screen by the time the hand lifts.
// DrawMode is what a gesture does. The three shape modes are the StrokeKind of
// the same name, which is why they are spelled the same: press reads the mode
// and puts it straight into the command.
export type DrawMode = "pen" | "rect" | "circle" | "cone" | "erase";

// SHAPES is which modes are placed rather than drawn, and it is the one test
// that separates them. It is the client's twin of StrokeKind.Shape() in Go.
const SHAPES: readonly DrawMode[] = ["rect", "circle", "cone"];

// Shaped is the three modes above, as the kind they send.
type Shaped = "rect" | "circle" | "cone";

export interface DrawOptions {
	mode: DrawMode;
	color: string;
	width: number;
}

export interface DrawDeps {
	state: State;
	role: Role;
	user: string;
	viewed: () => string;

	// grid is what turns map pixels into the feet a label reads in. It is asked
	// per frame rather than held, because a GM retuning the cell size while a
	// circle sits on the table has changed what that circle is worth.
	grid: () => Grid;
	send: (command: Outgoing) => void;
	invalidate: () => void;

	// scale is how many MAP pixels one CSS pixel covers. It is what decimation
	// is measured in: a hand moving a pixel across the screen has not drawn
	// anything worth a point, however many map pixels that is.
	scale: () => number;

	// drawing is whether the Draw tool is the one CHOSEN in the pill, which the
	// space bar does not change -- shoving the map along a corridor must not cut
	// a line in half. It is tools.ts's answer, asked at a press.
	drawing: () => boolean;

	options: () => DrawOptions;
}

export interface Draw {
	press(map: Point, mods: Modifiers): boolean;
	drag(map: Point, mods: Modifiers): void;
	release(map: Point, mods: Modifiers): void;

	// secondary is the right button, which abandons a line in hand and declines
	// when there is none.
	secondary(): boolean;

	abandon(): boolean;

	// key is the tool's keyboard, and it answers whether the key was spent.
	key(e: KeyboardEvent): boolean;

	// hover is the pointer moving over the table with nothing down, and null is
	// it leaving. The eraser's ring follows it.
	hover(map: Point | null): void;

	// outline is the shape being dragged, or the eraser's cursor, or null. It is
	// ONE reused object because there is only ever one of them: the two never
	// coexist, since each belongs to a different mode.
	//
	// A CONE IS NOT ONE OF THEM. The ring pass draws ellipses and rectangles,
	// which is a box and a ring and nothing else -- a triangle goes through
	// marks() instead, the way the fog's polygon does.
	outline(): Outline | null;

	// marks is the loose lines this tool wants over the table: the three sides
	// of a cone being dragged, and nothing else.
	marks(out: Segment[]): Segment[];

	// labels is the distance across every shape on the viewed floor, and across
	// the one in hand. See Label.
	labels(out: Label[]): Label[];

	// inHand is the stroke this viewer is drawing right now, from LOCAL points,
	// or null. The renderer draws it instead of the store's copy of the same
	// stroke; see the note on echoes in press.
	inHand(): Stroke | null;
}

// strokeSegments expands one stroke into the flat segment list the pass draws:
// x0, y0, x1, y1 per segment, appended to out.
//
// IT IS A PURE FUNCTION OF THE STROKE and it is exported for that reason -- the
// shapes are geometry with right answers, and a right answer is worth a test
// that does not need a GPU to run.
//
// A SINGLE POINT IS ONE SEGMENT OF NO LENGTH, which the shader draws as a round
// cap: a dot, which is what a click with a pen is.
//
// A SHAPE IS TWO POINTS AND THE KIND SAYS WHAT THEY MEAN -- a rectangle's
// opposite corners, a circle's centre and a point on its rim. Keeping them that
// way rather than storing the segments is what lets the distance under a shape
// be recomputed at any time, which is the whole reason a shape is a shape here.
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
			// A kind this build has never heard of draws nothing rather than
			// throwing: this runs inside a frame, and a stroke from a build
			// ahead of this one is not worth a black table.
			return out;
	}
}

// coneCorners writes a cone's three vertices into out -- the apex, then the two
// ends of its base -- from the two points a cone is stored as.
//
// THE BASE IS AS WIDE AS THE CONE IS LONG, and that is the whole shape of this
// function. A cone in the rules is written by ONE number, and at the far end it
// is as wide as it reaches; leaving the width free would be a shape with two
// things to aim at and a label that answered neither. So the only free
// parameters are where the point is and where the middle of the base is, which
// is exactly the four integers the wire carries.
//
// IT IS AN ISOSCELES TRIANGLE AT EVERY ANGLE. The base is perpendicular to the
// axis, so turning the pointer through a full circle turns the shape with it
// and never changes what it is worth.
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

	// The axis, and the perpendicular to it. Half the length each way from the
	// base's midpoint is a base the same width as the cone is long.
	const half = length / 2;
	const nx = (-dy / length) * half;
	const ny = (dx / length) * half;

	out.push(ax, ay);
	out.push(bx + nx, by + ny);
	out.push(bx - nx, by - ny);

	return out;
}

// circleSegments is how finely a circle is cut up, and the answer is "finely
// enough that nobody can see it".
//
// A CHORD OF FOUR MAP PIXELS ON A CIRCLE OF RADIUS r BULGES BY ABOUT 2/r MAP
// PIXELS at its middle, so a hundred-pixel circle is wrong by a fiftieth of a
// map pixel -- below a device pixel at any zoom this camera reaches. That is
// what makes tessellating here rather than drawing circles through the ring
// pass an invisible simplification: one buffer, one z-position, one rebuild
// path for every kind of stroke.
//
// THE CLAMP IS AT BOTH ENDS. Twenty-four keeps a tiny circle from being an
// octagon; five hundred and twelve keeps a circle drawn across a whole map from
// putting thousands of instances in the buffer for an accuracy nobody asked for.
export function circleSegments(radius: number): number {
	return Math.max(24, Math.min(512, Math.ceil((Math.PI * 2 * radius) / 4)));
}

// bounds is a cache of every stroke's bounding box, keyed by the stroke object
// itself.
//
// IT IS WHAT KEEPS THE ERASER OFF THE CRITICAL PATH. The hit test runs on every
// pointermove of a drag, and a floor can hold two hundred thousand coordinates
// under StrokePointsBudget -- so without a box to reject against, one sweep of
// the eraser is that number of distance tests per move. With one it is the
// stroke count in comparisons, and only the lines actually under the pointer
// are walked.
//
// A WeakMap AND NOT A FIELD, for fog.ts's reason: a Stroke is the protocol's own
// type, generated from Go and arriving off the socket, and it is not ours to
// hang a cache on. A stroke that leaves the store takes its entry with it.
//
// IT IS ONLY EVER ASKED ABOUT A FINISHED STROKE, which is what makes caching a
// box safe at all: an unfinished one grows a point at a time and its box would
// be stale by the next chunk. See erasable.
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

// strokeHit answers whether a point comes within `radius` map pixels of the ink
// of this stroke. The stroke's own width counts: a sixty-four pixel brush is
// hit where it is drawn, not where its centre line runs.
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

// segmentDistanceSquared is the squared distance from a point to a segment.
// Squared, because the only thing asked of it is a comparison against a
// threshold, and a square root per segment per pointermove buys nothing.
function segmentDistanceSquared(
	px: number, py: number, x0: number, y0: number, x1: number, y1: number,
): number {
	const dx = x1 - x0;
	const dy = y1 - y0;
	const length = dx * dx + dy * dy;

	// A segment of no length is a dot, and the distance to it is the distance
	// to the point.
	let t = 0;
	if (length > 0) {
		t = Math.min(1, Math.max(0, ((px - x0) * dx + (py - y0) * dy) / length));
	}

	const nx = px - (x0 + t * dx);
	const ny = py - (y0 + t * dy);

	return nx * nx + ny * ny;
}

// measure works out what a shape's labels say and where they sit, and hands
// each to `add`. It is the one place the conventions live.
//
// WHAT EACH SHAPE SAYS IS THE NUMBER THE SPELL IS WRITTEN WITH, which is the
// whole reason the labels exist:
//
//   Circle -- the RADIUS, because a spell is written "20-foot radius sphere".
//   Cone   -- the LENGTH from the point to the base, which by coneCorners is
//             also how wide it is at the far end. A thirty-foot cone reads
//             "30 ft.", which is the number on the spell.
//   Rect   -- one per axis, because a wall or a room is two measurements.
//
// A PEN STROKE SAYS NOTHING. A freehand squiggle has no distance anybody asked
// for, and a label per line would bury the table in text.
//
// EVERY NUMBER IS A STRAIGHT LINE AND NOT A COUNT OF SQUARES. feetBetween is
// Pythagoras with the diagonal rule ignored, which is what a radius and a
// wall's length are; cellsMoved is for a creature walking through cells and is
// a different question. See path.ts.
export function measure(
	kind: StrokeKind, points: readonly number[], grid: Grid, color: string,
	add: (text: string, x: number, y: number, color: string) => void,
): void {
	if (points.length < 4) {
		return;
	}

	// ONE ANCHOR RULE FOR EVERY SINGLE-NUMBER SHAPE: horizontally centred on the
	// shape and at the TOP of it, whichever way round it was drawn. The pass
	// lifts a label above whatever point it is given, so this puts the number
	// just clear of the ink at every angle -- which a cone needs and a circle
	// gets for free, since a circle's top is the same place however it was
	// dragged.
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

	// ONE PER AXIS AND BOTH HORIZONTAL. The atlas has no rotated text and the
	// pass draws none, so the height's number sits at the middle of the left
	// edge rather than running down it.
	if (x1 > x0) {
		add(distanceLabel(feetBetween(x1 - x0, 0, grid)), (x0 + x1) / 2, y0, color);
	}
	if (y1 > y0) {
		add(distanceLabel(feetBetween(0, y1 - y0, grid)), x0, (y0 + y1) / 2, color);
	}
}

function blankLabel(): Label {
	return { text: "", x: 0, y: 0, color: [1, 1, 1], alpha: 1 };
}

export function createDraw(deps: DrawDeps): Draw {
	// The line in hand: the local copy, which is what is drawn until the server
	// says it is finished.
	let local: Stroke | null = null;

	// Points added since the last chunk went out, and when that was.
	let pending: number[] = [];
	let sentAt = 0;

	// step is the decimation threshold in map pixels, read once per gesture.
	// Reading it per point would let a zoom mid-stroke change what counts as
	// movement halfway along one line.
	let step = 1;

	// rubbing is the set of ids one sweep of the eraser has crossed, or null
	// when the eraser is not down. ONE COMMAND PER SWEEP and not one per line:
	// a drag across a sketch crosses a dozen strokes, and a dozen erases would
	// be a dozen broadcasts and a dozen re-renders on every screen.
	let rubbing: Set<string> | null = null;

	// pointer is where the eraser's ring sits: the last place the pointer was
	// seen over the table, or null when it has left.
	let pointer: Point | null = null;

	// The eraser's ring, reused. It is read on every frame the tool is chosen.
	const ring: Outline = {
		x: 0, y: 0, halfW: 0, halfH: 0,
		color: ERASE_COLOR, alpha: 0.9, thickness: ERASE_WIDTH,
		rect: false, rotation: 0,
	};

	// Scratch for the hit test's segments, so a sweep allocates nothing.
	const segments: number[] = [];

	// shaping is the rectangle or circle under the hand, or null. Unlike a pen
	// stroke it has NOT left this browser: a shape is one message sent when the
	// button comes up, so abandoning one sends nothing at all.
	let shaping: { kind: Shaped; x0: number; y0: number; x1: number; y1: number } | null = null;

	// Scratch for the cone's three corners, so the preview allocates nothing
	// beyond the segments themselves.
	const corners: number[] = [];

	// The shape's preview, reused, and the tuple its colour is written into.
	//
	// THE TUPLE IS HELD SEPARATELY BECAUSE Outline.color IS READONLY, and it has
	// to be mutable here: pawns.ts copies the reference into its own slot and
	// the renderer re-reads it every frame, so replacing the tuple would leave
	// last frame's colour in the slot until the next Object.assign.
	const previewColor: [number, number, number] = [1, 1, 1];
	const preview: Outline = {
		x: 0, y: 0, halfW: 0, halfH: 0,
		color: previewColor, alpha: 0.95, thickness: PREVIEW_WIDTH,
		rect: false, rotation: 0,
	};

	// tint is the scratch parseColor writes into, shared by the preview and the
	// labels because neither is on screen while the other is being built.
	const tint = new Float32Array(4);

	// paintPreview reads the pill's colour into the preview, so the shape being
	// dragged is the colour it will land in.
	function paintPreview(): void {
		parseColor(deps.options().color, tint);
		previewColor[0] = tint[0];
		previewColor[1] = tint[1];
		previewColor[2] = tint[2];
	}

	// erasable is which strokes this viewer may rub out, and it is the client
	// half of a rule the server enforces in StrokeErase: the GM may take out
	// anybody's line and everybody else only their own.
	//
	// FILTERING HERE IS NOT TRUSTING THE CLIENT, it is the difference between an
	// eraser that passes over somebody else's line and one that asks to remove
	// it and is refused with an alert modal. The refusal is what makes the rule
	// true; this is what makes the tool feel like a tool.
	//
	// AND AN UNFINISHED LINE IS NOBODY'S TO ERASE, including its author's.
	// Somebody is still drawing it -- the points are arriving in chunks -- and
	// yanking it out from under a hand mid-stroke would leave that hand's next
	// chunk refused with "Stroke gone". Lifting the pen is one gesture away.
	function erasable(stroke: Stroke): boolean {
		return stroke.done
			&& stroke.layerId === deps.viewed()
			&& (deps.role === "gm" || stroke.by === deps.user);
	}

	// radius is the eraser's reach in map pixels: a fixed size on the screen,
	// converted at the zoom it is being aimed at.
	function radius(): number {
		return ERASE_RADIUS * deps.scale();
	}

	// rub adds every erasable stroke under the point to the sweep.
	function rub(map: Point): void {
		if (!rubbing) {
			return;
		}

		const reach = radius();
		for (const stroke of deps.state.strokes) {
			if (rubbing.has(stroke.id) || !erasable(stroke)) {
				continue;
			}
			if (strokeHit(stroke, map.x, map.y, reach, segments)) {
				rubbing.add(stroke.id);
			}
		}
	}

	function flush(): void {
		if (!local || pending.length === 0) {
			return;
		}

		deps.send({ type: "stroke.extend", id: local.id, points: pending });
		pending = [];
		sentAt = performance.now();
	}

	// keep decides whether a point is far enough from the last one to be worth
	// a point. The threshold is one CSS pixel's worth of map, never less than
	// one map pixel -- because the wire carries integers and two points that
	// round to the same place are one point.
	function keep(x: number, y: number): boolean {
		if (!local) {
			return false;
		}

		const n = local.points.length;
		const dx = x - local.points[n - 2];
		const dy = y - local.points[n - 1];

		return dx * dx + dy * dy >= step * step;
	}

	function add(x: number, y: number): void {
		if (!local) {
			return;
		}

		local.points.push(x, y);
		pending.push(x, y);

		if (pending.length >= CHUNK_POINTS * 2 || performance.now() - sentAt >= CHUNK_MS) {
			flush();
		}

		deps.invalidate();
	}

	// finish sends the end and drops the local copy. From here the store's copy
	// is what draws, which is the same points and one fewer thing to keep in
	// step.
	function finish(): void {
		if (!local) {
			return;
		}

		flush();
		deps.send({ type: "stroke.end", id: local.id });
		local = null;
		deps.invalidate();
	}

	// place sends the finished shape, or refuses one that has no size.
	//
	// ONE COMMAND AND NOT TWO. A shape arrives Done -- StrokeBegin.Apply marks
	// every kind but free finished the moment it exists -- so there is no
	// stroke.end to send, and sending one would be a second broadcast per shape
	// that told everybody something they already knew.
	//
	// A RECTANGLE NEEDS BOTH AXES AND A CIRCLE NEEDS A RADIUS, and the test is
	// on the ROUNDED coordinates because those are what go on the wire: a drag
	// of half a pixel rounds to the same place and would send a shape nobody
	// could see or point at to rub out. The server refuses a zero-size shape as
	// well; this is what keeps the refusal off the screen.
	function place(shape: { kind: Shaped; x0: number; y0: number; x1: number; y1: number }): void {
		const layer = deps.viewed();
		if (layer === "") {
			return;
		}

		if (shape.kind === "rect") {
			if (shape.x0 === shape.x1 || shape.y0 === shape.y1) {
				return;
			}
		} else if (shape.x0 === shape.x1 && shape.y0 === shape.y1) {
			return;
		}

		const { color, width } = deps.options();

		deps.send({
			type: "stroke.begin",
			id: ulid(),
			layer,
			kind: shape.kind,
			color,
			width,
			// A RECTANGLE IS NORMALISED ON THE WAY OUT, so one dragged up and to
			// the left is the same four integers as one dragged down and to the
			// right. A circle is not: its first point is the CENTRE and its
			// second is on the rim, and swapping them would turn it inside out.
			points: shape.kind === "rect"
				? [
					Math.min(shape.x0, shape.x1), Math.min(shape.y0, shape.y1),
					Math.max(shape.x0, shape.x1), Math.max(shape.y0, shape.y1),
				]
				: [shape.x0, shape.y0, shape.x1, shape.y1],
		});
	}

	function abandon(): boolean {
		// A SWEEP DROPPED PART-WAY SENDS NOTHING, which is the opposite of what
		// abandoning a line does and is right for the same reason: nothing has
		// left this browser yet, so there is nothing on anybody else's table to
		// take back.
		if (rubbing) {
			rubbing = null;
			deps.invalidate();

			return true;
		}

		// AND SO DOES A SHAPE, for the same reason: it is one message sent on
		// release, so a shape dropped part-way was never anywhere but here.
		if (shaping) {
			shaping = null;
			deps.invalidate();

			return true;
		}

		if (!local) {
			return false;
		}

		const id = local.id;
		finish();
		deps.send({ type: "stroke.erase", ids: [id] });

		return true;
	}

	// undo takes back this viewer's newest finished line on the floor they are
	// looking at.
	//
	// IT IS THE VIEWER'S OWN AND NOT THE FLOOR'S NEWEST, even for the GM. Ctrl+Z
	// means "take back what I just did", and a GM whose undo removed the line a
	// player drew a second earlier would have a key that reaches across the
	// table.
	//
	// THE STORE IS SORTED BY ID and ids are minted in order, so the newest match
	// is the last one -- which is why ulid.ts is the monotonic variant.
	function undo(): boolean {
		const strokes = deps.state.strokes;

		for (let i = strokes.length - 1; i >= 0; i--) {
			const stroke = strokes[i];
			if (!stroke.done || stroke.layerId !== deps.viewed() || stroke.by !== deps.user) {
				continue;
			}

			deps.send({ type: "stroke.erase", ids: [stroke.id] });

			return true;
		}

		return false;
	}

	return {
		press(map, mods) {
			if (!deps.drawing()) {
				return false;
			}

			const layer = deps.viewed();
			if (layer === "") {
				return false;
			}

			// A SECOND PRESS WHILE A LINE IS IN HAND FINISHES THE FIRST, and it
			// is asked before the mode is, because the line has to be closed
			// whichever mode this press turns out to be. It takes a browser
			// losing a pointerup to get here -- a context menu on another
			// window, a drag that left the tab -- and the answer is to close
			// what is open rather than to leave a stroke nothing will ever end.
			finish();

			pointer = { x: map.x, y: map.y };

			// THE ERASER IS A MODE OF THIS TOOL AND NOT A TOOL OF ITS OWN,
			// which is what lets a hand switch between drawing and rubbing out
			// without leaving the pill's second row.
			const mode = deps.options().mode;

			if (mode === "erase") {
				rubbing = new Set();
				rub(map);
				deps.invalidate();

				return true;
			}

			// A SHAPE IS DRAGGED OUT AND SENT WHOLE. Nothing leaves the browser
			// until the button comes up, which is why Escape and the right
			// button abandon one for free -- and why the other people at the
			// table see it appear complete rather than growing. See
			// room.StrokeBegin.
			if (SHAPES.includes(mode)) {
				shaping = {
					kind: mode as Shaped,
					x0: Math.round(map.x), y0: Math.round(map.y),
					x1: Math.round(map.x), y1: Math.round(map.y),
				};
				deps.invalidate();

				return true;
			}

			const { color, width } = deps.options();
			const x = Math.round(map.x);
			const y = Math.round(map.y);

			step = Math.max(1, deps.scale());

			// THE ECHO OF THIS COMMAND IS IGNORED WHILE THE LINE IS IN HAND.
			// stroke.began and stroke.extended come back to their own author
			// like everybody else's, and the store applies them -- but the
			// renderer draws inHand() instead for as long as it is not null, so
			// the line under the pen is the hand's own points rather than the
			// hand's points a round trip ago.
			local = {
				id: ulid(),
				by: "",
				layerId: layer,
				kind: "free",
				color,
				width,
				points: [x, y],
				done: false,
			};

			pending = [];
			sentAt = performance.now();

			deps.send({
				type: "stroke.begin",
				id: local.id,
				layer,
				kind: "free",
				color,
				width,
				points: [x, y],
			});

			deps.invalidate();

			return true;
		},

		drag(map, mods) {
			pointer = { x: map.x, y: map.y };

			if (rubbing) {
				rub(map);
				deps.invalidate();

				return;
			}

			if (shaping) {
				shaping.x1 = Math.round(map.x);
				shaping.y1 = Math.round(map.y);
				deps.invalidate();

				return;
			}

			if (!local) {
				return;
			}

			const x = Math.round(map.x);
			const y = Math.round(map.y);
			if (!keep(x, y)) {
				return;
			}

			add(x, y);
		},

		release(map, mods) {
			pointer = { x: map.x, y: map.y };

			if (rubbing) {
				rub(map);

				const ids = [...rubbing];
				rubbing = null;
				if (ids.length > 0) {
					deps.send({ type: "stroke.erase", ids });
				}
				deps.invalidate();

				return;
			}

			if (shaping) {
				shaping.x1 = Math.round(map.x);
				shaping.y1 = Math.round(map.y);

				const done = shaping;
				shaping = null;
				deps.invalidate();
				place(done);

				return;
			}

			if (!local) {
				return;
			}

			// THE LAST POINT IS KEPT WHATEVER THE THRESHOLD SAYS, so a line ends
			// where the hand lifted rather than at the last sample that happened
			// to be far enough from the one before it.
			const x = Math.round(map.x);
			const y = Math.round(map.y);
			const n = local.points.length;
			if (local.points[n - 2] !== x || local.points[n - 1] !== y) {
				add(x, y);
			}

			finish();
		},

		secondary: abandon,

		// ABANDONING A LINE IS ENDING IT AND THEN RUBBING IT OUT, and it cannot
		// be anything else: stroke.begin went out on the press, so the line is
		// already on every other screen at the table and there is nothing to
		// un-begin. Ending it first is what makes the erase legal for a player,
		// whose own stroke this is.
		//
		// It is Escape's and the right button's, and it is the only gesture on
		// this table that undoes something by sending two commands.
		abandon,

		// CTRL+Z BELONGS TO THIS TOOL WHILE IT IS CHOSEN and to nothing at all
		// otherwise, which is the rule fog.ts follows with the same three keys:
		// answering false is how the page keeps its own undo.
		key(e) {
			if (!deps.drawing() || typing(e.target)) {
				return false;
			}

			if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "z") {
				// SPENT WHETHER OR NOT THERE WAS ANYTHING TO UNDO. A press with
				// no line left is the end of a run of them, and letting it fall
				// through to the browser at exactly that moment would be an
				// undo that suddenly did something else.
				undo();

				return true;
			}

			return false;
		},

		hover(map) {
			pointer = map ? { x: map.x, y: map.y } : null;
		},

		// The shape under the hand, or the eraser's reach. Never both: each
		// belongs to a different mode, so one reused object serves them.
		//
		// THE ERASER SHOWS ITS REACH because the radius is a few pixels on the
		// screen and nothing else on the table says where it ends -- without the
		// ring the tool is aimed by guessing.
		outline() {
			if (!deps.drawing()) {
				return null;
			}

			if (shaping) {
				// A CONE IS THREE LINES AND NOT A RING, so it declines here and
				// is drawn through marks() below.
				if (shaping.kind === "cone") {
					return null;
				}

				paintPreview();

				if (shaping.kind === "rect") {
					// A RECTANGLE OF NO WIDTH IS NOT DRAWN, which is the
					// marquee's rule: the first pixel of a drag is a box that
					// has not left its first corner.
					const halfW = Math.abs(shaping.x1 - shaping.x0) / 2;
					const halfH = Math.abs(shaping.y1 - shaping.y0) / 2;
					if (halfW <= 0 || halfH <= 0) {
						return null;
					}

					preview.x = (shaping.x0 + shaping.x1) / 2;
					preview.y = (shaping.y0 + shaping.y1) / 2;
					preview.halfW = halfW;
					preview.halfH = halfH;
					preview.rect = true;

					return preview;
				}

				const r = Math.hypot(shaping.x1 - shaping.x0, shaping.y1 - shaping.y0);
				if (r <= 0) {
					return null;
				}

				preview.x = shaping.x0;
				preview.y = shaping.y0;
				preview.halfW = r;
				preview.halfH = r;
				preview.rect = false;

				return preview;
			}

			if (deps.options().mode !== "erase" || !pointer) {
				return null;
			}

			const reach = radius();
			ring.x = pointer.x;
			ring.y = pointer.y;
			ring.halfW = reach;
			ring.halfH = reach;

			return ring;
		},

		// THE CONE UNDER THE HAND, as its three sides. It goes through the same
		// pass the fog's half-drawn polygon does, which puts it over everything
		// on the table -- a preview is a mark ON the table rather than a thing
		// on it, and it has to be visible against whatever it is being aimed at.
		marks(out) {
			if (!deps.drawing() || shaping?.kind !== "cone") {
				return out;
			}

			coneCorners(shaping.x0, shaping.y0, shaping.x1, shaping.y1, corners);
			if (corners.length === 0) {
				return out;
			}

			paintPreview();

			for (let i = 0; i < 3; i++) {
				const j = (i + 1) % 3;
				out.push({
					x0: corners[i * 2], y0: corners[i * 2 + 1],
					x1: corners[j * 2], y1: corners[j * 2 + 1],
					color: previewColor, alpha: 0.95, width: PREVIEW_WIDTH,
				});
			}

			return out;
		},

		// THE DISTANCE ACROSS EVERY SHAPE, and it is recomputed per frame rather
		// than stored. The grid is what turns map pixels into feet and a GM can
		// retune it mid-session, so a cached number would be the right answer to
		// last week's cell size. Forty short strings a frame is nursery garbage;
		// a cache keyed on the grid would have to be re-validated every frame
		// anyway, which is the work it was meant to save.
		labels(out) {
			let count = 0;

			const add = (text: string, x: number, y: number, color: string): void => {
				const slot = out[count] ?? (out[count] = blankLabel());
				parseColor(color, tint);

				slot.text = text;
				slot.x = x;
				slot.y = y;
				slot.color[0] = tint[0];
				slot.color[1] = tint[1];
				slot.color[2] = tint[2];
				slot.alpha = 1;
				count++;
			};

			const grid = deps.grid();
			const viewed = deps.viewed();

			for (const stroke of deps.state.strokes) {
				if (stroke.layerId !== viewed) {
					continue;
				}

				measure(stroke.kind, stroke.points, grid, stroke.color, add);
			}

			// AND THE SHAPE IN HAND, which is the whole point of the number: a
			// GM drags until it reads thirty feet and lets go.
			if (shaping) {
				measure(
					shaping.kind,
					[shaping.x0, shaping.y0, shaping.x1, shaping.y1],
					grid, deps.options().color, add,
				);
			}

			out.length = count;

			return out;
		},

		inHand() {
			return local;
		},
	};
}
