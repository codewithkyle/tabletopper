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

import type { Stroke, StrokeKind } from "./protocol.ts";
import type { Outgoing } from "./socket.ts";
import type { Modifiers } from "./render/input.ts";
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

// DEFAULT_WIDTH is the pen until the options pill exists to change it. Four map
// pixels is a pen line on a seventy-pixel cell.
export const DEFAULT_WIDTH = 4;

// DrawOptions is what the tool draws with. It is asked for rather than pushed,
// and it is read when a stroke STARTS rather than when it finishes -- which is
// the opposite of the fog's rule and is right for the opposite reason: a fog
// shape is one message sent at the end, and a stroke's colour is already on
// everybody's screen by the time the hand lifts.
export interface DrawOptions {
	color: string;
	width: number;
}

export interface DrawDeps {
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
		if (!local) {
			return false;
		}

		const id = local.id;
		finish();
		deps.send({ type: "stroke.erase", ids: [id] });

		return true;
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

			// A SECOND PRESS WHILE A LINE IS IN HAND FINISHES THE FIRST. It
			// takes a browser losing a pointerup to get here -- a context menu
			// on another window, a drag that left the tab -- and the answer is
			// to close what is open rather than to leave a stroke nothing will
			// ever end.
			finish();

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

		// The tool's keyboard. There is nothing on it yet -- Ctrl+Z arrives with
		// the eraser, which is what makes an undo worth having -- and it answers
		// false so that no key is taken from the page in the meantime.
		key(e) {
			if (!deps.drawing() || typing(e.target)) {
				return false;
			}

			return false;
		},

		inHand() {
			return local;
		},
	};
}
