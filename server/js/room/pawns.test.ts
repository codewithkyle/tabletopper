// What a pointer on the table means.
//
// THE FOUR THINGS PINNED HERE ARE THE FOUR THAT ARE EASY TO GET SUBTLY WRONG.
// Hit testing has to match what was drawn or a click lands on the wrong pawn;
// the drag threshold has to exist or a click moves things; the delta has to be
// the ANCHOR'S so a wagon's passengers keep their seats; and a cancelled drag
// has to SEND something, because the event it produces is what tells everybody
// else to drop the ghosts they are drawing.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { Grid, Pawn, State } from "./protocol.ts";
import { empty } from "./store.ts";

// createTable listens for Escape on the document and reads the device pixel
// ratio off the window, neither of which Node has. Both are the whole of what
// it touches, so both are stood up here and the import follows.
const keydown: ((e: { key: string }) => void)[] = [];

(globalThis as unknown as { document: unknown }).document = {
	addEventListener(type: string, fn: (e: { key: string }) => void) {
		if (type === "keydown") {
			keydown.push(fn);
		}
	},
	removeEventListener() {},
};

(globalThis as unknown as { window: unknown }).window = { devicePixelRatio: 1 };

const { createTable, hitTest } = await import("./pawns.ts");

const GROUND = "01LAYERGROUND";
const CELLAR = "01LAYERCELLAR";
const GM = "01GM";

function grid(over: Partial<Grid> = {}): Grid {
	return {
		visible: true,
		cellSize: 64,
		offsetX: 0,
		offsetY: 0,
		color: "#000000FF",
		snap: "cells",
		feetPerCell: 5,
		diagonals: "equal",
		...over,
	};
}

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

const NONE = { shift: false, alt: false };
const SHIFT = { shift: true, alt: false };
const ALT = { shift: false, alt: true };

const at = (x: number, y: number) => ({ x, y });

// A medium creature is one cell, so its radius is half a cell: 32 pixels.
test("hit testing uses a disc for a creature", () => {
	const pawns = [pawn({ id: "goblin", x: 100, y: 100 })];

	assert.equal(hitTest(pawns, GROUND, grid(), 100, 100)?.id, "goblin");
	assert.equal(hitTest(pawns, GROUND, grid(), 125, 100)?.id, "goblin");

	// The corner of its bounding box is 45 pixels away and outside the disc,
	// which is exactly what the shader drew.
	assert.equal(hitTest(pawns, GROUND, grid(), 131, 131), null);
});

test("hit testing uses a rectangle for an object", () => {
	const wagon = pawn({ id: "wagon", kind: "object", width: 128, height: 256, x: 0, y: 0 });

	// Half extents are 64 across and 128 down.
	assert.equal(hitTest([wagon], GROUND, grid(), 60, 120)?.id, "wagon");
	assert.equal(hitTest([wagon], GROUND, grid(), 63, 127)?.id, "wagon", "a corner of the wagon is the wagon");
	assert.equal(hitTest([wagon], GROUND, grid(), 70, 0), null);
});

// The topmost is what a click means, and it is the same order the pawn pass
// drew in: z, then id for a tie.
test("hit testing prefers the topmost pawn", () => {
	const pawns = [
		pawn({ id: "under", z: 1 }),
		pawn({ id: "over", z: 5 }),
		pawn({ id: "middle", z: 3 }),
	];

	assert.equal(hitTest(pawns, GROUND, grid(), 0, 0)?.id, "over");
});

test("hit testing ignores another floor", () => {
	const pawns = [pawn({ id: "upstairs", layerId: CELLAR })];

	assert.equal(hitTest(pawns, GROUND, grid(), 0, 0), null);
});

// A table wired to a recording send, so a test can read what crossed the wire.
function table(pawns: Pawn[], over: Partial<{ role: "gm" | "player"; user: string; grid: Grid }> = {}) {
	const state: State = empty();
	state.pawns = pawns;
	state.table.grid = over.grid ?? grid();
	state.table.activeLayer = GROUND;

	const sent: Record<string, unknown>[] = [];

	const controller = createTable({
		state,
		role: over.role ?? "gm",
		user: over.user ?? GM,
		viewed: () => GROUND,
		send: (command) => {
			sent.push(command as unknown as Record<string, unknown>);
		},
		invalidate: () => {},
	});

	return { controller, sent, state };
}

// A CLICK STILL SELECTS, which is the whole reason the threshold exists: a
// click that moved a goblin one cell is a click nobody notices until the fight
// is over.
test("a press that goes nowhere selects rather than moving", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, sent } = table([goblin]);

	controller.tool.press(at(32, 32), at(200, 200), NONE);
	controller.tool.drag(at(34, 34), at(202, 202), NONE);
	controller.tool.release(at(34, 34), at(202, 202), NONE);

	assert.deepEqual(sent, [], "a click sent a command");
	assert.deepEqual(controller.selection.ids(), ["goblin"]);
});

test("shift-clicking toggles rather than replacing", () => {
	const a = pawn({ id: "a", x: 32, y: 32 });
	const b = pawn({ id: "b", x: 300, y: 32 });
	const { controller } = table([a, b]);

	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.release(at(32, 32), at(0, 0), NONE);

	controller.tool.press(at(300, 32), at(0, 0), SHIFT);
	controller.tool.release(at(300, 32), at(0, 0), SHIFT);

	assert.deepEqual(controller.selection.ids(), ["a", "b"]);
});

// Empty table with nothing under it is the camera's gesture, and the click that
// ends it clears what was chosen.
test("a click on empty table clears the selection", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller } = table([goblin]);

	controller.selection.set(["goblin"]);

	const claimed = controller.tool.press(at(900, 900), at(0, 0), NONE);
	controller.tool.release(at(900, 900), at(0, 0), NONE);

	assert.equal(claimed, false, "the camera was refused a pan from empty table");
	assert.deepEqual(controller.selection.ids(), []);
});

// A pan that actually panned is not a click, and must not throw the selection
// away on the way past.
test("a drag from empty table keeps the selection", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller } = table([goblin]);

	controller.selection.set(["goblin"]);

	controller.tool.press(at(900, 900), at(0, 0), NONE);
	controller.tool.drag(at(800, 900), at(100, 0), NONE);
	controller.tool.release(at(800, 900), at(100, 0), NONE);

	assert.deepEqual(controller.selection.ids(), ["goblin"]);
});

// ONE DELTA, TAKEN FROM THE ANCHOR, APPLIED TO EVERYTHING. A wagon with three
// people on it must arrive with them in the same three spots; snapping each one
// on its own would shuffle them into the wagon's cells.
test("the others move by the anchor's snapped delta and nothing else", () => {
	const anchor = pawn({ id: "anchor", x: 32, y: 32, z: 1 });
	const rider = pawn({ id: "rider", x: 100, y: 200, z: 2 });
	const { controller } = table([anchor, rider]);

	controller.selection.set(["anchor", "rider"]);

	// Grabbed exactly at the anchor's centre, so the pointer and the pawn move
	// together.
	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.drag(at(90, 90), at(58, 58), NONE);

	const ghosts = controller.ghosts([]);
	const byID = new Map(ghosts.map((g) => [g.id, g]));

	// 90 snaps to the cell centre at 96, so the delta is 64 on both axes.
	assert.deepEqual([byID.get("anchor")?.x, byID.get("anchor")?.y], [96, 96]);
	assert.deepEqual([byID.get("rider")?.x, byID.get("rider")?.y], [164, 264]);
});

test("a drag reports itself and commits on release", () => {
	const anchor = pawn({ id: "anchor", x: 32, y: 32 });
	const { controller, sent } = table([anchor]);

	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.drag(at(90, 90), at(58, 58), NONE);
	controller.tool.release(at(90, 90), at(58, 58), NONE);

	assert.equal(sent[0]?.type, "pawn.drag");
	assert.deepEqual([sent[0]?.x, sent[0]?.y], [96, 96]);

	const move = sent[sent.length - 1];
	assert.equal(move?.type, "pawn.move");
	assert.deepEqual([move?.x, move?.y], [96, 96]);
	assert.deepEqual(move?.others, []);
});

// A CANCELLED DRAG SENDS THE COMMITTED POSITION RATHER THAN NOTHING. The
// pawn.moved that comes back carries unchanged positions, and THAT is what
// tells everybody else to drop the ghosts they are drawing -- which is why
// Escape needs no event of its own.
test("Escape sends the committed position with the same others", () => {
	const anchor = pawn({ id: "anchor", x: 32, y: 32, z: 1 });
	const rider = pawn({ id: "rider", x: 100, y: 200, z: 2 });
	const { controller, sent } = table([anchor, rider]);

	controller.selection.set(["anchor", "rider"]);
	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.drag(at(300, 300), at(268, 268), NONE);

	for (const fn of keydown) {
		fn({ key: "Escape" });
	}

	const move = sent[sent.length - 1];
	assert.equal(move?.type, "pawn.move");
	assert.deepEqual([move?.x, move?.y], [32, 32], "Escape did not put the anchor back");
	assert.deepEqual(move?.others, ["rider"]);

	// And the ghosts are gone, because the gesture is over.
	assert.deepEqual(controller.ghosts([]), []);
});

// The wagon rule, through the state machine rather than through the lookup:
// Alt is how a GM takes it out from under the party.
//
// THE PRESS IS ON AN EMPTY CORNER OF THE WAGON AND THAT IS NOT INCIDENTAL. A
// press at its centre hits the RIDER standing there -- the topmost pawn is what
// a click means, and a passenger is above the thing carrying it by definition.
// Grabbing a wagon means grabbing a part of it nobody is standing on, which is
// what a hand does anyway.
function wagonAndRider() {
	const wagon = pawn({ id: "wagon", kind: "object", width: 128, height: 128, x: 0, y: 0, z: 1 });
	const rider = pawn({ id: "rider", x: 10, y: 10, z: 5 });

	return [wagon, rider];
}

test("Alt drags a wagon without its riders", () => {
	const { controller, sent } = table(wagonAndRider());

	controller.tool.press(at(-50, -50), at(0, 0), ALT);
	controller.tool.drag(at(14, -50), at(64, 0), ALT);
	controller.tool.release(at(14, -50), at(64, 0), ALT);

	const move = sent[sent.length - 1];
	assert.equal(move?.anchor, "wagon");
	assert.deepEqual(move?.others, [], "Alt took the riders anyway");
});

test("a wagon without Alt carries what is on it", () => {
	const { controller, sent } = table(wagonAndRider());

	controller.tool.press(at(-50, -50), at(0, 0), NONE);
	controller.tool.drag(at(14, -50), at(64, 0), NONE);
	controller.tool.release(at(14, -50), at(64, 0), NONE);

	const move = sent[sent.length - 1];
	assert.equal(move?.anchor, "wagon");
	assert.deepEqual(move?.others, ["rider"]);

	// An even footprint straddles a vertex rather than centring in a cell, so
	// the wagon lands on 64 and not on 96.
	assert.deepEqual([move?.x, move?.y], [64, 0]);
});

// And the reason the two above press where they do, asserted rather than
// implied: a passenger is on top of what carries it.
test("a press on a rider grabs the rider and not the wagon", () => {
	const { controller, sent } = table(wagonAndRider());

	controller.tool.press(at(10, 10), at(0, 0), NONE);
	controller.tool.drag(at(74, 10), at(64, 0), NONE);
	controller.tool.release(at(74, 10), at(64, 0), NONE);

	assert.equal(sent[sent.length - 1]?.anchor, "rider");
});

// Shift on empty table draws a box, and the box selects on release.
test("a shift-drag marquees", () => {
	const a = pawn({ id: "a", x: 100, y: 100 });
	const b = pawn({ id: "b", x: 900, y: 900 });
	const { controller } = table([a, b]);

	assert.equal(controller.tool.press(at(0, 0), at(0, 0), SHIFT), true);
	controller.tool.drag(at(200, 200), at(200, 200), SHIFT);
	controller.tool.release(at(200, 200), at(200, 200), SHIFT);

	assert.deepEqual(controller.selection.ids(), ["a"]);
});

// PLACEMENT SURVIVES A SPAWN, which is the whole reason arming is worth a round
// trip: an encounter is eight goblins and eight clicks.
test("arming places on every click until Escape", () => {
	const { controller, sent } = table([]);

	controller.arm({
		kind: "monster", id: "01MONSTER", name: "Goblin", image: "",
		visible: false, size: "medium", width: 0, height: 0,
	});

	controller.tool.press(at(90, 90), at(0, 0), NONE);
	controller.tool.press(at(200, 40), at(0, 0), NONE);

	assert.equal(sent.length, 2);
	assert.equal(sent[0]?.type, "pawn.spawn");
	assert.equal(sent[0]?.layer, GROUND, "a spawn landed on a floor nobody is looking at");
	assert.deepEqual([sent[0]?.x, sent[0]?.y], [96, 96], "the placement was not snapped");
	assert.equal(sent[0]?.visible, false, "the dialog's visibility toggle was ignored");
	assert.equal(sent[0]?.monsterId, "01MONSTER");

	assert.equal(controller.isArmed(), true);

	for (const fn of keydown) {
		fn({ key: "Escape" });
	}

	assert.equal(controller.isArmed(), false);

	controller.tool.press(at(300, 300), at(0, 0), NONE);
	assert.equal(sent.length, 2, "a click after Escape still placed something");
});

// A player cannot start a drag on a pawn they do not own -- the press is the
// camera's, and the server would refuse the move anyway.
test("a player pressing somebody else's pawn pans instead", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32, ownerId: null });
	const { controller } = table([goblin], { role: "player", user: "01ME" });

	assert.equal(controller.tool.press(at(32, 32), at(0, 0), NONE), false);
	assert.deepEqual(controller.selection.ids(), []);
});

// Somebody else's drag is drawn from the event and dropped by the move that
// ends it, which is the pair that keeps a ghost from outliving the hand.
test("another player's drag draws ghosts until a move ends it", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller } = table([goblin]);

	controller.preview({
		type: "pawn.dragging", seq: 4, by: "01OTHER",
		pawns: [{ id: "goblin", x: 160, y: 160 }],
	});

	const ghosts = controller.ghosts([]);
	assert.equal(ghosts.length, 1);
	assert.deepEqual([ghosts[0].x, ghosts[0].y], [160, 160]);

	controller.preview({
		type: "pawn.moved", seq: 5, pawns: [{ id: "goblin", x: 160, y: 160 }],
	});

	assert.deepEqual(controller.ghosts([]), []);
});

// A client never draws a ghost for its own drag twice: it is already drawing it
// from the gesture, and the echo would double it.
test("a client ignores its own dragging event", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller } = table([goblin]);

	controller.preview({
		type: "pawn.dragging", seq: 4, by: GM,
		pawns: [{ id: "goblin", x: 160, y: 160 }],
	});

	assert.deepEqual(controller.ghosts([]), []);
});
