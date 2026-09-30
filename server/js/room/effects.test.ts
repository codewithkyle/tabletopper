import assert from "node:assert/strict";
import { test } from "node:test";
import type { Event } from "./protocol.ts";
import { departures, fanOut, refusals } from "./effects.ts";
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
test("a closed room sends a player home to read why", () => {
	let closed = 0;
	const kicks: string[] = [];
	const effect = departures({ role: "player", kicked: (reason) => kicks.push(reason), closed: () => closed++ });
	effect({ type: "room.closed", seq: 4 });
	assert.equal(closed, 1);
	assert.deepEqual(kicks, []);
});
test("the GM stays put on a close, because the response to their own click moves them", () => {
	let closed = 0;
	const effect = departures({ role: "gm", kicked: () => closed++, closed: () => closed++ });
	effect({ type: "room.closed", seq: 4 });
	assert.equal(closed, 0);
});
test("a kick sends the kicked player home with the reason the server gave", () => {
	let closed = 0;
	const kicks: string[] = [];
	const effect = departures({ role: "player", kicked: (reason) => kicks.push(reason), closed: () => closed++ });
	effect({ type: "player.kicked", seq: 4, reason: "The GM removed you from the room." });
	assert.deepEqual(kicks, ["The GM removed you from the room."]);
	assert.equal(closed, 0);
});
test("a departure effect leaves every other event alone", () => {
	let touched = 0;
	const effect = departures({ role: "player", kicked: () => touched++, closed: () => touched++ });
	effect({ type: "pinged", seq: 3, layer: "", x: 0, y: 0 } as Event);
	assert.equal(touched, 0);
});
