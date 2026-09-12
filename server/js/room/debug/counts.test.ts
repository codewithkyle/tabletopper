import assert from "node:assert/strict";
import { test } from "node:test";
import { newCounts } from "./counts.ts";
test("totals accumulate per type and sort heaviest first", () => {
	const counts = newCounts();
	counts.add("pawns.moved");
	counts.add("pawns.moved");
	counts.add("fog.upserted");
	const rows = counts.sample(0);
	assert.deepEqual(rows.map((row) => row.type), ["pawns.moved", "fog.upserted"]);
	assert.equal(rows[0].total, 2);
});
test("a rate is only settled once a full second has passed under it", () => {
	const counts = newCounts();
	counts.sample(0);
	for (let i = 0; i < 30; i++) {
		counts.add("pawn.dragging");
	}
	assert.equal(counts.sample(250)[0].rate, 0, "a quarter second is too short to divide by");
	const settled = counts.sample(1000);
	assert.equal(settled[0].rate, 30);
	assert.equal(settled[0].total, 30);
});
test("a rate falls back to zero when the firehose stops", () => {
	const counts = newCounts();
	counts.sample(0);
	counts.add("pawn.dragging");
	counts.sample(1000);
	assert.equal(counts.sample(2000)[0].rate, 0);
	assert.equal(counts.sample(2000)[0].total, 1, "the total is cumulative and does not decay");
});
test("ties between types are broken by name so the list does not jitter", () => {
	const counts = newCounts();
	counts.add("b");
	counts.add("a");
	assert.deepEqual(counts.sample(0).map((row) => row.type), ["a", "b"]);
});
test("clearing forgets every type", () => {
	const counts = newCounts();
	counts.add("pawns.moved");
	counts.reset();
	assert.deepEqual(counts.sample(0), []);
});
