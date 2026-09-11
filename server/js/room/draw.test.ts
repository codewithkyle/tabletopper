import assert from "node:assert/strict";
import { test } from "node:test";

import type { Grid, Stroke, StrokeKind } from "./protocol.ts";
import { circleSegments, coneCorners, measure, strokeHit, strokeSegments } from "./draw.ts";
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




test("a one-point stroke is one segment of no length", () => {
	assert.deepEqual(strokeSegments(stroke("free", [7, 9]), []), [7, 9, 7, 9]);
});

test("a stroke with no points at all is no segments", () => {
	assert.deepEqual(strokeSegments(stroke("free", []), []), []);
});



test("segments are appended to what is already there", () => {
	const out = [1, 2, 3, 4];
	strokeSegments(stroke("free", [0, 0, 5, 5]), out);

	assert.deepEqual(out, [1, 2, 3, 4, 0, 0, 5, 5]);
});



test("a rectangle is four closed segments", () => {
	const out = strokeSegments(stroke("rect", [0, 0, 100, 60]), []);

	assert.deepEqual(out, [
		0, 0, 100, 0,
		100, 0, 100, 60,
		100, 60, 0, 60,
		0, 60, 0, 0,
	]);
});




test("a rectangle dragged up and left is the same box", () => {
	const forward = strokeSegments(stroke("rect", [0, 0, 100, 60]), []);
	const backward = strokeSegments(stroke("rect", [100, 60, 0, 0]), []);

	assert.deepEqual(backward, forward);
});



test("a circle closes and stays on its radius", () => {
	const r = 80;
	const out = strokeSegments(stroke("circle", [10, 20, 10 + r, 20]), []);

	assert.equal(out.length % 4, 0);
	assert.equal(out.length / 4, circleSegments(r));

	
	for (let i = 0; i + 3 < out.length; i += 4) {
		assert.ok(Math.abs(Math.hypot(out[i] - 10, out[i + 1] - 20) - r) < 1e-6, "off the rim");
		if (i > 0) {
			assert.ok(Math.abs(out[i] - out[i - 2]) < 1e-9 && Math.abs(out[i + 1] - out[i - 1]) < 1e-9, "a gap between segments");
		}
	}

	
	const last = out.length - 2;
	assert.ok(Math.abs(out[last] - out[0]) < 1e-6 && Math.abs(out[last + 1] - out[1]) < 1e-6, "the circle is open");
});



test("a circle takes its radius from any direction", () => {
	const straight = strokeSegments(stroke("circle", [0, 0, 100, 0]), []);
	const diagonal = strokeSegments(stroke("circle", [0, 0, 60, 80]), []);

	assert.equal(diagonal.length, straight.length, "a 3-4-5 rim is the same radius");
});




test("a circle is cut finely enough and never too finely", () => {
	assert.equal(circleSegments(1), 24, "a tiny circle still gets its floor");
	assert.equal(circleSegments(1_000_000), 512, "a huge circle stops at the ceiling");

	const middling = circleSegments(200);
	assert.ok(middling > 24 && middling < 512, `${middling} is not between the clamps`);

	
	const sagitta = 200 * (1 - Math.cos(Math.PI / middling));
	assert.ok(sagitta < 0.05, `a chord bulges by ${sagitta} map pixels`);
});

test("a shape with fewer than two points draws nothing", () => {
	for (const kind of ["rect", "circle"] as const) {
		assert.deepEqual(strokeSegments(stroke(kind, [0, 0]), []), [], kind);
	}

	
	assert.deepEqual(strokeSegments(stroke("circle", [5, 5, 5, 5]), []), []);
});



test("a cone's base is as wide as the cone is long", () => {
	
	const out = coneCorners(0, 0, 0, 100, []);

	assert.deepEqual(out, [0, 0, -50, 100, 50, 100]);

	const width = Math.hypot(out[4] - out[2], out[5] - out[3]);
	assert.equal(width, 100, "the base is not the same as the length");
});



test("a cone keeps its proportions through a full turn", () => {
	const length = 120;

	for (let deg = 0; deg < 360; deg += 15) {
		const a = (deg * Math.PI) / 180;
		const bx = Math.cos(a) * length;
		const by = Math.sin(a) * length;

		const out = coneCorners(0, 0, bx, by, []);
		assert.equal(out.length, 6, `${deg}deg`);

		
		
		const left = Math.hypot(out[2], out[3]);
		const right = Math.hypot(out[4], out[5]);
		assert.ok(Math.abs(left - right) < 1e-9, `${deg}deg: not isosceles`);

		const base = Math.hypot(out[4] - out[2], out[5] - out[3]);
		assert.ok(Math.abs(base - length) < 1e-9, `${deg}deg: base is ${base}`);

		
		const dot = bx * (out[4] - out[2]) + by * (out[5] - out[3]);
		assert.ok(Math.abs(dot) < 1e-6, `${deg}deg: the base is not square to the axis`);
	}
});

test("a cone of no length has no corners", () => {
	assert.deepEqual(coneCorners(40, 40, 40, 40, []), []);
});


test("a cone is three closed segments", () => {
	const out = strokeSegments(stroke("cone", [0, 0, 0, 100]), []);

	assert.deepEqual(out, [
		0, 0, -50, 100,
		-50, 100, 50, 100,
		50, 100, 0, 0,
	]);
});

test("a cone with no length draws nothing", () => {
	assert.deepEqual(strokeSegments(stroke("cone", [7, 7, 7, 7]), []), []);
	assert.deepEqual(strokeSegments(stroke("cone", [0, 0]), []), []);
});




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




test("a fat line is hit across its whole width", () => {
	const thin = line({ width: 2 });
	const fat = line({ width: 24 });

	
	
	assert.equal(strokeHit(thin, 50, 15, 6, []), false);
	assert.equal(strokeHit(fat, 50, 15, 6, []), true);
});



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



test("the hit test does not accumulate across calls", () => {
	const scratch: number[] = [];
	const it = line();

	strokeHit(it, 50, 0, 6, scratch);
	const first = scratch.length;
	strokeHit(it, 50, 0, 6, scratch);

	assert.equal(scratch.length, first);
});




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



test("a circle says its radius", () => {
	
	assert.deepEqual(said("circle", [500, 500, 500 + 256, 500]), ["20 ft."]);
});

test("a circle's radius is a straight line in any direction", () => {
	
	assert.deepEqual(said("circle", [0, 0, 192, 256]), ["25 ft."]);
});




test("a diagonal radius is not scored by the diagonal rule", () => {
	const equal = said("circle", [0, 0, 192, 256], grid({ diagonals: "equal" }));
	const alternating = said("circle", [0, 0, 192, 256], grid({ diagonals: "alternating" }));

	assert.deepEqual(alternating, equal);
});


test("a rectangle says both of its sides", () => {
	assert.deepEqual(said("rect", [0, 0, 384, 128]), ["30 ft.", "10 ft."]);
});

test("a rectangle's labels do not care which way it was dragged", () => {
	assert.deepEqual(said("rect", [384, 128, 0, 0]), said("rect", [0, 0, 384, 128]));
});



test("a freehand line says nothing", () => {
	assert.deepEqual(said("free", [0, 0, 100, 0, 200, 50]), []);
});

test("a shape with no size says nothing", () => {
	assert.deepEqual(said("circle", [10, 10, 10, 10]), []);
	assert.deepEqual(said("rect", [10, 10, 10, 10]), []);

	
	assert.deepEqual(said("rect", [0, 0, 384, 0]), ["30 ft."]);
});




test("a label follows the grid it is measured against", () => {
	const points = [0, 0, 256, 0];

	assert.deepEqual(said("circle", points), ["20 ft."]);
	assert.deepEqual(said("circle", points, grid({ feetPerCell: 10 })), ["40 ft."]);
	assert.deepEqual(said("circle", points, grid({ cellSize: 128 })), ["10 ft."]);
});



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



test("a circle's label sits above its rim and a rectangle's above its top edge", () => {
	const circle: [number, number][] = [];
	measure("circle", [100, 200, 100, 260], grid(), "#FFF", (_t, x, y) => circle.push([x, y]));
	assert.deepEqual(circle, [[100, 140]], "the label is not above the top of the circle");

	const rect: [number, number][] = [];
	measure("rect", [0, 100, 200, 300], grid(), "#FFF", (_t, x, y) => rect.push([x, y]));
	assert.deepEqual(rect, [[100, 100], [0, 200]], "the two labels are not on the two edges");
});



test("a cone says the distance from its point to its base", () => {
	
	assert.deepEqual(said("cone", [0, 0, 0, 384]), ["30 ft."]);
});



test("a cone reads the same at every angle", () => {
	const length = 384;

	for (let deg = 0; deg < 360; deg += 15) {
		const a = (deg * Math.PI) / 180;
		const said_ = said("cone", [0, 0, Math.round(Math.cos(a) * length), Math.round(Math.sin(a) * length)]);

		assert.deepEqual(said_, ["30 ft."], `${deg}deg`);
	}
});

test("a cone with no length says nothing", () => {
	assert.deepEqual(said("cone", [10, 10, 10, 10]), []);
});




test("a cone's label is above it at every angle", () => {
	const length = 200;

	for (let deg = 0; deg < 360; deg += 15) {
		const a = (deg * Math.PI) / 180;
		const bx = Math.round(Math.cos(a) * length);
		const by = Math.round(Math.sin(a) * length);

		const at: [number, number][] = [];
		measure("cone", [0, 0, bx, by], grid(), "#FFF", (_t, x, y) => at.push([x, y]));
		assert.equal(at.length, 1, `${deg}deg`);

		const corners = coneCorners(0, 0, bx, by, []);
		let minX = Infinity, maxX = -Infinity, minY = Infinity;
		for (let i = 0; i + 1 < corners.length; i += 2) {
			minX = Math.min(minX, corners[i]);
			maxX = Math.max(maxX, corners[i]);
			minY = Math.min(minY, corners[i + 1]);
		}

		assert.ok(at[0][1] <= minY + 1e-9, `${deg}deg: the label is inside the cone`);
		assert.ok(at[0][0] >= minX - 1e-9 && at[0][0] <= maxX + 1e-9, `${deg}deg: the label is off to one side`);
	}
});
