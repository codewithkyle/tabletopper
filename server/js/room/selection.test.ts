import assert from "node:assert/strict";
import { test } from "node:test";
import type { Pawn } from "./protocol.ts";
import { SELECTION_MAX, Selection, dragSet, marqueeSelect, mayMove, riders } from "./selection.ts";
const GROUND = "01LAYERGROUND";
const CELLAR = "01LAYERCELLAR";
const ME = "01PLAYERME";
const THEM = "01PLAYERTHEM";
const CELL = 64;
function pawn(over: Partial<Pawn> = {}): Pawn {
	return {
		id: "01PAWN",
		kind: "monster",
		layerId: GROUND,
		name: "Goblin",
		image: "",
		x: 0,
		y: 0,
		z: 1,
		size: "medium",
		width: 0,
		height: 0,
		rotation: 0,
		visible: true,
		hp: 7,
		maxHp: 7,
		hpBand: null,
		ac: 15,
		conditions: [],
		ownerId: null,
		monsterId: null,
		characterId: null,
		...over,
	};
}
test("the GM may move anything and a player only their own", () => {
	const mine = pawn({ ownerId: ME });
	const theirs = pawn({ ownerId: THEM });
	const nobodys = pawn({ ownerId: null });
	assert.equal(mayMove(mine, "gm", "01GM"), true);
	assert.equal(mayMove(nobodys, "gm", "01GM"), true);
	assert.equal(mayMove(mine, "player", ME), true);
	assert.equal(mayMove(theirs, "player", ME), false);
	assert.equal(mayMove(nobodys, "player", ME), false);
});
test("a marquee takes every movable pawn whose centre is inside", () => {
	const pawns = [
		pawn({ id: "in", x: 100, y: 100 }),
		pawn({ id: "out", x: 400, y: 100 }),
		pawn({ id: "edge", x: 200, y: 200 }),
	];
	const found = marqueeSelect(pawns, GROUND, { x1: 0, y1: 0, x2: 200, y2: 200 }, "gm", "01GM");
	assert.deepEqual(found, ["in", "edge"]);
});
test("a marquee dragged backwards is the same rectangle", () => {
	const pawns = [pawn({ id: "in", x: 100, y: 100 })];
	assert.deepEqual(
		marqueeSelect(pawns, GROUND, { x1: 200, y1: 200, x2: 0, y2: 0 }, "gm", "01GM"),
		["in"],
	);
});
test("a marquee takes only what the viewer may move", () => {
	const pawns = [
		pawn({ id: "goblin", x: 10, y: 10, ownerId: null }),
		pawn({ id: "mine", x: 20, y: 20, ownerId: ME }),
		pawn({ id: "theirs", x: 30, y: 30, ownerId: THEM }),
	];
	const rect = { x1: -100, y1: -100, x2: 100, y2: 100 };
	assert.deepEqual(marqueeSelect(pawns, GROUND, rect, "player", ME), ["mine"]);
	assert.deepEqual(marqueeSelect(pawns, GROUND, rect, "gm", "01GM").length, 3);
});
test("a marquee takes nothing from another floor", () => {
	const pawns = [
		pawn({ id: "here", x: 10, y: 10 }),
		pawn({ id: "upstairs", x: 20, y: 20, layerId: CELLAR }),
	];
	const rect = { x1: -100, y1: -100, x2: 100, y2: 100 };
	assert.deepEqual(marqueeSelect(pawns, GROUND, rect, "gm", "01GM"), ["here"]);
});
test("a marquee stops at the protocol's own limit", () => {
	const pawns: Pawn[] = [];
	for (let i = 0; i < SELECTION_MAX + 50; i++) {
		pawns.push(pawn({ id: `p${i}`, x: i, y: 0 }));
	}
	const found = marqueeSelect(pawns, GROUND, { x1: -1, y1: -1, x2: 1e6, y2: 1 }, "gm", "01GM");
	assert.equal(found.length, SELECTION_MAX);
});
function wagon(): Pawn {
	return pawn({ id: "wagon", kind: "object", width: 128, height: 256, z: 1, x: 0, y: 0 });
}
test("riders are the pawns standing on a wagon and not the ones beside it", () => {
	const pawns = [
		wagon(),
		pawn({ id: "aboard", x: 30, y: 60, z: 5 }),
		pawn({ id: "alsoAboard", x: -60, y: -100, z: 4 }),
		pawn({ id: "beside", x: 200, y: 0, z: 6 }),
	];
	assert.deepEqual(riders(pawns, wagon(), CELL).sort(), ["aboard", "alsoAboard"]);
});
test("riders are whatever is above the wagon in the draw order", () => {
	const rug = { kind: "object" as const, width: 32, height: 32, x: 10, y: 10 };
	const pawns = [
		wagon(),
		pawn({ id: "spawnedFirst", x: 10, y: 10, z: 0 }),
		pawn({ id: "spawnedLast", x: 10, y: 10, z: 2 }),
		pawn({ id: "rugUnder", ...rug, z: 0 }),
		pawn({ id: "rugOver", ...rug, z: 2 }),
	];
	assert.deepEqual(riders(pawns, wagon(), CELL).sort(), ["rugOver", "spawnedFirst", "spawnedLast"]);
});
test("riders ignore pawns on another floor", () => {
	const pawns = [
		wagon(),
		pawn({ id: "upstairs", x: 10, y: 10, z: 5, layerId: CELLAR }),
	];
	assert.deepEqual(riders(pawns, wagon(), CELL), []);
});
test("a medium creature carries nobody", () => {
	const goblin = pawn({ id: "goblin", z: 1 });
	const pawns = [goblin, pawn({ id: "other", x: 4, y: 4, z: 2 })];
	assert.deepEqual(riders(pawns, goblin, CELL), []);
});
test("a large creature is wide enough to carry", () => {
	const giant = pawn({ id: "giant", size: "large", z: 1 });
	const pawns = [giant, pawn({ id: "rider", x: 10, y: 10, z: 2 })];
	assert.deepEqual(riders(pawns, giant, CELL), ["rider"]);
});
test("the drag set is the selection when the anchor is in it", () => {
	const a = pawn({ id: "a" });
	const b = pawn({ id: "b", x: 500 });
	const c = pawn({ id: "c", x: 900 });
	const pawns = [a, b, c];
	const selection = new Selection();
	selection.set(["a", "b"]);
	const options = { withRiders: true, role: "gm" as const, user: "01GM", cellSize: CELL };
	assert.deepEqual(dragSet(pawns, a, selection, options), ["a", "b"]);
	assert.deepEqual(dragSet(pawns, c, selection, options), ["c"]);
});
test("riders join a drag that selected nothing, and Alt leaves them", () => {
	const pawns = [wagon(), pawn({ id: "aboard", x: 10, y: 10, z: 5 })];
	const selection = new Selection();
	const base = { role: "gm" as const, user: "01GM", cellSize: CELL };
	assert.deepEqual(dragSet(pawns, wagon(), selection, { ...base, withRiders: true }), ["wagon", "aboard"]);
	assert.deepEqual(dragSet(pawns, wagon(), selection, { ...base, withRiders: false }), ["wagon"]);
});
test("a rider the viewer may not move does not join the drag", () => {
	const cart = pawn({ id: "cart", kind: "object", width: 128, height: 128, z: 1, ownerId: ME });
	const pawns = [cart, pawn({ id: "theirs", x: 10, y: 10, z: 5, ownerId: THEM })];
	const found = dragSet(pawns, cart, new Selection(), {
		withRiders: true, role: "player", user: ME, cellSize: CELL,
	});
	assert.deepEqual(found, ["cart"]);
});
test("the anchor is always first and never doubled", () => {
	const a = pawn({ id: "a" });
	const pawns = [a, pawn({ id: "b", x: 500 })];
	const selection = new Selection();
	selection.set(["b", "a"]);
	const found = dragSet(pawns, a, selection, {
		withRiders: true, role: "gm", user: "01GM", cellSize: CELL,
	});
	assert.equal(found[0], "a");
	assert.equal(new Set(found).size, found.length);
});
test("the drag set stops at the protocol's own limit", () => {
	const anchor = pawn({ id: "anchor" });
	const pawns: Pawn[] = [anchor];
	const ids: string[] = ["anchor"];
	for (let i = 0; i < SELECTION_MAX + 50; i++) {
		pawns.push(pawn({ id: `p${i}`, x: i }));
		ids.push(`p${i}`);
	}
	const selection = new Selection();
	selection.set(ids);
	const found = dragSet(pawns, anchor, selection, {
		withRiders: false, role: "gm", user: "01GM", cellSize: CELL,
	});
	assert.equal(found.length, SELECTION_MAX);
});
test("a selection reports whether it actually changed", () => {
	const selection = new Selection();
	assert.equal(selection.set(["a", "b"]), true);
	assert.equal(selection.set(["b", "a"]), false, "the same ids in another order is not a change");
	assert.equal(selection.clear(), true);
	assert.equal(selection.clear(), false);
});
test("shift-clicking toggles one in and out", () => {
	const selection = new Selection();
	selection.set(["a"]);
	selection.toggle("b");
	assert.deepEqual(selection.ids(), ["a", "b"]);
	selection.toggle("a");
	assert.deepEqual(selection.ids(), ["b"]);
});
test("pruning drops ids that are no longer on the table", () => {
	const selection = new Selection();
	selection.set(["a", "b", "c"]);
	assert.equal(selection.prune(new Set(["a", "c"])), true);
	assert.deepEqual(selection.ids(), ["a", "c"]);
	assert.equal(selection.prune(new Set(["a", "c"])), false);
});
test("only answers the single id and nothing else", () => {
	const selection = new Selection();
	assert.equal(selection.only(), null);
	selection.set(["a"]);
	assert.equal(selection.only(), "a");
	selection.add(["b"]);
	assert.equal(selection.only(), null);
});
