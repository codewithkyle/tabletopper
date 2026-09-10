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

import type { Role, State, Stroke, StrokeKind } from "./protocol.ts";
import type { Outgoing } from "./socket.ts";
import type { Modifiers } from "./render/input.ts";
import type { Outline } from "./pawns.ts";
import type { Point } from "./render/camera.ts";
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
export type DrawMode = "pen" | "erase";

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

	// outline is the eraser's cursor, or null. It is ONE reused object for the
	// reason everything else on the frame path is.
	outline(): Outline | null;

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

		default:
			// The three shapes are checkpoints 4 and 5. A kind with no expansion
			// draws nothing rather than throwing: this runs inside a frame, and
			// a stroke from a build ahead of this one is not worth a black
			// table.
			return out;
	}
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
			if (deps.options().mode === "erase") {
				rubbing = new Set();
				rub(map);
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

		// THE ERASER SHOWS ITS REACH. The radius is a few pixels on the screen
		// and there is nothing else on the table that says where it ends, so
		// without the ring the tool is aimed by guessing.
		outline() {
			if (!deps.drawing() || deps.options().mode !== "erase" || !pointer) {
				return null;
			}

			const reach = radius();
			ring.x = pointer.x;
			ring.y = pointer.y;
			ring.halfW = reach;
			ring.halfH = reach;

			return ring;
		},

		inHand() {
			return local;
		},
	};
}
