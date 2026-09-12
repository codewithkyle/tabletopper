import assert from "node:assert/strict";
import { test } from "node:test";
import type { Event } from "../protocol.ts";
import type { Revisions } from "./revisions.ts";
import { revise, revisions, watching } from "./revisions.ts";
function sent(type: Event["type"]): Event {
	return { type, seq: 1 } as Event;
}
function after(types: readonly Event["type"][]): Revisions {
	const rev = revisions();
	for (const type of types) {
		revise(rev, sent(type));
	}
	return rev;
}
test("a fresh set of revisions counts nothing", () => {
	assert.deepEqual(revisions(), { pawns: 0, fog: 0, strokes: 0, table: 0, initiative: 0 });
});
test("the pawn family bumps the pawns and leaves the rest alone", () => {
	const rev = after(["pawn.spawned", "pawn.updated", "pawn.moved", "pawn.removed"]);
	assert.deepEqual(rev, { pawns: 4, fog: 0, strokes: 0, table: 0, initiative: 0 });
});
test("the fog family bumps the fog", () => {
	const rev = after(["fog.added", "fog.removed"]);
	assert.deepEqual(rev, { pawns: 0, fog: 2, strokes: 0, table: 0, initiative: 0 });
});
test("the stroke family bumps the strokes, extensions included", () => {
	const rev = after(["stroke.began", "stroke.extended", "stroke.ended", "stroke.erased"]);
	assert.deepEqual(rev, { pawns: 0, fog: 0, strokes: 4, table: 0, initiative: 0 });
});
test("the table and the initiative each bump their own", () => {
	assert.deepEqual(after(["table.updated"]), { pawns: 0, fog: 0, strokes: 0, table: 1, initiative: 0 });
	assert.deepEqual(after(["initiative.updated"]), { pawns: 0, fog: 0, strokes: 0, table: 0, initiative: 1 });
});
test("a snapshot bumps every slice", () => {
	assert.deepEqual(after(["snapshot"]), { pawns: 1, fog: 1, strokes: 1, table: 1, initiative: 1 });
});
test("what the table does not hold bumps nothing", () => {
	const rev = after([
		"error", "pinged", "pawn.dragging", "player.joined", "player.updated",
		"player.left", "player.kicked", "room.updated", "room.closed",
	]);
	assert.deepEqual(rev, revisions());
});
test("an event with no revision is a mistake, not a silence", () => {
	assert.throws(() => revise(revisions(), sent("pawn.vanished" as Event["type"])));
});
test("a watch reports its first look and then only what moves", () => {
	const rev = revisions();
	const watch = watching(["pawns", "fog"]);
	assert.equal(watch.changed(rev), true, "a watch slept through the first frame");
	assert.equal(watch.changed(rev), false);
	revise(rev, sent("stroke.began"));
	assert.equal(watch.changed(rev), false, "a stroke woke a watch that does not read strokes");
	revise(rev, sent("fog.added"));
	assert.equal(watch.changed(rev), true);
	assert.equal(watch.changed(rev), false);
});
test("a watch on several slices wakes for any one of them", () => {
	const rev = revisions();
	const watch = watching(["pawns", "table", "fog"]);
	watch.changed(rev);
	for (const type of ["pawn.moved", "table.updated", "fog.removed"] as const) {
		revise(rev, sent(type));
		assert.equal(watch.changed(rev), true, type + " left the watch asleep");
		assert.equal(watch.changed(rev), false);
	}
});
test("a reset watch reports the next look with nothing having moved", () => {
	const rev = revisions();
	const watch = watching(["pawns"]);
	watch.changed(rev);
	assert.equal(watch.changed(rev), false);
	watch.reset();
	assert.equal(watch.changed(rev), true);
	assert.equal(watch.changed(rev), false);
});
