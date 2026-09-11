import assert from "node:assert/strict";
import { test } from "node:test";
import type { Event } from "./protocol.ts";
import { fanOut, refusals, touchesPawns } from "./effects.ts";
test("a refused command opens the alert with the server's words", () => {
	const shown: string[][] = [];
	let resyncs = 0;
	const effect = refusals({
		alert: (heading, message) => shown.push([heading, message]),
		resync: () => resyncs++,
	});
	effect({ type: "error", seq: 3, cid: "7", code: "forbidden", heading: "Not yours", message: "You can only move your own pawns." });
	assert.deepEqual(shown, [["Not yours", "You can only move your own pawns."]]);
	assert.equal(resyncs, 0);
});
test("a not_found refusal resyncs and says nothing", () => {
	const shown: string[][] = [];
	let resyncs = 0;
	const effect = refusals({
		alert: (heading, message) => shown.push([heading, message]),
		resync: () => resyncs++,
	});
	effect({ type: "error", seq: 3, cid: "7", code: "not_found", heading: "Pawn gone", message: "That pawn is no longer on the table." });
	assert.deepEqual(shown, []);
	assert.equal(resyncs, 1);
});
test("anything that is not an error passes through untouched", () => {
	let touched = 0;
	const effect = refusals({ alert: () => touched++, resync: () => touched++ });
	effect({ type: "pinged", seq: 3, layer: "", x: 0, y: 0 } as Event);
	assert.equal(touched, 0);
});
test("the fan-out runs every effect in order", () => {
	const seen: string[] = [];
	const effect = fanOut([
		() => seen.push("first"),
		() => seen.push("second"),
	]);
	effect({ type: "room.closed", seq: 1 });
	effect({ type: "room.closed", seq: 2 });
	assert.deepEqual(seen, ["first", "second", "first", "second"]);
});
test("the fog family rebuilds the pawn buffer", () => {
	for (const type of ["fog.added", "fog.removed", "fog.cleared"] as const) {
		assert.equal(touchesPawns(type), true, type + " does not rebuild the pawns");
	}
});
test("the pawn family and the whole table rebuild it too", () => {
	assert.equal(touchesPawns("snapshot"), true);
	assert.equal(touchesPawns("table.updated"), true);
	assert.equal(touchesPawns("pawn.moved"), true);
	assert.equal(touchesPawns("pawn.updated"), true);
});
test("a drag preview does not rebuild the pawn buffer", () => {
	assert.equal(touchesPawns("pawn.dragging"), false);
	assert.equal(touchesPawns("player.joined"), false);
	assert.equal(touchesPawns("initiative.updated"), false);
});
