import assert from "node:assert/strict";
import { test } from "node:test";
import type { Initiative } from "../protocol.ts";
import { actingPawnIds, compareStack } from "./stack.ts";
test("every token is drawn under every creature", () => {
	const rug = { id: "rug", kind: "object" as const, z: 999 };
	const goblin = { id: "goblin", kind: "monster" as const, z: 0 };
	assert.ok(compareStack(rug, goblin) < 0);
	assert.ok(compareStack(goblin, rug) > 0);
});
test("z orders two of a kind and the id breaks a tie", () => {
	const early = { id: "b", kind: "object" as const, z: 1 };
	const late = { id: "a", kind: "object" as const, z: 2 };
	assert.ok(compareStack(early, late) < 0);
	assert.ok(compareStack({ ...early, z: 2 }, late) > 0);
	assert.equal(compareStack(late, late), 0);
});
function order(over: Partial<Initiative> = {}): Initiative {
	return {
		entries: [
			{ id: "01HERO", pawnIds: ["hero"], name: "Ilya", initiative: 18 },
			{ id: "01MOB", pawnIds: ["a", "b", "c"], name: "Goblins", initiative: 12 },
			{ id: "01LAIR", pawnIds: [], name: "Lair action", initiative: 20 },
		],
		active: null,
		round: 1,
		...over,
	};
}
test("a solo line is acting on its own", () => {
	assert.deepEqual(actingPawnIds(order({ active: "01HERO" })), ["hero"]);
});
test("a grouped line hands over every creature on it", () => {
	assert.deepEqual(actingPawnIds(order({ active: "01MOB" })), ["a", "b", "c"]);
});
test("a line with no creature behind it marks nobody", () => {
	assert.deepEqual(actingPawnIds(order({ active: "01LAIR" })), []);
});
test("nothing is acting outside a fight", () => {
	assert.deepEqual(actingPawnIds(order()), []);
});
test("an active line that is not in the order marks nobody", () => {
	assert.deepEqual(actingPawnIds(order({ active: "01GONE" })), []);
});
test("the empty answer allocates nothing", () => {
	assert.equal(actingPawnIds(order()), actingPawnIds(order({ active: "01GONE" })));
});
