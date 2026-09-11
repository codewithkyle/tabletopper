import assert from "node:assert/strict";
import { test } from "node:test";
import { SHAPED_QUAD, writeShaped } from "./shaped-quad.ts";
import { createFakeBatch } from "../gl/fake-batch.ts";
test("a shaped quad is rect, colour, style and spin in that order", () => {
	const batch = createFakeBatch(SHAPED_QUAD);
	writeShaped(batch, 10, 20, 3, 4, [0.1, 0.2, 0.3], 0.5, 6, 7, 8, 9, 0);
	assert.deepEqual(batch.instance(0), [
		10, 20, 3, 4,
		0.1, 0.2, 0.3, 0.5,
		6, 7, 8, 9,
		1, 0,
	].map((v) => Math.fround(v)));
});
test("no rotation is the identity spin rather than a cosine of zero", () => {
	const batch = createFakeBatch(SHAPED_QUAD);
	writeShaped(batch, 0, 0, 1, 1, [0, 0, 0], 1, 0, 0, 0, 0, 0);
	const spun = batch.instance(0);
	assert.equal(spun[12], 1);
	assert.equal(spun[13], 0);
});
test("a quarter turn writes its cosine and sine", () => {
	const batch = createFakeBatch(SHAPED_QUAD);
	writeShaped(batch, 0, 0, 1, 1, [0, 0, 0], 1, 0, 0, 0, 0, 90);
	const spun = batch.instance(0);
	assert.ok(Math.abs(spun[12]) < 1e-6, `cos was ${spun[12]}`);
	assert.ok(Math.abs(spun[13] - 1) < 1e-6, `sin was ${spun[13]}`);
});
test("each instance lands one stride after the last", () => {
	const batch = createFakeBatch(SHAPED_QUAD);
	writeShaped(batch, 1, 1, 1, 1, [0, 0, 0], 1, 0, 0, 0, 0, 0);
	writeShaped(batch, 2, 2, 1, 1, [0, 0, 0], 1, 0, 0, 0, 0, 0);
	assert.equal(batch.count, 2);
	assert.equal(batch.instance(1)[0], 2);
});
