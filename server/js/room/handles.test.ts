// The geometry of the eight boxes and the one circle: where they are, which one
// a hand has hold of, and what dragging one asks for.
//
// EVERYTHING HERE IS ABOUT THE CENTRE, and that is what most of these tests are
// really pinning. A resize that anchored the opposite corner would be a change
// of size AND position, and position is another command with another authority
// check -- so a gesture that got half of what it asked for would leave a token
// somewhere nobody dragged it.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { Handle } from "./handles.ts";
import type { Placed } from "./render/path.ts";
import { HANDLE_GRAB, OBJECT_PIXELS_MAX, SPIN_GAP, SPIN_STEP, handleAt, handlesFor, resized, turned } from "./handles.ts";

const CELL = 64;

function token(over: Partial<Placed> = {}): Placed {
	return {
		kind: "object",
		size: "medium",
		width: 128,
		height: 256,
		rotation: 0,
		x: 0,
		y: 0,
		...over,
	};
}

function at(handles: readonly Handle[], lx: number, ly: number): Handle | undefined {
	return handles.find((h) => h.lx === lx && h.ly === ly && !h.turns);
}

test("a token has eight resize handles on its own edges and one below them", () => {
	const handles = handlesFor(token(), CELL, 1, []);

	assert.equal(handles.length, 9);
	assert.deepEqual([at(handles, 1, 1)?.x, at(handles, 1, 1)?.y], [64, 128]);
	assert.deepEqual([at(handles, -1, -1)?.x, at(handles, -1, -1)?.y], [-64, -128]);
	assert.deepEqual([at(handles, 1, 0)?.x, at(handles, 1, 0)?.y], [64, 0]);
	assert.deepEqual([at(handles, 0, -1)?.x, at(handles, 0, -1)?.y], [0, -128]);

	// THE ROTATE HANDLE IS BELOW THE TOKEN AND NOT ABOVE IT, because above is
	// where the pawn overlay goes -- and the overlay is showing exactly when the
	// handles are, so a handle up there would always be underneath it.
	const spinner = handles.find((h) => h.turns);
	assert.deepEqual([spinner?.x, spinner?.y], [0, 128 + SPIN_GAP]);
});

// THE GAP IS IN CSS PIXELS AND THE POSITIONS ARE IN MAP PIXELS, so zooming out
// pushes the rotate handle further from the token in table units and leaves it
// exactly where it was on screen.
test("the rotate handle keeps its distance on screen rather than on the table", () => {
	const close = handlesFor(token(), CELL, 1, []).find((h) => h.turns);
	const far = handlesFor(token(), CELL, 4, []).find((h) => h.turns);

	assert.equal(close?.y, 128 + SPIN_GAP);
	assert.equal(far?.y, 128 + SPIN_GAP * 4);
});

// A TURNED TOKEN'S HANDLES TRAVEL WITH IT, which is what keeps the rotate
// handle marking the same edge all the way round.
test("the handles turn with the token", () => {
	const handles = handlesFor(token({ rotation: 90 }), CELL, 1, []);

	// A quarter turn clockwise sends the right edge's handle to the bottom.
	const right = at(handles, 1, 0);
	assert.ok(right && Math.abs(right.x - 0) < 1e-9 && Math.abs(right.y - 64) < 1e-9, `right edge at ${right?.x},${right?.y}`);

	// A quarter turn sends the bottom edge, and the handle past it, to the left.
	const spinner = handles.find((h) => h.turns);
	assert.ok(spinner && Math.abs(spinner.x + (128 + SPIN_GAP)) < 1e-9, `rotate handle at ${spinner?.x}`);
});

// A creature has a size category out of the rules rather than a rectangle, so
// there is nothing to drag it to.
test("a creature has no handles", () => {
	assert.deepEqual(handlesFor(token({ kind: "monster" }), CELL, 1, []), []);
});

// THE TARGET IS BIGGER THAN THE BOX, because what is drawn is a mark a person
// aims at and what is tested is a square a hand can land in.
test("a handle is grabbed from a screen distance away", () => {
	const handles = handlesFor(token(), CELL, 1, []);

	assert.equal(handleAt(handles, 64, 128, 1)?.lx, 1, "the corner itself was missed");
	assert.equal(handleAt(handles, 64 + HANDLE_GRAB - 1, 128, 1)?.lx, 1);
	assert.equal(handleAt(handles, 64 + HANDLE_GRAB + 1, 128, 1), null);

	// Zoomed out, the same screen distance is more of the table.
	assert.ok(handleAt(handles, 64 + HANDLE_GRAB * 3, 128, 4));
});

// THE NEAREST RATHER THAN THE FIRST, so the answer does not depend on the order
// the handles were built in -- which matters on a token small enough that its
// eight resize handles overlap each other.
test("the nearest handle wins", () => {
	const handles = handlesFor(token({ width: 8, height: 8 }), CELL, 1, []);
	const grabbed = handleAt(handles, 4, 4, 1);

	assert.deepEqual([grabbed?.lx, grabbed?.ly], [1, 1]);
});

// AN EDGE HANDLE LEAVES THE OTHER AXIS EXACTLY ALONE, including its rounding: a
// GM stretching a wagon lengthwise must not find it a pixel narrower.
test("an edge handle scales one axis and a corner scales both", () => {
	const wagon = token();
	const handles = handlesFor(wagon, CELL, 1, []);

	const right = at(handles, 1, 0) as Handle;
	assert.deepEqual(resized(wagon, right, 100, 999, CELL, false), [200, 256]);

	const bottom = at(handles, 0, 1) as Handle;
	assert.deepEqual(resized(wagon, bottom, 999, 50, CELL, false), [128, 100]);

	const corner = at(handles, 1, 1) as Handle;
	assert.deepEqual(resized(wagon, corner, 100, 50, CELL, false), [200, 100]);
});

// PULLING PAST THE CENTRE IS A DISTANCE AND NOT A NEGATIVE SIZE. The half
// extent is |offset|, so dragging the right edge through the middle and out the
// other side grows the token rather than inverting it.
test("dragging a handle through the centre grows rather than inverts", () => {
	const wagon = token();
	const right = at(handlesFor(wagon, CELL, 1, []), 1, 0) as Handle;

	assert.deepEqual(resized(wagon, right, -100, 0, CELL, false), [200, 256]);
});

// SHIFT ON A CORNER KEEPS THE PICTURE'S PROPORTIONS, which is what somebody
// resizing a portrait wants and what somebody stretching a road does not. The
// factor is the LARGER of the two the hand asked for, so the token follows the
// pointer rather than lagging behind the axis that moved less.
test("shift on a corner keeps the aspect and shift on an edge does nothing", () => {
	const wagon = token();
	const handles = handlesFor(wagon, CELL, 1, []);

	const corner = at(handles, 1, 1) as Handle;
	assert.deepEqual(resized(wagon, corner, 128, 128, CELL, true), [256, 512]);

	// An edge has one axis to scale, so there is no ratio to keep.
	const right = at(handles, 1, 0) as Handle;
	assert.deepEqual(resized(wagon, right, 100, 0, CELL, true), [200, 256]);
});

// A RESIZE ON A TURNED TOKEN IS ALONG THE TOKEN'S OWN AXES. Dragging the right
// edge of a token turned a quarter turn widens it along what is now the screen's
// vertical, because that is where its width went.
test("resizing a turned token measures along its own axes", () => {
	const upright = token({ rotation: 90 });
	const right = at(handlesFor(upright, CELL, 1, []), 1, 0) as Handle;

	assert.deepEqual(resized(upright, right, 0, 100, CELL, false), [200, 256]);
});

// The core refuses anything outside this and a drag that ran past it would be
// answered with an alert modal in the middle of a gesture.
test("a resize is clamped to what the protocol accepts", () => {
	const wagon = token();
	const corner = at(handlesFor(wagon, CELL, 1, []), 1, 1) as Handle;

	assert.deepEqual(resized(wagon, corner, 0, 0, CELL, false), [1, 1]);
	assert.deepEqual(
		resized(wagon, corner, OBJECT_PIXELS_MAX, OBJECT_PIXELS_MAX, CELL, false),
		[OBJECT_PIXELS_MAX, OBJECT_PIXELS_MAX],
	);
});

// THE HANDLE MARKS THE BOTTOM EDGE, so the angle is measured from straight down
// rather than from the x axis atan2 answers in. A quarter turn clockwise takes
// the bottom of the token round to the left, which is where the hand drags to.
test("the rotate handle reads its angle from straight down", () => {
	const wagon = token();

	assert.equal(turned(wagon, 0, 100, 0), 0);
	assert.equal(turned(wagon, -100, 0, 0), 90);
	assert.equal(turned(wagon, 0, -100, 0), 180);
	assert.equal(turned(wagon, 100, 0, 0), 270);
});

// AN ANGLE IS FOLDED RATHER THAN CLAMPED, because -30 and 330 are the same
// facing -- which is the rule normalizeRotation applies on the server too.
test("an angle comes back inside one turn", () => {
	const wagon = token();

	// A hair anticlockwise of straight down is 359 rather than -1.
	assert.equal(turned(wagon, 1, 100, 0), 359);
	assert.equal(turned(wagon, 100, 100, 0), 315);
});

// Fifteen degrees gives back every multiple of forty-five and every right
// angle, which is the only way to get a token exactly square by hand.
test("shift steps the rotation", () => {
	const wagon = token();

	assert.equal(turned(wagon, -100, 4, SPIN_STEP), 90);
	assert.equal(turned(wagon, -100, 100, SPIN_STEP), 45);
	assert.equal(SPIN_STEP, 15);

	// The step is taken before the fold, so a hand a little anticlockwise of
	// straight down snaps to 270 rather than to a negative ninety.
	assert.equal(turned(wagon, 100, 4, SPIN_STEP), 270);
});
