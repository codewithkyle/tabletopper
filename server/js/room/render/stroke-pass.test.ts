import assert from "node:assert/strict";
import { test } from "node:test";
import type { Stroke } from "../protocol.ts";
import { createFakeBatch } from "../gl/fake-batch.ts";
import { fillStrokes } from "./stroke-pass.ts";
import { layout } from "./shaders/stroke.ts";
function stroke(over: Partial<Stroke> = {}): Stroke {
	return {
		id: "01STROKE",
		by: "01USER",
		layerId: "01LAYER",
		kind: "free",
		color: "#FF0000FF",
		width: 4,
		points: [0, 0, 10, 0],
		done: true,
		...over,
	};
}
test("a two point stroke writes one segment as endpoints, colour and half width", () => {
	const batch = createFakeBatch(layout);
	fillStrokes(batch, [stroke()]);
	assert.equal(batch.count, 1);
	assert.deepEqual(batch.instance(0), [0, 0, 10, 0, 1, 0, 0, 1, 2]);
});
test("the half width never drops below half a unit", () => {
	const batch = createFakeBatch(layout);
	fillStrokes(batch, [stroke({ width: 0 })]);
	assert.equal(batch.instance(0)[8], 0.5);
});
test("a longer stroke writes one instance per segment", () => {
	const batch = createFakeBatch(layout);
	fillStrokes(batch, [stroke({ points: [0, 0, 10, 0, 10, 10] })]);
	assert.equal(batch.count, 2);
	assert.deepEqual(batch.instance(1).slice(0, 4), [10, 0, 10, 10]);
});
test("a stroke with no geometry contributes nothing", () => {
	const batch = createFakeBatch(layout);
	fillStrokes(batch, [stroke({ points: [] })]);
	assert.equal(batch.count, 0);
});
test("filling again starts from the beginning", () => {
	const batch = createFakeBatch(layout);
	fillStrokes(batch, [stroke(), stroke()]);
	assert.equal(batch.count, 2);
	fillStrokes(batch, [stroke()]);
	assert.equal(batch.count, 1);
});
