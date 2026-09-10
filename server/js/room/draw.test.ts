import assert from "node:assert/strict";
import { test } from "node:test";

import type { Grid, Stroke, StrokeKind } from "./protocol.ts";
import { circleSegments, measure, strokeHit, strokeSegments } from "./draw.ts";
import { GLYPHS } from "./render/glyphs.ts";

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

// A RECTANGLE IS FOUR SEGMENTS AND IT CLOSES. An open box would leave a gap at
// one corner that reads as the shape being broken.
test("a rectangle is four closed segments", () => {
	const out = strokeSegments(stroke("rect", [0, 0, 100, 60]), []);

	assert.deepEqual(out, [
		0, 0, 100, 0,
		100, 0, 100, 60,
		100, 60, 0, 60,
		0, 60, 0, 0,
	]);
});

// CORNERS ARE NORMALISED ON THE WAY IN AS WELL AS ON THE WAY OUT, so a
// rectangle from an older build, or one somebody posted by hand, draws the same
// box as one the tool sent.
test("a rectangle dragged up and left is the same box", () => {
	const forward = strokeSegments(stroke("rect", [0, 0, 100, 60]), []);
	const backward = strokeSegments(stroke("rect", [100, 60, 0, 0]), []);

	assert.deepEqual(backward, forward);
});

// A CIRCLE IS A CENTRE AND A POINT ON ITS RIM, which is what keeps it four
// integers on the wire and lets its radius be recomputed at any time.
test("a circle closes and stays on its radius", () => {
	const r = 80;
	const out = strokeSegments(stroke("circle", [10, 20, 10 + r, 20]), []);

	assert.equal(out.length % 4, 0);
	assert.equal(out.length / 4, circleSegments(r));

	// Every point is on the rim, and each segment starts where the last ended.
	for (let i = 0; i + 3 < out.length; i += 4) {
		assert.ok(Math.abs(Math.hypot(out[i] - 10, out[i + 1] - 20) - r) < 1e-6, "off the rim");
		if (i > 0) {
			assert.ok(Math.abs(out[i] - out[i - 2]) < 1e-9 && Math.abs(out[i + 1] - out[i - 1]) < 1e-9, "a gap between segments");
		}
	}

	// And it comes back to where it started.
	const last = out.length - 2;
	assert.ok(Math.abs(out[last] - out[0]) < 1e-6 && Math.abs(out[last + 1] - out[1]) < 1e-6, "the circle is open");
});

// THE RIM POINT IS A POINT AND NOT AN AXIS, so a circle dragged diagonally is
// the same size as one dragged straight out.
test("a circle takes its radius from any direction", () => {
	const straight = strokeSegments(stroke("circle", [0, 0, 100, 0]), []);
	const diagonal = strokeSegments(stroke("circle", [0, 0, 60, 80]), []);

	assert.equal(diagonal.length, straight.length, "a 3-4-5 rim is the same radius");
});

// A CHORD OF FOUR MAP PIXELS IS INVISIBLE AT ANY ZOOM THIS CAMERA REACHES, and
// the clamp at both ends is what keeps a tiny circle from being an octagon and
// a huge one from filling the buffer.
test("a circle is cut finely enough and never too finely", () => {
	assert.equal(circleSegments(1), 24, "a tiny circle still gets its floor");
	assert.equal(circleSegments(1_000_000), 512, "a huge circle stops at the ceiling");

	const middling = circleSegments(200);
	assert.ok(middling > 24 && middling < 512, `${middling} is not between the clamps`);

	// The bulge in the middle of a chord, which is what a person would see.
	const sagitta = 200 * (1 - Math.cos(Math.PI / middling));
	assert.ok(sagitta < 0.05, `a chord bulges by ${sagitta} map pixels`);
});

test("a shape with fewer than two points draws nothing", () => {
	for (const kind of ["rect", "circle"] as const) {
		assert.deepEqual(strokeSegments(stroke(kind, [0, 0]), []), [], kind);
	}

	// And a circle whose rim is its own centre has no radius to draw.
	assert.deepEqual(strokeSegments(stroke("circle", [5, 5, 5, 5]), []), []);
});

// The cone is checkpoint 5. Until then it expands to nothing rather than
// throwing, which is what keeps a stroke from a build ahead of this one from
// taking the frame down with it.
test("a cone draws nothing until its expansion is written", () => {
	assert.deepEqual(strokeSegments(stroke("cone", [0, 0, 40, 40]), []), []);
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

// THE LABELS. What each shape says is the number the spell is written with, so
// these are the tests that decide whether the feature does its job.

function grid(over: Partial<Grid> = {}): Grid {
	return {
		lines: "solid", cellSize: 64, offsetX: 0, offsetY: 0,
		color: "#000000FF", snap: "cells", feetPerCell: 5, diagonals: "equal", ...over,
	};
}

function said(kind: StrokeKind, points: number[], g: Grid = grid()): string[] {
	const out: string[] = [];
	measure(kind, points, g, "#FFFFFF", (text) => out.push(text));

	return out;
}

// A CIRCLE SAYS ITS RADIUS, because a spell is written "20-foot radius sphere".
// A diameter would be the same shape described with a number nobody can look up.
test("a circle says its radius", () => {
	// Four cells of rim on a 64-pixel, 5-foot grid.
	assert.deepEqual(said("circle", [500, 500, 500 + 256, 500]), ["20 ft."]);
});

test("a circle's radius is a straight line in any direction", () => {
	// A 3-4-5 rim: 192 by 256 is 320 pixels, which is five cells.
	assert.deepEqual(said("circle", [0, 0, 192, 256]), ["25 ft."]);
});

// EVERY NUMBER IS PYTHAGORAS AND NOT A COUNT OF SQUARES. A radius is a line on
// a map and does not care which cells it crosses; cellsMoved is for a creature
// walking, and would answer this question with the diagonal rule applied.
test("a diagonal radius is not scored by the diagonal rule", () => {
	const equal = said("circle", [0, 0, 192, 256], grid({ diagonals: "equal" }));
	const alternating = said("circle", [0, 0, 192, 256], grid({ diagonals: "alternating" }));

	assert.deepEqual(alternating, equal);
});

// A RECTANGLE SAYS ONE PER AXIS, because a wall or a room is two measurements.
test("a rectangle says both of its sides", () => {
	assert.deepEqual(said("rect", [0, 0, 384, 128]), ["30 ft.", "10 ft."]);
});

test("a rectangle's labels do not care which way it was dragged", () => {
	assert.deepEqual(said("rect", [384, 128, 0, 0]), said("rect", [0, 0, 384, 128]));
});

// A pen stroke says nothing: a freehand squiggle has no distance anybody asked
// for, and a label per line would bury the table in text.
test("a freehand line says nothing", () => {
	assert.deepEqual(said("free", [0, 0, 100, 0, 200, 50]), []);
});

test("a shape with no size says nothing", () => {
	assert.deepEqual(said("circle", [10, 10, 10, 10]), []);
	assert.deepEqual(said("rect", [10, 10, 10, 10]), []);

	// A rectangle with one axis is a line, and it says only that axis.
	assert.deepEqual(said("rect", [0, 0, 384, 0]), ["30 ft."]);
});

// THE GRID IS WHAT TURNS MAP PIXELS INTO FEET, and a GM can retune it
// mid-session -- which is why the number is recomputed per frame rather than
// stored with the shape.
test("a label follows the grid it is measured against", () => {
	const points = [0, 0, 256, 0];

	assert.deepEqual(said("circle", points), ["20 ft."]);
	assert.deepEqual(said("circle", points, grid({ feetPerCell: 10 })), ["40 ft."]);
	assert.deepEqual(said("circle", points, grid({ cellSize: 128 })), ["10 ft."]);
});

// THE ATLAS HOLDS FOURTEEN CHARACTERS and a label with anything else in it
// renders a hole. Nothing here may ever produce a letter that is not f or t.
test("every label is inside the glyph atlas", () => {
	const cases: [StrokeKind, number[]][] = [
		["circle", [0, 0, 37, 91]],
		["rect", [0, 0, 411, 173]],
		["circle", [0, 0, 1, 0]],
		["rect", [-5000, -5000, 5000, 5000]],
	];

	for (const [kind, points] of cases) {
		for (const text of said(kind, points)) {
			for (const char of text) {
				assert.ok(GLYPHS.includes(char), `${JSON.stringify(text)} has a character the atlas cannot draw: ${char}`);
			}
		}
	}
});

// WHERE A LABEL SITS IS THE SAME WHETHER THE SHAPE IS IN HAND OR ON THE TABLE,
// so the number does not jump when the button comes up.
test("a circle's label sits above its rim and a rectangle's above its top edge", () => {
	const circle: [number, number][] = [];
	measure("circle", [100, 200, 100, 260], grid(), "#FFF", (_t, x, y) => circle.push([x, y]));
	assert.deepEqual(circle, [[100, 140]], "the label is not above the top of the circle");

	const rect: [number, number][] = [];
	measure("rect", [0, 100, 200, 300], grid(), "#FFF", (_t, x, y) => rect.push([x, y]));
	assert.deepEqual(rect, [[100, 100], [0, 200]], "the two labels are not on the two edges");
});
