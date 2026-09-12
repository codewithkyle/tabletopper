import assert from "node:assert/strict";
import { test } from "node:test";
import type { Handle } from "./model/overlay.ts";
import type { Placed } from "./model/shape.ts";
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
	const spinner = handles.find((h) => h.turns);
	assert.deepEqual([spinner?.x, spinner?.y], [0, 128 + SPIN_GAP]);
});
test("the rotate handle keeps its distance on screen rather than on the table", () => {
	const close = handlesFor(token(), CELL, 1, []).find((h) => h.turns);
	const far = handlesFor(token(), CELL, 4, []).find((h) => h.turns);
	assert.equal(close?.y, 128 + SPIN_GAP);
	assert.equal(far?.y, 128 + SPIN_GAP * 4);
});
test("the handles turn with the token", () => {
	const handles = handlesFor(token({ rotation: 90 }), CELL, 1, []);
	const right = at(handles, 1, 0);
	assert.ok(right && Math.abs(right.x - 0) < 1e-9 && Math.abs(right.y - 64) < 1e-9, `right edge at ${right?.x},${right?.y}`);
	const spinner = handles.find((h) => h.turns);
	assert.ok(spinner && Math.abs(spinner.x + (128 + SPIN_GAP)) < 1e-9, `rotate handle at ${spinner?.x}`);
});
test("a creature has no handles", () => {
	assert.deepEqual(handlesFor(token({ kind: "monster" }), CELL, 1, []), []);
});
test("a handle is grabbed from a screen distance away", () => {
	const handles = handlesFor(token(), CELL, 1, []);
	assert.equal(handleAt(handles, 64, 128, 1)?.lx, 1, "the corner itself was missed");
	assert.equal(handleAt(handles, 64 + HANDLE_GRAB - 1, 128, 1)?.lx, 1);
	assert.equal(handleAt(handles, 64 + HANDLE_GRAB + 1, 128, 1), null);
	assert.ok(handleAt(handles, 64 + HANDLE_GRAB * 3, 128, 4));
});
test("the nearest handle wins", () => {
	const handles = handlesFor(token({ width: 8, height: 8 }), CELL, 1, []);
	const grabbed = handleAt(handles, 4, 4, 1);
	assert.deepEqual([grabbed?.lx, grabbed?.ly], [1, 1]);
});
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
test("dragging a handle through the centre grows rather than inverts", () => {
	const wagon = token();
	const right = at(handlesFor(wagon, CELL, 1, []), 1, 0) as Handle;
	assert.deepEqual(resized(wagon, right, -100, 0, CELL, false), [200, 256]);
});
test("shift on a corner keeps the aspect and shift on an edge does nothing", () => {
	const wagon = token();
	const handles = handlesFor(wagon, CELL, 1, []);
	const corner = at(handles, 1, 1) as Handle;
	assert.deepEqual(resized(wagon, corner, 128, 128, CELL, true), [256, 512]);
	const right = at(handles, 1, 0) as Handle;
	assert.deepEqual(resized(wagon, right, 100, 0, CELL, true), [200, 256]);
});
test("resizing a turned token measures along its own axes", () => {
	const upright = token({ rotation: 90 });
	const right = at(handlesFor(upright, CELL, 1, []), 1, 0) as Handle;
	assert.deepEqual(resized(upright, right, 0, 100, CELL, false), [200, 256]);
});
test("a resize is clamped to what the protocol accepts", () => {
	const wagon = token();
	const corner = at(handlesFor(wagon, CELL, 1, []), 1, 1) as Handle;
	assert.deepEqual(resized(wagon, corner, 0, 0, CELL, false), [1, 1]);
	assert.deepEqual(
		resized(wagon, corner, OBJECT_PIXELS_MAX, OBJECT_PIXELS_MAX, CELL, false),
		[OBJECT_PIXELS_MAX, OBJECT_PIXELS_MAX],
	);
});
test("the rotate handle reads its angle from straight down", () => {
	const wagon = token();
	assert.equal(turned(wagon, 0, 100, 0), 0);
	assert.equal(turned(wagon, -100, 0, 0), 90);
	assert.equal(turned(wagon, 0, -100, 0), 180);
	assert.equal(turned(wagon, 100, 0, 0), 270);
});
test("an angle comes back inside one turn", () => {
	const wagon = token();
	assert.equal(turned(wagon, 1, 100, 0), 359);
	assert.equal(turned(wagon, 100, 100, 0), 315);
});
test("shift steps the rotation", () => {
	const wagon = token();
	assert.equal(turned(wagon, -100, 4, SPIN_STEP), 90);
	assert.equal(turned(wagon, -100, 100, SPIN_STEP), 45);
	assert.equal(SPIN_STEP, 15);
	assert.equal(turned(wagon, 100, 4, SPIN_STEP), 270);
});
