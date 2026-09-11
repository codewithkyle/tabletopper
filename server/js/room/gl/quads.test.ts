import assert from "node:assert/strict";
import { test } from "node:test";
import { createFakeBatch } from "./fake-batch.ts";
import { strideOf } from "./quads.ts";
const LAYOUT = [{ size: 4 }, { size: 2 }] as const;
test("the stride is the sum of the attribute sizes", () => {
	assert.equal(strideOf(LAYOUT), 6);
	assert.equal(strideOf([]), 0);
	assert.equal(strideOf([{ size: 1 }, { size: 1 }, { size: 3 }]), 5);
});
test("a cursor hands out one stride at a time and counts them", () => {
	const batch = createFakeBatch(LAYOUT);
	assert.equal(batch.cursor(), 0);
	assert.equal(batch.cursor(), 6);
	assert.equal(batch.cursor(), 12);
	assert.equal(batch.count, 3);
});
test("begin rewinds the count without dropping the store", () => {
	const batch = createFakeBatch(LAYOUT);
	const at = batch.cursor();
	batch.data[at] = 7;
	batch.begin();
	assert.equal(batch.count, 0);
	assert.equal(batch.cursor(), 0, "the next instance writes over the first");
});
test("the store grows past its initial capacity and keeps what was written", () => {
	const batch = createFakeBatch(LAYOUT);
	for (let i = 0; i < 64; i++) {
		const at = batch.cursor();
		batch.data[at] = i;
	}
	assert.equal(batch.count, 64);
	assert.equal(batch.instance(0)[0], 0, "growing lost the first instance");
	assert.equal(batch.instance(63)[0], 63);
});
test("reserving up front does not add instances", () => {
	const batch = createFakeBatch(LAYOUT);
	batch.reserve(500);
	assert.equal(batch.count, 0);
	assert.ok(batch.data.length >= 500 * 6);
});
