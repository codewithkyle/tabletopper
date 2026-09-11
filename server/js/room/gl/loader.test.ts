import assert from "node:assert/strict";
import { test } from "node:test";
import { newLoader } from "./loader.ts";
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
test("a tile that has left the viewport keeps its connection until one is needed", () => {
	const net = stubFetch();
	try {
		const loader = newLoader(() => {});
		loader.begin();
		for (let i = 0; i < 10; i++) {
			loader.want(`k${i}`, `/tiles/${i}`, i);
		}
		loader.end();
		assert.equal(net.started.length, 8, "eight at once is the cap");
		loader.begin();
		loader.end();
		assert.equal(net.aborted.length, 0, "cancelled with nothing to cancel for");
		loader.begin();
		loader.want("fresh", "/tiles/fresh", 0);
		loader.end();
		assert.equal(net.aborted.length, 1, "freed more connections than were asked for");
		assert.equal(net.started.at(-1), "/tiles/fresh");
	} finally {
		net.restore();
	}
});
test("a level change frees as many connections as the new level needs", () => {
	const net = stubFetch();
	try {
		const loader = newLoader(() => {});
		loader.begin();
		for (let i = 0; i < 8; i++) {
			loader.want(`old${i}`, `/tiles/old/${i}`, i);
		}
		loader.end();
		assert.equal(net.started.length, 8);
		loader.begin();
		for (let i = 0; i < 20; i++) {
			loader.want(`new${i}`, `/tiles/new/${i}`, i);
		}
		loader.end();
		assert.equal(net.aborted.length, 8, "the level just left is still holding connections");
		assert.equal(net.started.length, 16, "the new level did not get the cap");
	} finally {
		net.restore();
	}
});
