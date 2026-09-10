import assert from "node:assert/strict";
import { test } from "node:test";

import type { Stroke, StrokeKind } from "./protocol.ts";
import { strokeSegments } from "./draw.ts";

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
