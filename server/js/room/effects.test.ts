import assert from "node:assert/strict";
import { test } from "node:test";

import type { Event } from "./protocol.ts";
import { fanOut, refusals } from "./effects.ts";

// A refusal reaches the person. The server writes a heading and a message for
// them and the client used to throw both away.
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

// not_found is the table's own race and is answered with the truth rather than
// a dialog: the snapshot that follows prunes whatever named the missing thing.
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

// The list runs in order, every entry, every event.
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
