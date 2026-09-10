import assert from "node:assert/strict";
import { test } from "node:test";

import type { Event } from "./protocol.ts";
import { fanOut, refusals, touchesPawns } from "./effects.ts";

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

// WHAT REBUILDS THE PAWN BUFFER, AND WHY FOG IS IN THE LIST. A pawn under the
// cover is left out of the buffer rather than painted over, so uncovering a room
// changes which pawns are drawn without changing a single pawn -- and the bug
// this pins is a player watching an uncovered room stay empty until the next
// time anybody moved anything.
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

// A GHOST IS DRAWN FROM A BUFFER OF ITS OWN and arrives twenty times a second.
// Rebuilding the whole table's instances for one is what the split between the
// two buffers exists to avoid.
test("a drag preview does not rebuild the pawn buffer", () => {
	assert.equal(touchesPawns("pawn.dragging"), false);
	assert.equal(touchesPawns("player.joined"), false);
	assert.equal(touchesPawns("initiative.updated"), false);
});
