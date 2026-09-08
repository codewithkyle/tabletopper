// Who is selected, and what comes with them when one is dragged.
//
// THE MARQUEE AND THE RIDERS ARE THE TWO RULES WORTH PINNING. Both are the kind
// of geometry that is obviously right until somebody drags a box across a
// battle line or parks a goblin next to a wagon, and both have a wrong answer
// that looks reasonable: intersection instead of centres, and adjacency instead
// of containment.

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

// The GM may move anything; a player may move what they own. The client's copy
// of the rule exists so a selection cannot be built that the server would
// refuse -- which would teach somebody the wrong thing about their own table.
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

// THE CENTRE AND NOT THE FOOTPRINT. Intersection would put a gargantuan dragon
// into a box drawn between its toes; containment would make a box drawn across
// the middle of a battle line select nothing at all.
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

// A player who marquees across a room full of goblins and their own fighter
// gets the fighter. A selection with the goblins in it would be a selection
// whose every drag came back forbidden.
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

// The floor is the filter everywhere, and a marquee is one of the places the
// plan names. A GM looking at the cellar cannot box-select the party upstairs.
test("a marquee takes nothing from another floor", () => {
	const pawns = [
		pawn({ id: "here", x: 10, y: 10 }),
		pawn({ id: "upstairs", x: 20, y: 20, layerId: CELLAR }),
	];

	const rect = { x1: -100, y1: -100, x2: 100, y2: 100 };

	assert.deepEqual(marqueeSelect(pawns, GROUND, rect, "gm", "01GM"), ["here"]);
});

// THE PROTOCOL CAPS A SELECTION AND SO DOES THE BOX. A marquee across a stress
// test must not build a command the server answers "too many".
test("a marquee stops at the protocol's own limit", () => {
	const pawns: Pawn[] = [];
	for (let i = 0; i < SELECTION_MAX + 50; i++) {
		pawns.push(pawn({ id: `p${i}`, x: i, y: 0 }));
	}

	const found = marqueeSelect(pawns, GROUND, { x1: -1, y1: -1, x2: 1e6, y2: 1 }, "gm", "01GM");

	assert.equal(found.length, SELECTION_MAX);
});

// A wagon's picture is 128 by 256 pixels, so it reaches 64 across and 128 down
// from its centre. What is ON it is inside that box and above it in draw order.
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

// THE DRAW ORDER IS WHAT "ON" MEANS. A wagon spawned after the party sits above
// them, and dragging it must not pick up the people it was parked over -- which
// is the same rule read the other way.
test("riders are only what is above the wagon", () => {
	const pawns = [
		wagon(),
		pawn({ id: "under", x: 10, y: 10, z: 0 }),
		pawn({ id: "over", x: 10, y: 10, z: 2 }),
	];

	assert.deepEqual(riders(pawns, wagon(), CELL), ["over"]);
});

test("riders ignore pawns on another floor", () => {
	const pawns = [
		wagon(),
		pawn({ id: "upstairs", x: 10, y: 10, z: 5, layerId: CELLAR }),
	];

	assert.deepEqual(riders(pawns, wagon(), CELL), []);
});

// IT NEVER TRIGGERS ON A ONE-CELL CREATURE, which is what keeps two goblins in
// adjacent squares from picking each other up.
test("a medium creature carries nobody", () => {
	const goblin = pawn({ id: "goblin", z: 1 });
	const pawns = [goblin, pawn({ id: "other", x: 4, y: 4, z: 2 })];

	assert.deepEqual(riders(pawns, goblin, CELL), []);
});

// A large creature IS two cells on both axes, so it carries -- which is the
// rule as written, and is right: a giant picking up what is standing on its
// square is the same mechanic as a wagon.
test("a large creature is wide enough to carry", () => {
	const giant = pawn({ id: "giant", size: "large", z: 1 });
	const pawns = [giant, pawn({ id: "rider", x: 10, y: 10, z: 2 })];

	assert.deepEqual(riders(pawns, giant, CELL), ["rider"]);
});

// The selection comes along only when the anchor is in it, which is what makes
// dragging one pawn out of a selected group possible.
test("the drag set is the selection when the anchor is in it", () => {
	const a = pawn({ id: "a" });
	const b = pawn({ id: "b", x: 500 });
	const c = pawn({ id: "c", x: 900 });
	const pawns = [a, b, c];

	const selection = new Selection();
	selection.set(["a", "b"]);

	const options = { withRiders: true, role: "gm" as const, user: "01GM", cellSize: CELL };

	assert.deepEqual(dragSet(pawns, a, selection, options), ["a", "b"]);

	// Grabbing something that was not selected means you meant that thing.
	assert.deepEqual(dragSet(pawns, c, selection, options), ["c"]);
});

// AND THE RIDERS COME WHETHER OR NOT ANYTHING WAS SELECTED, because a wagon
// with three people on it is one object as far as a hand is concerned.
test("riders join a drag that selected nothing, and Alt leaves them", () => {
	const pawns = [wagon(), pawn({ id: "aboard", x: 10, y: 10, z: 5 })];
	const selection = new Selection();
	const base = { role: "gm" as const, user: "01GM", cellSize: CELL };

	assert.deepEqual(dragSet(pawns, wagon(), selection, { ...base, withRiders: true }), ["wagon", "aboard"]);
	assert.deepEqual(dragSet(pawns, wagon(), selection, { ...base, withRiders: false }), ["wagon"]);
});

// A player dragging their own wagon must not take somebody else's pawn with it:
// the server would refuse the whole move, so the whole move is never built.
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

// A pawn that left the table cannot stay selected: its next drag would be
// answered not_found, and the overlay would name something nobody can see.
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
