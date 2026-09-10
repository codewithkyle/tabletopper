import assert from "node:assert/strict";
import { test } from "node:test";

import type { Stroke, StrokeKind } from "./protocol.ts";
import { strokeHit, strokeSegments } from "./draw.ts";

function stroke(kind: StrokeKind, points: number[]): Pick<Stroke, "kind" | "points"> {
	return { kind, points };
}

test("a freehand stroke is the segments between its points", () => {
	const out = strokeSegments(stroke("free", [0, 0, 10, 0, 10, 10]), []);

	assert.deepEqual(out, [
		0, 0, 10, 0,
		10, 0, 10, 10,
	]);
});

// A CLICK WITH A PEN IS A DOT, and the shader draws a segment of no length as
// one round cap. Dropping it instead would mean a tap that left no mark and no
// explanation.
test("a one-point stroke is one segment of no length", () => {
	assert.deepEqual(strokeSegments(stroke("free", [7, 9]), []), [7, 9, 7, 9]);
});

test("a stroke with no points at all is no segments", () => {
	assert.deepEqual(strokeSegments(stroke("free", []), []), []);
});

// The expansion APPENDS, because the pass fills one array per batch and reuses
// it across every stroke on the floor.
test("segments are appended to what is already there", () => {
	const out = [1, 2, 3, 4];
	strokeSegments(stroke("free", [0, 0, 5, 5]), out);

	assert.deepEqual(out, [1, 2, 3, 4, 0, 0, 5, 5]);
});

// The three shapes are checkpoints 4 and 5. Until then they expand to nothing
// rather than throwing, which is what keeps a stroke from a build ahead of this
// one from taking the frame down with it.
test("a shape draws nothing until its expansion is written", () => {
	for (const kind of ["rect", "circle", "cone"] as const) {
		assert.deepEqual(strokeSegments(stroke(kind, [0, 0, 40, 40]), []), [], kind);
	}
});

// THE ERASER'S HIT TEST. It is a distance to the ink rather than to the centre
// line, behind a bounding box that rejects the lines nowhere near the pointer.

function line(over: Partial<Stroke> = {}): Stroke {
	return {
		id: "01A", by: "01GM", layerId: "L", kind: "free",
		color: "#FF0000", width: 4, points: [0, 0, 100, 0], done: true, ...over,
	};
}

test("the eraser finds a line under the pointer", () => {
	const it = line();

	assert.equal(strokeHit(it, 50, 0, 6, []), true, "dead on the line");
	assert.equal(strokeHit(it, 50, 7, 6, []), true, "within the radius plus half the width");
	assert.equal(strokeHit(it, 50, 40, 6, []), false, "nowhere near it");
});

// THE STROKE'S OWN WIDTH COUNTS. A sixty-four pixel brush is hit where it is
// drawn and not where its centre line runs -- otherwise a fat line would have to
// be aimed at down its middle.
test("a fat line is hit across its whole width", () => {
	const thin = line({ width: 2 });
	const fat = line({ width: 64 });

	assert.equal(strokeHit(thin, 50, 20, 6, []), false);
	assert.equal(strokeHit(fat, 50, 20, 6, []), true);
});

// PAST THE END OF A SEGMENT THE DISTANCE IS TO THE ENDPOINT, which is what
// makes the reach round rather than an infinite band down the line's axis.
test("the eraser reach ends where the line does", () => {
	const it = line();

	assert.equal(strokeHit(it, 103, 0, 6, []), true, "just past the end, inside the reach");
	assert.equal(strokeHit(it, 130, 0, 6, []), false, "well past the end");
	assert.equal(strokeHit(it, -30, 0, 6, []), false, "well before the start");
});

test("the eraser finds a corner of a bent line", () => {
	const bent = line({ points: [0, 0, 100, 0, 100, 100] });

	assert.equal(strokeHit(bent, 100, 50, 6, []), true);
	assert.equal(strokeHit(bent, 50, 50, 6, []), false);
});

test("the eraser finds a single-point dot", () => {
	const dot = line({ points: [40, 40] });

	assert.equal(strokeHit(dot, 42, 42, 6, []), true);
	assert.equal(strokeHit(dot, 80, 80, 6, []), false);
});

// The scratch array is reused across every stroke in a sweep, so the hit test
// has to fill it rather than append to it.
test("the hit test does not accumulate across calls", () => {
	const scratch: number[] = [];
	const it = line();

	strokeHit(it, 50, 0, 6, scratch);
	const first = scratch.length;
	strokeHit(it, 50, 0, 6, scratch);

	assert.equal(scratch.length, first);
});
