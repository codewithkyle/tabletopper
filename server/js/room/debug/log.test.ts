import assert from "node:assert/strict";
import { test } from "node:test";
import type { Frame } from "../protocol.ts";
import { cidOf, describe, incoming, matches, outgoing, pretty } from "./log.ts";
const changes: Frame = {
	type: "changes",
	seq: 12,
	by: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
	events: [
		{ type: "pawns.moved", pawns: [] },
		{ type: "pawns.upserted", pawns: [] },
	],
};
test("an incoming frame carries its sequence, its size and who caused it", () => {
	const entry = incoming('{"type":"changes"}', changes, null, 61_000);
	assert.equal(entry.out, false);
	assert.equal(entry.type, "changes");
	assert.equal(entry.seq, 12);
	assert.equal(entry.bytes, 18);
	assert.equal(entry.by, "9G5FAV", "the tail of a ulid is enough to tell two players apart");
	assert.equal(entry.detail, "(2) pawns.moved pawns.upserted");
});
test("a frame the socket dropped says so, which is the whole point of logging it", () => {
	const entry = incoming("{}", changes, "duplicate", 0);
	assert.equal(entry.note, "duplicate");
	assert.match(describe(entry), /\[duplicate\]/);
});
test("a frame that would not parse is still an entry", () => {
	const entry = incoming("not json", null, "unparsed", 0);
	assert.equal(entry.type, "unreadable");
	assert.equal(entry.seq, 0);
	assert.equal(entry.by, "");
});
test("an outgoing command is marked as ours and keeps its cid", () => {
	const entry = outgoing('{"type":"pawn.move","cid":"7"}', 0);
	assert.equal(entry.out, true);
	assert.equal(entry.type, "pawn.move");
	assert.equal(entry.detail, "cid 7");
});
test("a described line leads with the clock and the direction", () => {
	const line = describe(outgoing('{"type":"ping","cid":"3"}', 61_500));
	assert.match(line, /^01:01\.500 > ping cid 3 25b$/);
});
test("a filter matches the type, the note or anything in the payload", () => {
	const entry = incoming('{"type":"changes","events":[{"type":"fog.upserted"}]}', changes, null, 0);
	assert.ok(matches(entry, ""), "an empty filter keeps everything");
	assert.ok(matches(entry, "CHANGES"), "matching is case insensitive");
	assert.ok(matches(entry, "fog.upserted"), "a type inside the payload is reachable");
	assert.ok(!matches(entry, "initiative"));
});
test("cid comes back off the wire format the socket writes", () => {
	assert.equal(cidOf('{"type":"ping","cid":"41"}'), "41");
	assert.equal(cidOf('{"type":"ping"}'), "");
	assert.equal(cidOf("garbage"), "");
});
test("pretty printing is idempotent, so reopening a row does not mangle it", () => {
	const once = pretty('{"a":1}');
	assert.equal(once, '{\n  "a": 1\n}');
	assert.equal(pretty(once), once);
	assert.equal(pretty("not json"), "not json");
});
