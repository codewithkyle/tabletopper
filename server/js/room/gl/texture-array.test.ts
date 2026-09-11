import assert from "node:assert/strict";
import { test } from "node:test";
import { Slots } from "./texture-array.ts";
test("slots hand out every layer before evicting anything", () => {
	const slots = new Slots(3);
	const layers = ["a", "b", "c"].map((k) => slots.claim(k, 512, 512)?.layer);
	assert.equal(new Set(layers).size, 3, "three keys should take three distinct layers");
	assert.equal(slots.size, 3);
});
test("the least recently drawn tile is the one that goes", () => {
	const slots = new Slots(2);
	slots.claim("a", 512, 512);
	slots.claim("b", 512, 512);
	slots.tick();
	slots.get("a");
	slots.tick();
	slots.claim("c", 512, 512);
	assert.ok(slots.has("a"), "a was drawn a frame ago and should have stayed");
	assert.ok(!slots.has("b"), "b was the least recently drawn");
	assert.ok(slots.has("c"));
});
test("a tile drawn this frame is never evicted for another", () => {
	const slots = new Slots(2);
	slots.claim("a", 512, 512);
	slots.claim("b", 512, 512);
	assert.equal(slots.claim("c", 512, 512), null, "the cache is full of this frame's tiles");
	assert.ok(slots.has("a"));
	assert.ok(slots.has("b"));
	slots.tick();
	assert.notEqual(slots.claim("c", 512, 512), null);
});
test("claiming a key that is already resident keeps its layer", () => {
	const slots = new Slots(4);
	const first = slots.claim("a", 512, 512);
	const again = slots.claim("a", 224, 512);
	assert.equal(again?.layer, first?.layer);
	assert.equal(again?.w, 224, "an edge tile's real width should be updated");
	assert.equal(slots.size, 1);
});
test("a key that was never claimed is not resident", () => {
	const slots = new Slots(2);
	assert.equal(slots.get("nothing"), undefined);
	assert.equal(slots.has("nothing"), false);
});
function stubFetch(): { started: string[]; aborted: string[]; restore: () => void } {
	const real = globalThis.fetch;
	const started: string[] = [];
	const aborted: string[] = [];
	globalThis.fetch = ((url: string, init?: { signal?: AbortSignal }) => {
		started.push(url);
		init?.signal?.addEventListener("abort", () => aborted.push(url));
		return new Promise<Response>(() => {});
	}) as typeof globalThis.fetch;
	return { started, aborted, restore: () => { globalThis.fetch = real; } };
}
