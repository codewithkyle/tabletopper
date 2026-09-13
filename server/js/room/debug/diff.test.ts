import assert from "node:assert/strict";
import { test } from "node:test";
import type { Pawn, State } from "../protocol.ts";
import { diffState, firstDifference, same } from "./diff.ts";
import { empty } from "../store.ts";
function pawn(id: string, x: number): Pawn {
	return {
		id,
		kind: "monster",
		layerId: "floor",
		name: "Goblin",
		image: "",
		x,
		y: 0,
		z: 0,
		size: "medium",
		width: 1,
		height: 1,
		rotation: 0,
		visible: true,
		hp: 7,
		maxHp: 7,
		hpBand: "healthy",
		ac: 15,
		conditions: [],
		ownerId: null,
		monsterId: null,
		characterId: null,
	};
}
function pair(): [State, State] {
	const mine = empty();
	const theirs = empty();
	mine.pawns = [pawn("a", 10), pawn("b", 20)];
	theirs.pawns = [pawn("a", 10), pawn("b", 20)];
	return [mine, theirs];
}
test("two states built the same way converge", () => {
	const [mine, theirs] = pair();
	assert.deepEqual(diffState(mine, theirs), []);
});
test("a slice compares by structure and not by the order its keys were written", () => {
	const [mine, theirs] = pair();
	theirs.room = { locked: false, name: "", id: "" } as State["room"];
	assert.deepEqual(diffState(mine, theirs), [], "the same fields in another order are the same room");
});
test("a pawn the client never got is named as missing", () => {
	const [mine, theirs] = pair();
	theirs.pawns.push(pawn("01JCZZZZZZZZZZZZZZZZZZZZZZ", 30));
	const [found, ...rest] = diffState(mine, theirs);
	assert.equal(rest.length, 0, "only the pawns slice should differ");
	assert.equal(found.name, "pawns");
	assert.match(found.detail, /2 here, 3 there/);
	assert.match(found.detail, /1 never arrived \(ZZZZZZ\)/);
});
test("a pawn the client kept after the server dropped it is named as stale", () => {
	const [mine, theirs] = pair();
	theirs.pawns = theirs.pawns.slice(0, 1);
	const [found] = diffState(mine, theirs);
	assert.equal(found.name, "pawns");
	assert.match(found.detail, /1 never left \(b\)/);
});
test("a pawn that drifted is named and the counts still agree", () => {
	const [mine, theirs] = pair();
	theirs.pawns[1].x = 999;
	const [found] = diffState(mine, theirs);
	assert.equal(found.name, "pawns");
	assert.match(found.detail, /2 here, 2 there/);
	assert.match(found.detail, /1 differ \(b\)/);
});
test("keyed slices ignore the order the client happened to receive them in", () => {
	const [mine, theirs] = pair();
	theirs.pawns = [theirs.pawns[1], theirs.pawns[0]];
	assert.deepEqual(diffState(mine, theirs), []);
});
test("an ordered slice reports the position that differs", () => {
	const [mine, theirs] = pair();
	mine.table.layers = [
		{ id: "one", name: "Ground", map: null, fogEnabled: false, fogPrefill: false, partyStart: null },
		{ id: "two", name: "Upper", map: null, fogEnabled: false, fogPrefill: false, partyStart: null },
	];
	theirs.table.layers = [mine.table.layers[1], mine.table.layers[0]];
	const [found] = diffState(mine, theirs);
	assert.equal(found.name, "layers");
	assert.match(found.detail, /\[0\]/, "the first floor is the first disagreement");
});
test("a table setting that drifted names the field and both values", () => {
	const [mine, theirs] = pair();
	theirs.table.grid.cellSize = 70;
	const [found] = diffState(mine, theirs);
	assert.equal(found.name, "table");
	assert.equal(found.detail, "grid.cellSize: 64 here, 70 there");
});
test("the sequence is not compared, because a reduced state trails the snapshot", () => {
	const [mine, theirs] = pair();
	mine.seq = 12;
	theirs.seq = 4096;
	assert.deepEqual(diffState(mine, theirs), []);
});
test("every slice that differs is reported, not just the first", () => {
	const [mine, theirs] = pair();
	theirs.table.pawnLabels = "none";
	theirs.pawns[0].x = 1;
	theirs.initiative.round = 3;
	const names = diffState(mine, theirs).map((slice) => slice.name);
	assert.deepEqual(names, ["table", "initiative", "pawns"]);
});
test("a field present on one side alone says which side has it", () => {
	assert.equal(firstDifference({ a: 1 }, { a: 1, b: 2 }, ""), "value.b: only there");
	assert.equal(firstDifference({ a: 1, b: 2 }, { a: 1 }, ""), "value.b: only here");
});
test("same is structural and does not care about key order", () => {
	assert.ok(same({ a: 1, b: [1, 2] }, { b: [1, 2], a: 1 }));
	assert.ok(!same({ a: 1, b: [1, 2] }, { a: 1, b: [2, 1] }));
	assert.ok(!same([1, 2], { 0: 1, 1: 2 }));
	assert.ok(same(null, null));
	assert.ok(!same(null, {}));
});
