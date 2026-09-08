// The eight boxes and the one circle that resize and turn a token.
//
// THIS IS AN OBJECT'S AFFORDANCE AND NOBODY ELSE'S. A creature is a disc whose
// size is a category out of the rules -- medium is one cell, large is two -- so
// there is nothing to drag it to; a token is a picture somebody laid on the
// floor at whatever size and angle the map wants, and the only way to line a
// road up with the road under it is to take hold of it.
//
// EVERYTHING SCALES ABOUT THE CENTRE, resizing included, and that is the one
// choice here worth arguing. An image editor anchors the opposite corner, which
// makes a resize a change of SIZE AND POSITION together -- and a position is
// PawnMove's, snapped, authorised and echoed, while a size is PawnUpdate's. One
// gesture would have to send two commands that can be refused independently,
// and a client that got half of what it asked for would leave the token
// somewhere nobody dragged it. Scaling about the centre keeps the whole gesture
// inside one idempotent command, and it means the rotate handle and the resize
// handles share an origin instead of fighting over where the thing IS.
//
// THE HANDLES ARE A FIXED SIZE ON SCREEN and everything else here is in map
// pixels, which is why almost every function takes mapPerPixel. A handle that
// scaled with the camera would be a speck on a zoomed-out table and a slab on a
// zoomed-in one, and it is a target for a hand either way.
//
// NOTHING HERE TOUCHES A PAWN. Every function is a question answered from a
// pawn and a pointer; what to do about the answer is pawns.ts's, and what it
// does is preview locally and send one PawnUpdate on release.

import type { Placed } from "./render/path.ts";
import type { Point } from "./render/camera.ts";
import { pawnExtents, spin, unrotate } from "./render/path.ts";

// HANDLE_HALF is how big a handle is drawn, and HANDLE_GRAB how close a pointer
// has to be to take hold of one -- both in CSS pixels, both half-extents.
//
// THE TARGET IS BIGGER THAN THE BOX, which is the ordinary rule for a small
// control: what is drawn is a mark a person aims at and what is tested is a
// square a hand can actually land in. Ten CSS pixels is about three millimetres.
export const HANDLE_HALF = 4;
export const HANDLE_GRAB = 10;

// SPIN_GAP is how far the rotate handle floats past the BOTTOM edge, in CSS
// pixels, so it is never sitting on the corner handles beside it.
//
// BELOW AND NOT ABOVE, WHICH IS WHERE EVERY OTHER EDITOR PUTS IT. Above the
// token is already taken: the pawn overlay -- the name, the hit points, the
// conditions and the Edit button -- is placed on the top of the same box and
// lifted eight pixels off it, and the handles are only ever up when exactly one
// pawn is selected, which is exactly when that overlay is showing. A rotate
// handle above the edge is therefore not occasionally covered, it is always
// covered. Below the token is empty.
export const SPIN_GAP = 26;

// SPIN_STEP is the angle Shift snaps a rotation to. Fifteen degrees gives back
// every multiple of 45 and every right angle, which is the only way to get a
// token exactly square again once it has been turned by hand.
export const SPIN_STEP = 15;

// OBJECT_PIXELS_MAX is room.ObjectPixelsMax: what the core accepts on one axis.
// It is written here rather than imported because the generator emits types and
// not limits, and a drag that ran past it would be answered with an alert modal
// in the middle of a gesture. A Go test pins the two together.
export const OBJECT_PIXELS_MAX = 8_192;

// Handle is one control, in map pixels, plus which way dragging it scales.
//
// lx AND ly ARE THE CORNER IN THE OBJECT'S OWN FRAME, each -1, 0 or 1: (1, 0)
// is the middle of the right edge and scales the width alone, (-1, -1) is the
// top-left corner and scales both. They are the handle's identity as well as
// its position, which is what lets a gesture hold on to one across a drag while
// the token under it changes shape.
export interface Handle {
	lx: number;
	ly: number;

	// turns marks the one handle that rotates instead of resizing.
	turns: boolean;

	x: number;
	y: number;

	// rotation is the pawn's own, so the box is drawn square to the token
	// rather than square to the screen.
	rotation: number;
}

// CORNERS FIRST, THEN EDGES, and the rotate handle last. The order is only a
// tie-break -- handleAt takes the NEAREST rather than the first -- but two
// handles are exactly equidistant often enough on a small token that the tie
// wants a stable answer, and a corner is the more useful of the two.
const OFFSETS: readonly (readonly [number, number])[] = [
	[-1, -1], [1, -1], [1, 1], [-1, 1],
	[0, -1], [1, 0], [0, 1], [-1, 0],
];

const local: Point = { x: 0, y: 0 };

// handlesFor is where a token's controls are, in map pixels. It writes into a
// reused array, because it is read on every frame a selection is up.
//
// AN EMPTY LIST IS THE ANSWER FOR ANYTHING THAT IS NOT AN OBJECT, so no caller
// has to ask twice.
export function handlesFor(
	pawn: Placed,
	cellSize: number,
	mapPerPixel: number,
	out: Handle[],
): Handle[] {
	if (pawn.kind !== "object") {
		out.length = 0;

		return out;
	}

	const [halfW, halfH] = pawnExtents(pawn, cellSize);
	let count = 0;

	const put = (lx: number, ly: number, ox: number, oy: number, turns: boolean): void => {
		spin(pawn.rotation, ox, oy, local);

		const slot = out[count] ?? (out[count] = { lx: 0, ly: 0, turns: false, x: 0, y: 0, rotation: 0 });

		slot.lx = lx;
		slot.ly = ly;
		slot.turns = turns;
		slot.x = pawn.x + local.x;
		slot.y = pawn.y + local.y;
		slot.rotation = pawn.rotation;

		count++;
	};

	for (const [lx, ly] of OFFSETS) {
		put(lx, ly, lx * halfW, ly * halfH, false);
	}

	// THE ROTATE HANDLE IS BELOW THE BOTTOM EDGE IN THE TOKEN'S OWN FRAME, so it
	// travels round with the token and always marks the same edge. Its gap is a
	// screen distance rather than a fraction of the token, or it would be
	// inside a tall thin one and a field away from a small one.
	put(0, 1, 0, halfH + SPIN_GAP * mapPerPixel, true);

	out.length = count;

	return out;
}

// handleAt is which control a point has hold of, or null.
//
// THE NEAREST RATHER THAN THE FIRST, so the answer does not depend on the order
// they were built in -- which matters on a token small enough that its eight
// resize handles overlap each other.
export function handleAt(
	handles: readonly Handle[],
	x: number,
	y: number,
	mapPerPixel: number,
): Handle | null {
	const reach = HANDLE_GRAB * mapPerPixel;
	const limit = reach * reach;

	let best: Handle | null = null;
	let nearest = Infinity;

	for (const handle of handles) {
		const dx = x - handle.x;
		const dy = y - handle.y;
		const distance = dx * dx + dy * dy;

		if (distance <= limit && distance < nearest) {
			best = handle;
			nearest = distance;
		}
	}

	return best;
}

// resized is the width and height a resize handle is asking for, in map pixels.
//
// THE POINTER IS TAKEN INTO THE TOKEN'S FRAME FIRST, which is what makes
// dragging the right edge of a token turned forty degrees widen it along its
// OWN width rather than along the screen's. The distance from the centre is
// then half the new size, because the centre is the anchor.
//
// AN EDGE HANDLE LEAVES THE OTHER AXIS EXACTLY ALONE, including its rounding: a
// GM stretching a wagon lengthwise must not find it a pixel narrower than it
// was.
export function resized(
	pawn: Placed,
	handle: Handle,
	x: number,
	y: number,
	cellSize: number,
	lockAspect: boolean,
): [number, number] {
	const [halfW, halfH] = pawnExtents(pawn, cellSize);
	const fromW = halfW * 2;
	const fromH = halfH * 2;

	unrotate(pawn.rotation, x - pawn.x, y - pawn.y, local);

	let width = fromW;
	let height = fromH;

	if (handle.lx !== 0) {
		width = clamp(Math.round(Math.abs(local.x) * 2));
	}
	if (handle.ly !== 0) {
		height = clamp(Math.round(Math.abs(local.y) * 2));
	}

	// SHIFT ON A CORNER KEEPS THE PICTURE'S PROPORTIONS, which is what somebody
	// resizing a portrait wants and what somebody stretching a road does not.
	// The factor is the LARGER of the two the hand asked for, so the token
	// follows the pointer rather than lagging behind the axis that moved less.
	if (lockAspect && handle.lx !== 0 && handle.ly !== 0) {
		const factor = Math.max(width / fromW, height / fromH);

		width = clamp(Math.round(fromW * factor));
		height = clamp(Math.round(fromH * factor));
	}

	return [width, height];
}

// turned is the angle the rotate handle is asking for, in whole degrees
// clockwise, folded into [0, 360).
//
// THE HANDLE MARKS THE BOTTOM EDGE, so the angle is measured from straight down
// rather than from the x axis that atan2 answers in -- hence the ninety, and
// hence it being subtracted rather than added.
//
// step IS THE SHIFT KEY. Zero is free rotation; SPIN_STEP is what gets a token
// exactly square again, which free rotation by hand never quite does.
export function turned(pawn: Placed, x: number, y: number, step: number): number {
	const degrees = (Math.atan2(y - pawn.y, x - pawn.x) * 180) / Math.PI - 90;
	const stepped = step > 0 ? Math.round(degrees / step) * step : Math.round(degrees);

	return ((stepped % 360) + 360) % 360;
}

function clamp(pixels: number): number {
	return Math.min(Math.max(pixels, 1), OBJECT_PIXELS_MAX);
}
