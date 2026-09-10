// What a pointer on the table means.
//
// THE FIVE THINGS PINNED HERE ARE THE FIVE THAT ARE EASY TO GET SUBTLY WRONG.
// Hit testing has to match what was drawn or a click lands on the wrong pawn;
// the drag threshold has to exist or a click moves things; the delta has to be
// the ANCHOR'S so a wagon's passengers keep their seats; a cancelled drag has to
// SEND something, because the event it produces is what tells everybody else to
// drop the ghosts they are drawing; and the two pointer modes have to differ in
// exactly one way -- who gets the press -- with the selection surviving the trip
// through the one that does not want it.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { FogMode, FogShape, Grid, Pawn, ShapeKind, State, Stroke } from "./protocol.ts";
import type { DrawMode } from "./draw.ts";
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

// THE DOUBLE CLICK IS MEASURED ON performance.now AND THESE TESTS OWN IT. The
// skew runs at zero, so every drag test below still sees real elapsed time and
// the throttle in moveDrag behaves as it does in a browser; the one test that
// needs two clicks to be too far apart pushes it forward by hand.
let skew = 0;
const realNow = performance.now.bind(performance);
performance.now = () => realNow() + skew;

function wait(ms: number): void {
	skew += ms;
}

const { createTable, hitTest } = await import("./pawns.ts");
const { createFog } = await import("./fog.ts");
const { createDraw } = await import("./draw.ts");

const GROUND = "01LAYERGROUND";
const CELLAR = "01LAYERCELLAR";
const GM = "01GM";
const PLAYER = "01PLAYER";

function grid(over: Partial<Grid> = {}): Grid {
	return {
		lines: "solid",
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

// press is a key arriving on the document, optionally from a control. The
// listener createTable installed is the only one there is.
function press(key: string, target: unknown = null, mods: Record<string, boolean> = {}): void {
	for (const fn of keydown) {
		(fn as (e: unknown) => void)({ key, target, ...mods });
	}
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
// drew in: objects underneath, then z, then id for a tie.
test("hit testing prefers the topmost pawn", () => {
	const pawns = [
		pawn({ id: "under", z: 1 }),
		pawn({ id: "over", z: 5 }),
		pawn({ id: "middle", z: 3 }),
	];

	assert.equal(hitTest(pawns, GROUND, grid(), 0, 0)?.id, "over");
});

// A TOKEN IS ALWAYS UNDER A CREATURE AND A CLICK FOLLOWS THE DRAW ORDER. A rug
// laid down after the party is drawn beneath them, so clicking where a goblin
// stands on it picks the goblin -- the rug is not what is on top there, and a
// hit test that disagreed with what a person can see would select something
// hidden behind what they aimed at.
test("a creature is picked over a token it is standing on", () => {
	const rug = pawn({ id: "rug", kind: "object", width: 256, height: 256, x: 0, y: 0, z: 99 });
	const goblin = pawn({ id: "goblin", x: 0, y: 0, z: 1 });

	assert.equal(hitTest([rug, goblin], GROUND, grid(), 0, 0)?.id, "goblin");

	// And the rug is still reachable everywhere the goblin is not.
	assert.equal(hitTest([rug, goblin], GROUND, grid(), 100, 100)?.id, "rug");
});

// A ROTATED TOKEN'S CORNERS MOVE WITH IT. The hit test turns the POINT into the
// token's frame rather than growing a box round it, which is what makes a click
// just off a turned corner land on the table.
test("hit testing turns with the token", () => {
	const flat = pawn({ id: "beam", kind: "object", width: 256, height: 32, x: 0, y: 0 });
	const upright = pawn({ ...flat, rotation: 90 });

	assert.equal(hitTest([flat], GROUND, grid(), 120, 0)?.id, "beam");
	assert.equal(hitTest([flat], GROUND, grid(), 0, 120), null);

	// Turned a quarter turn, the same beam answers the other way round.
	assert.equal(hitTest([upright], GROUND, grid(), 120, 0), null);
	assert.equal(hitTest([upright], GROUND, grid(), 0, 120)?.id, "beam");
});

test("hit testing ignores another floor", () => {
	const pawns = [pawn({ id: "upstairs", layerId: CELLAR })];

	assert.equal(hitTest(pawns, GROUND, grid(), 0, 0), null);
});

// A table wired to a recording send, so a test can read what crossed the wire,
// and to recording versions of the two things it asks the page to do.
function table(
	pawns: Pawn[],
	over: Partial<{
		role: "gm" | "player"; user: string; grid: Grid;
		scale: number; panning: boolean; measuring: boolean;

		// The fog's half of the harness: whether the Fog tool is the one
		// chosen, what the options pill says, and how the ground floor's own
		// two flags are set.
		fogging: boolean; shape: ShapeKind; mode: FogMode;
		fogEnabled: boolean; fogPrefill: boolean; fog: FogShape[];

		// And the pen's: whether the Draw tool is the one chosen, what its own
		// pill says it is doing, and what is already drawn on the floor.
		inking: boolean; drawMode: DrawMode; strokes: Stroke[];
	}> = {},
) {
	const state: State = empty();
	state.pawns = pawns;
	state.table.grid = over.grid ?? grid();
	state.table.activeLayer = GROUND;
	state.table.layers = [{
		id: GROUND, name: "Ground floor", map: null,
		fogEnabled: over.fogEnabled ?? false,
		fogPrefill: over.fogPrefill ?? true,
	}];
	state.fog = over.fog ?? [];
	state.strokes = over.strokes ?? [];

	const sent: Record<string, unknown>[] = [];
	const opened: string[] = [];
	const menus: string[] = [];
	let removals = 0;

	// The pill's mode, which a test can throw mid-gesture: letting go of the
	// space bar halfway through a marquee is a thing hands do, and switching
	// away from the ruler is how one is put away.
	let panning = over.panning ?? false;
	let measuring = over.measuring ?? false;
	let fogging = over.fogging ?? false;
	let inking = over.inking ?? false;
	const drawOptions = { mode: over.drawMode ?? "pen", color: "#FF0000", width: 4 };
	const options = { shape: over.shape ?? "rect", mode: over.mode ?? "reveal" };

	const send = (command: unknown) => {
		sent.push(command as Record<string, unknown>);
	};

	// THE REAL FOG AND NOT A STUB. What these tests are about is the seam --
	// which gesture reaches which module, and whether a concealed pawn is
	// skipped in all four places -- so a fake fog would be testing the fake.
	const fog = createFog({
		state,
		role: over.role ?? "gm",
		user: over.user ?? GM,
		viewed: () => GROUND,
		grid: () => state.table.grid,
		send,
		invalidate: () => {},
		fogging: () => fogging,
		options: () => options,
	});

	// THE REAL PEN TOO, for the fog's reason: what is under test is which
	// gesture reaches which module.
	const draw = createDraw({
		state,
		role: over.role ?? "gm",
		user: over.user ?? GM,
		viewed: () => GROUND,
		grid: () => state.table.grid,
		send,
		invalidate: () => {},
		scale: () => over.scale ?? 1,
		drawing: () => inking,
		options: () => drawOptions,
	});

	const controller = createTable({
		state,
		role: over.role ?? "gm",
		user: over.user ?? GM,
		fog,
		draw,
		viewed: () => GROUND,
		send,
		invalidate: () => {},
		panning: () => panning,
		measuring: () => measuring,

		// ONE MAP PIXEL PER SCREEN PIXEL, so a handle's grab radius in these
		// tests is the constant itself and the arithmetic is readable.
		scale: () => over.scale ?? 1,

		details: (p) => {
			opened.push(p.id);
		},
		menu: (p) => {
			menus.push(p.id);
		},
		remove: () => {
			removals += 1;
		},
	});

	return {
		controller,
		sent,
		state,
		opened,
		menus,
		removals: () => removals,
		pan: (on: boolean) => {
			panning = on;
		},
		measure: (on: boolean) => {
			measuring = on;
		},
		fogTool: (on: boolean) => {
			fogging = on;
		},
		drawTool: (on: boolean) => {
			inking = on;
		},
		chooseDraw: (mode: DrawMode) => {
			drawOptions.mode = mode;
		},
		chooseBrush: (color: string, width: number) => {
			drawOptions.color = color;
			drawOptions.width = width;
		},
		chooseFog: (shape: ShapeKind, mode: FogMode) => {
			options.shape = shape;
			options.mode = mode;
		},
	};
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

// Empty table under the select tool is a press this module keeps, and the click
// that ends it clears what was chosen.
test("a click on empty table clears the selection", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller } = table([goblin]);

	controller.selection.set(["goblin"]);

	const claimed = controller.tool.press(at(900, 900), at(0, 0), NONE);
	controller.tool.release(at(900, 900), at(0, 0), NONE);

	assert.equal(claimed, true, "the camera took a press the marquee needs");
	assert.deepEqual(controller.selection.ids(), []);
});

// THE GROUP SELECTION IS A PLAIN DRAG AND NO LONGER A HELD KEY. Dragging a box
// round four goblins is the gesture every other tool on a canvas has, and the
// press it is made of is the one the camera used to take.
test("a drag from empty table marquees", () => {
	const a = pawn({ id: "a", x: 100, y: 100 });
	const b = pawn({ id: "b", x: 900, y: 900 });
	const { controller } = table([a, b]);

	assert.equal(controller.tool.press(at(0, 0), at(0, 0), NONE), true);
	controller.tool.drag(at(200, 200), at(200, 200), NONE);
	controller.tool.release(at(200, 200), at(200, 200), NONE);

	assert.deepEqual(controller.selection.ids(), ["a"]);
});

// A HAND SHAKES ON THE WAY OFF THE BUTTON, and under the threshold that is a
// click rather than a box a pixel wide over whatever it was resting on.
test("a click that shook is a click and not a marquee", () => {
	const goblin = pawn({ id: "goblin", x: 0, y: 0 });
	const { controller } = table([goblin]);

	controller.selection.set(["goblin"]);

	controller.tool.press(at(900, 900), at(400, 400), NONE);
	controller.tool.drag(at(902, 902), at(402, 402), NONE);
	controller.tool.release(at(902, 902), at(402, 402), NONE);

	assert.deepEqual(controller.selection.ids(), [], "a click on empty table did not clear the selection");
	assert.deepEqual(controller.outlines([]), [], "a click drew a marquee");
});

// A MARQUEE THAT CAUGHT NOTHING IS A SELECTION OF NOTHING. Dragging a box
// across empty floor is how somebody says "none of them" without hunting for a
// patch of table to click.
test("a marquee that found nothing clears the selection", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller } = table([goblin]);

	controller.selection.set(["goblin"]);

	controller.tool.press(at(900, 900), at(0, 0), NONE);
	controller.tool.drag(at(1200, 1200), at(300, 300), NONE);
	controller.tool.release(at(1200, 1200), at(300, 300), NONE);

	assert.deepEqual(controller.selection.ids(), []);
});

// THE MOVE TOOL IS THE CAMERA'S AND THE TABLE HEARS NOTHING. Every press is
// refused, whatever it landed on -- so nothing is dragged, nothing is picked
// out, and no box is drawn.
test("the move tool gives every press to the camera", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, sent } = table([goblin], { panning: true });

	assert.equal(controller.tool.press(at(32, 32), at(0, 0), NONE), false, "a press on a pawn was kept");
	controller.tool.drag(at(200, 200), at(168, 168), NONE);
	controller.tool.release(at(200, 200), at(168, 168), NONE);

	assert.equal(controller.tool.press(at(900, 900), at(0, 0), NONE), false, "a press on empty table was kept");
	controller.tool.drag(at(1200, 1200), at(300, 300), NONE);
	controller.tool.release(at(1200, 1200), at(300, 300), NONE);

	assert.deepEqual(sent, [], "the camera's mode moved something");
	assert.deepEqual(controller.selection.ids(), [], "the camera's mode selected something");
	assert.deepEqual(controller.outlines([]), [], "the camera's mode drew a marquee");
});

// AND IT LEAVES THE SELECTION WHERE IT FOUND IT. Shoving the map across to see
// where the party is going is not a reason to throw away the four goblins
// somebody just picked out.
test("the move tool keeps the selection", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller } = table([goblin], { panning: true });

	controller.selection.set(["goblin"]);

	controller.tool.press(at(900, 900), at(0, 0), NONE);
	controller.tool.release(at(900, 900), at(0, 0), NONE);

	assert.deepEqual(controller.selection.ids(), ["goblin"]);
});

// A GESTURE FINISHES UNDER THE TOOL IT BEGAN IN, which is why the mode is asked
// at the press and never again: the space bar is a key a hand lets go of, and
// letting go of it halfway through a drag must not drop the goblin.
test("a drag survives the tool changing under it", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, sent, pan } = table([goblin]);

	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.drag(at(90, 90), at(58, 58), NONE);

	pan(true);

	controller.tool.release(at(90, 90), at(58, 58), NONE);

	assert.equal(sent[sent.length - 1]?.type, "pawn.move", "the drag was abandoned mid-flight");
});

// THE RULER IS NOT A GESTURE, which is the one thing about it that is easy to
// build wrong: the point stays down when the button comes up, and the line goes
// on following the pointer while the hand is off the mouse entirely. That is
// what "how far is that" is asked with.
test("the measure tool puts a point down and runs a line to the pointer", () => {
	const { controller } = table([], { measuring: true });

	assert.equal(controller.tool.press(at(100, 100), at(0, 0), NONE), true, "the ruler gave the press away");
	controller.tool.release(at(100, 100), at(0, 0), NONE);

	// Three cells to the right, with the button already up.
	controller.tool.hover(at(292, 100));

	const [ruler, ...rest] = controller.rulers([]);

	assert.deepEqual(rest, [], "one measurement drew more than one ruler");
	assert.deepEqual([ruler?.x0, ruler?.y0], [100, 100]);
	assert.deepEqual([ruler?.x1, ruler?.y1], [292, 100]);
	assert.equal(ruler?.label, "15 ft.");

	// And the end that is not under the pointer is marked.
	const [point, ...others] = controller.outlines([]);

	assert.deepEqual(others, [], "the ruler drew more than the one point");
	assert.deepEqual([point?.x, point?.y], [100, 100]);
	assert.equal(point?.rect, false, "the point is not a ring");
});

// IT COUNTS NO SQUARES AND HIGHLIGHTS NONE. A move is a creature walking
// through cells and is scored by the table's diagonal rule; a ruler is a line
// across a map, and three cells diagonally is twenty-one feet rather than the
// fifteen the same walk costs.
test("a measurement is a straight line and not a square count", () => {
	const { controller } = table([], { measuring: true });

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.hover(at(192, 192));

	const [ruler] = controller.rulers([]);

	assert.equal(ruler?.label, "21 ft.");
	assert.deepEqual(ruler?.cells, [], "the free ruler tinted cells");
});

// AND NEITHER END IS SNAPPED, which is the whole reason it is a mode of its own:
// a fireball's radius, a bow's range and the gap between two rocks are not
// measured in squares, and a ruler that jumped to the lattice could not answer
// any of them.
test("a measurement snaps to nothing at either end", () => {
	const { controller } = table([], { measuring: true });

	controller.tool.press(at(37, 91), at(0, 0), NONE);
	controller.tool.hover(at(52, 103));

	const [ruler] = controller.rulers([]);

	assert.deepEqual([ruler?.x0, ruler?.y0, ruler?.x1, ruler?.y1], [37, 91, 52, 103]);
});

// THE TOOL IS TWO CLICKS AND THE SECOND IS THE FULL STOP, which is how a wall
// is measured in every CAD program there has ever been: the gesture ends where
// the eye already is rather than at a key.
test("a second press ends the measurement", () => {
	const { controller } = table([], { measuring: true });

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.hover(at(64, 0));

	assert.equal(controller.tool.press(at(320, 0), at(0, 0), NONE), true, "the second press was given away");

	assert.deepEqual(controller.rulers([]), [], "the second press left the ruler up");
	assert.deepEqual(controller.outlines([]), [], "the second press left the point down");

	// And the pointer travelling afterwards does not start one by itself.
	controller.tool.hover(at(640, 0));
	assert.deepEqual(controller.rulers([]), [], "the ruler came back on its own");
});

// A THIRD PRESS IS A NEW MEASUREMENT, so a GM asking one question after another
// is clicking rather than reaching for Escape between them.
test("a press after the end starts a fresh measurement", () => {
	const { controller } = table([], { measuring: true });

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.press(at(0, 0), at(0, 0), NONE);

	controller.tool.press(at(320, 0), at(0, 0), NONE);
	controller.tool.hover(at(384, 0));

	const [ruler, ...rest] = controller.rulers([]);

	assert.deepEqual(rest, [], "the new measurement drew more than one ruler");
	assert.deepEqual([ruler?.x0, ruler?.x1], [320, 384]);
	assert.equal(ruler?.label, "5 ft.");
});

// THE RULER TOUCHES NOTHING ON THE TABLE. It takes the primary button the way
// Move does and spends it on itself: no drag, no selection, no box, and the
// group somebody had picked out is still picked out afterwards.
test("the measure tool moves nothing and selects nothing", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const ogre = pawn({ id: "ogre", x: 900, y: 900 });
	const { controller, sent } = table([goblin, ogre], { measuring: true });

	controller.selection.set(["ogre"]);

	assert.equal(controller.tool.press(at(32, 32), at(0, 0), NONE), true, "the ruler gave a pawn away");
	controller.tool.drag(at(200, 200), at(168, 168), NONE);
	controller.tool.release(at(200, 200), at(168, 168), NONE);

	assert.deepEqual(sent, [], "the ruler moved something");
	assert.deepEqual(controller.selection.ids(), ["ogre"], "the ruler changed the selection");
});

// AND IT IS PUT AWAY THE WAY EVERYTHING ELSE ON THIS TABLE IS.
test("Escape puts the ruler away", () => {
	const { controller } = table([], { measuring: true });

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.hover(at(192, 0));

	press("Escape");

	assert.deepEqual(controller.rulers([]), [], "Escape left the ruler up");
	assert.deepEqual(controller.outlines([]), [], "Escape left the point down");
});

// CHOOSING ANOTHER TOOL IS THE OTHER WAY, and it FORGETS rather than hides: a
// GM who measured, moved a goblin and came back to the ruler is asking a new
// question, not resuming the one they left.
test("leaving the measure tool forgets the measurement", () => {
	const { controller, measure } = table([], { measuring: true });

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.hover(at(192, 0));

	measure(false);
	assert.deepEqual(controller.rulers([]), [], "another tool kept the ruler up");

	measure(true);
	assert.deepEqual(controller.rulers([]), [], "the old ruler came back");
});

// THE SPACE BAR IS NOT ANOTHER TOOL. It borrows the pointer for the camera and
// leaves the chosen tool alone, which is what lets a GM shove the map along a
// corridor with the ruler still stretched across it -- the whole gesture for
// measuring something further away than the screen.
test("panning with the space bar does not put the ruler away", () => {
	const { controller, pan } = table([], { measuring: true });

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.hover(at(192, 0));

	pan(true);

	assert.equal(controller.tool.press(at(400, 400), at(0, 0), NONE), false, "the hold did not reach the camera");

	const [ruler] = controller.rulers([]);

	assert.deepEqual([ruler?.x0, ruler?.x1], [0, 192], "the pan moved or dropped the ruler");
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

	press("Escape");

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

// SHIFT NO LONGER STARTS THE BOX AND STILL SAYS WHAT IT MEANS EVERYWHERE ELSE:
// it adds. A marquee held with Shift takes what it crossed on top of what was
// already picked out, and one without it replaces the lot.
test("a shift-drag adds to the selection rather than replacing it", () => {
	const a = pawn({ id: "a", x: 100, y: 100 });
	const b = pawn({ id: "b", x: 900, y: 900 });
	const { controller } = table([a, b]);

	controller.selection.set(["b"]);

	assert.equal(controller.tool.press(at(0, 0), at(0, 0), SHIFT), true);
	controller.tool.drag(at(200, 200), at(200, 200), SHIFT);
	controller.tool.release(at(200, 200), at(200, 200), SHIFT);

	assert.deepEqual(controller.selection.ids(), ["b", "a"]);
});

// AND A SHIFT CLICK THAT CAUGHT NOTHING IS NOT A REASON TO EMPTY IT. A miss
// while building a group is a miss.
test("a shift click on empty table keeps the selection", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller } = table([goblin]);

	controller.selection.set(["goblin"]);

	controller.tool.press(at(900, 900), at(0, 0), SHIFT);
	controller.tool.release(at(900, 900), at(0, 0), SHIFT);

	assert.deepEqual(controller.selection.ids(), ["goblin"]);
});

// PLACEMENT SURVIVES A SPAWN, which is the whole reason arming is worth a round
// trip: an encounter is eight goblins and eight clicks.
test("arming places on every click until Escape", () => {
	const { controller, sent } = table([]);

	controller.arm({
		kind: "monster", id: "01MONSTER", name: "Goblin", image: "",
		visible: false, size: "medium", width: 0, height: 0,
		hp: 0, maxHp: 0, ac: 0,
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

	press("Escape");

	assert.equal(controller.isArmed(), false);

	controller.tool.press(at(300, 300), at(0, 0), NONE);
	assert.equal(sent.length, 2, "a click after Escape still placed something");
});

// AN NPC IS THE ONE KIND THAT CARRIES ITS OWN STAT LINE, because a face out of
// the avatar library has no row anywhere for the hub to read one from. A
// monster's numbers stay off the wire in the same breath: those are in the
// manual, and a browser describing them would be a browser the server had to
// distrust.
test("an armed NPC sends the numbers the form asked for", () => {
	const { controller, sent } = table([]);

	controller.arm({
		kind: "npc", id: "01AVATAR", name: "Innkeeper", image: "/assets/images/01AVATAR",
		visible: true, size: "small", width: 0, height: 0,
		hp: 9, maxHp: 12, ac: 13,
	});

	controller.tool.press(at(90, 90), at(0, 0), NONE);

	assert.equal(sent[0]?.type, "pawn.spawn");
	assert.equal(sent[0]?.assetId, "01AVATAR");
	assert.equal(sent[0]?.size, "small");
	assert.deepEqual([sent[0]?.hp, sent[0]?.maxHp, sent[0]?.ac], [9, 12, 13]);
});

// PLACING IS NOT PANNING. A GM with a goblin on the cursor who holds the space
// bar to see where the rest of the room is has not asked to drop it there.
test("the move tool places nothing", () => {
	const { controller, sent } = table([], { panning: true });

	controller.arm({
		kind: "monster", id: "01MONSTER", name: "Goblin", image: "",
		visible: false, size: "medium", width: 0, height: 0,
		hp: 0, maxHp: 0, ac: 0,
	});

	assert.equal(controller.tool.press(at(90, 90), at(0, 0), NONE), false);

	assert.deepEqual(sent, []);
	assert.equal(controller.isArmed(), true, "the camera's mode disarmed what was held");
});

// A player cannot start a drag on a pawn they do not own, and the server would
// refuse the move anyway. What the press becomes instead is the same thing a
// press on empty table is: a click that selects nothing, or a marquee.
test("a player pressing somebody else's pawn selects nothing", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32, ownerId: null });
	const { controller, sent } = table([goblin], { role: "player", user: "01ME" });

	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.drag(at(200, 200), at(168, 168), NONE);
	controller.tool.release(at(200, 200), at(168, 168), NONE);

	assert.deepEqual(sent, [], "a player moved a pawn that is not theirs");
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

// A TOKEN MOVES FREELY AND A CREATURE DOES NOT, with whole-cell snapping
// switched on for both. It is internal/room.snapPawn's rule and the drag has to
// preview what the server is going to store, or the token jumps when the echo
// arrives.
test("a token commits where the hand let go and a creature commits to the lattice", () => {
	const wagon = pawn({ id: "wagon", kind: "object", width: 128, height: 128, x: 0, y: 0 });
	const { controller, sent } = table([wagon]);

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(101, 99), at(101, 99), NONE);
	controller.tool.release(at(101, 99), at(101, 99), NONE);

	const move = sent.at(-1);
	assert.equal(move?.type, "pawn.move");
	assert.deepEqual([move?.x, move?.y], [101, 99], "the token was pulled onto the grid");

	const goblin = pawn({ id: "goblin", x: 0, y: 0 });
	const creature = table([goblin]);

	creature.controller.tool.press(at(0, 0), at(0, 0), NONE);
	creature.controller.tool.drag(at(101, 99), at(101, 99), NONE);
	creature.controller.tool.release(at(101, 99), at(101, 99), NONE);

	const snappedMove = creature.sent.at(-1);
	assert.deepEqual([snappedMove?.x, snappedMove?.y], [96, 96], "the creature ignored the lattice");
});

// RIGHT CLICK IS ESCAPE FOR A HAND THAT IS ALREADY ON THE MOUSE, and placement
// wins over everything else: a GM halfway through putting down an encounter
// pressed it to STOP, and putting a menu over the table they were working on
// would be the opposite of what they asked for.
test("the right button abandons placement rather than opening anything", () => {
	const goblin = pawn({ id: "goblin", x: 0, y: 0 });
	const { controller, menus } = table([goblin]);

	controller.arm({
		kind: "object", id: "01ASSET", name: "Barrel", image: "",
		visible: true, size: "medium", width: 64, height: 64,
		hp: 0, maxHp: 0, ac: 0,
	});
	assert.equal(controller.isArmed(), true);

	controller.tool.secondary(at(0, 0), at(0, 0));

	assert.equal(controller.isArmed(), false, "placement survived a right click");
	assert.deepEqual(menus, [], "a menu opened over the encounter being placed");
});

// AND IT PUTS A DRAG BACK, which is the other half of what Escape does. The
// cancelled move is still SENT: the server answers with unchanged positions,
// and that event is what tells everybody else to drop the ghost they are
// drawing.
test("the right button puts a dragged pawn back", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, sent, menus } = table([goblin]);

	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.drag(at(300, 300), at(268, 268), NONE);

	controller.tool.secondary(at(300, 300), at(268, 268));

	const move = sent.at(-1);
	assert.equal(move?.type, "pawn.move");
	assert.deepEqual([move?.x, move?.y], [32, 32], "the pawn did not go back where it started");
	assert.deepEqual(menus, [], "abandoning a drag also opened a menu");
});

// WITH NOTHING TO ABANDON IT IS A QUESTION ABOUT WHAT IS UNDER THE POINTER,
// which is the one thing a right click does that Escape cannot.
//
// AND IT ASKS FOR A MENU RATHER THAN OPENING THE WINDOW, which is the change
// playtesting bought: the window is what a DOUBLE click opens, and it is the
// first item on this menu for anybody who learnt the old gesture.
test("the right button on a pawn asks for its menu and opens nothing", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, menus, opened } = table([goblin]);

	controller.tool.secondary(at(32, 32), at(0, 0));
	assert.deepEqual(menus, ["goblin"]);
	assert.deepEqual(opened, [], "the right button opened the window behind the menu");

	// Empty table asks nothing, and the browser's own menu is gone either way
	// -- that is input.ts's decision and not this module's.
	controller.tool.secondary(at(900, 900), at(0, 0));
	assert.deepEqual(menus, ["goblin"]);
});

// IT IS THE HIT TEST AND NOT A SEPARATE RULE, so a right click picks the same
// pawn a left click would: the goblin standing on the rug rather than the rug.
test("the right button follows the draw order", () => {
	const rug = pawn({ id: "rug", kind: "object", width: 256, height: 256, x: 0, y: 0, z: 9 });
	const goblin = pawn({ id: "goblin", x: 0, y: 0, z: 1 });
	const { controller, menus } = table([rug, goblin]);

	controller.tool.secondary(at(0, 0), at(0, 0));
	assert.deepEqual(menus, ["goblin"]);
});

// A RIGHT CLICK IS NOT A SELECTION. Answering "let me look at that" by throwing
// away whatever the GM had picked out would make it a destructive gesture.
test("the right button leaves the selection alone", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const wagon = pawn({ id: "wagon", kind: "object", width: 64, height: 64, x: 400, y: 400 });
	const { controller } = table([goblin, wagon]);

	controller.selection.set(["wagon"]);
	controller.tool.secondary(at(32, 32), at(0, 0));

	assert.deepEqual(controller.selection.ids(), ["wagon"]);
});

// A DOUBLE CLICK IS WHAT OPENS A PAWN NOW, and the six tests below are the
// whole of the gesture: it takes two, it takes them close together, it takes
// them on the same pawn, it does not take a third, it survives a hand that may
// not move the thing it is asking about, and Shift is left out of it.
//
// THE PAIR IS COUNTED HERE RATHER THAN BY THE BROWSER, so these are pinning a
// rule this module owns rather than one it inherits -- see DOUBLE_MS.

// click is a press and a release that went nowhere, which is exactly what the
// tool calls a click.
type Controller = ReturnType<typeof table>["controller"];

function click(controller: Controller, x: number, y: number, mods = NONE): void {
	controller.tool.press(at(x, y), at(x, y), mods);
	controller.tool.release(at(x, y), at(x, y), mods);
}

test("a double click opens the pawn's window and one click does not", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, opened } = table([goblin]);

	click(controller, 32, 32);
	assert.deepEqual(opened, [], "one click opened a window");

	click(controller, 32, 32);
	assert.deepEqual(opened, ["goblin"]);

	// THE SELECTION IS STILL THE PAWN. Opening the window is on top of what the
	// clicks already meant, not instead of it -- a GM who double-clicks a
	// goblin to read it and then presses Delete is holding the goblin.
	assert.deepEqual(controller.selection.ids(), ["goblin"]);
});

// A FINGER RESTING ON THE BUTTON IS NOT A REQUEST FOR TWO WINDOWS. Forgetting
// the pair on the way out is what makes the third click the first half of the
// next one rather than the second half of this one.
test("a third click is not a second double click", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, opened } = table([goblin]);

	click(controller, 32, 32);
	click(controller, 32, 32);
	click(controller, 32, 32);

	assert.deepEqual(opened, ["goblin"]);
});

test("two clicks far enough apart are two clicks", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, opened } = table([goblin]);

	click(controller, 32, 32);
	wait(500);
	click(controller, 32, 32);

	assert.deepEqual(opened, []);
});

test("two clicks on two pawns are two clicks", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const orc = pawn({ id: "orc", x: 400, y: 400 });
	const { controller, opened } = table([goblin, orc]);

	click(controller, 32, 32);
	click(controller, 400, 400);

	assert.deepEqual(opened, []);
});

// CLICK, QUICK DRAG, CLICK IS THREE THINGS THAT HAPPENED. Without this the
// window opens on the far side of a move the GM made on purpose, which is the
// one moment they are least likely to want a panel over the table.
test("a drag between two clicks breaks the pair", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, opened } = table([goblin]);

	click(controller, 32, 32);

	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.drag(at(200, 200), at(168, 168), NONE);
	controller.tool.release(at(200, 200), at(168, 168), NONE);

	click(controller, 32, 32);

	assert.deepEqual(opened, []);
});

// A PLAYER DOUBLE-CLICKING A MONSTER IS THE CASE THE MARQUEE'S ANCHOR EXISTS
// FOR. A monster is not theirs to drag, so there is no Pressing gesture for the
// pair to be counted on, and the viewer most likely to be asking "what is that"
// is the one it would not work for.
test("a player opens a monster they may not move", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, opened } = table([goblin], { role: "player", user: "01PLAYER" });

	click(controller, 32, 32);
	click(controller, 32, 32);

	assert.deepEqual(opened, ["goblin"]);
	assert.deepEqual(controller.selection.ids(), [], "a player selected a monster they may not move");
});

// SHIFT IS BUILDING A SELECTION. Two shift clicks on one pawn put it into a
// group and take it straight back out, which is something somebody does on
// purpose -- and a window landing on the table halfway through picking a group
// is not.
test("shift clicks never open anything", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, opened } = table([goblin]);

	click(controller, 32, 32, SHIFT);
	click(controller, 32, 32, SHIFT);

	assert.deepEqual(opened, []);
	assert.deepEqual(controller.selection.ids(), [], "the second shift click did not toggle it back out");
});

// THE LABEL FOLLOWS THE HOVER AND LETS GO OF A SELECTION. A pawn somebody has
// picked out is a pawn they are about to drag, turn or resize, and a panel
// parked over the top of it is in the way of all three.
test("the label is about what is hovered and never what is selected", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const orc = pawn({ id: "orc", x: 400, y: 400 });
	const { controller } = table([goblin, orc]);

	controller.tool.hover(at(32, 32));
	assert.equal(controller.focus()?.id, "goblin");

	controller.selection.set(["goblin"]);
	assert.equal(controller.focus()?.id, "goblin", "hovering the selected pawn still labels it");

	// The hand moves off it. One thing is selected and nothing is under the
	// pointer, so there is nothing to label.
	controller.tool.hover(null);
	assert.equal(controller.focus(), null);
	assert.equal(controller.bounds(), null);

	// And hovering something ELSE labels that, not the selection.
	controller.tool.hover(at(400, 400));
	assert.equal(controller.focus()?.id, "orc");
});

// A TOKEN HAS NOTHING TO SAY: no hit points, no armour class, and a name the
// picture already tells you. What a label over one WOULD do is sit on top of
// the handles that appear the moment it is selected.
test("a token is never labelled", () => {
	const wagon = pawn({ id: "wagon", kind: "object", width: 128, height: 256, x: 0, y: 0 });
	const { controller } = table([wagon]);

	controller.tool.hover(at(0, 0));
	assert.equal(controller.focus(), null);
	assert.equal(controller.bounds(), null);

	controller.selection.set(["wagon"]);
	assert.equal(controller.focus(), null, "selecting a token labelled it");

	// And it still has its handles, which is what that space is for.
	assert.equal(controller.handles([]).length, 9);
});

// A GROUP IS THE ONE CASE THE LABEL STAYS UP FOR, because it is not describing
// a pawn at all -- it is the count and the controls that act on the group. Its
// box is the whole selection's, so the panel sits above all of them.
test("a group is boxed by the whole selection", () => {
	const a = pawn({ id: "a", x: 0, y: 0 });
	const b = pawn({ id: "b", x: 400, y: 0 });
	const { controller } = table([a, b]);

	controller.selection.set(["a", "b"]);
	controller.tool.hover(null);

	const box = controller.bounds();
	assert.deepEqual([box?.x1, box?.x2], [-32, 432], "the group's box is not both of them");
});

// DELETE ASKS THE PAGE RATHER THAN SENDING ANYTHING. Removing pawns is
// confirmed, and the confirmation lives on the element that makes the request,
// so what this can do is press it.
test("Delete asks for the selection to be removed and Escape does not", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, removals, sent } = table([goblin]);

	press("Delete");
	assert.equal(removals(), 0, "Delete removed something with nothing selected");

	controller.selection.set(["goblin"]);
	press("Delete");
	assert.equal(removals(), 1);
	assert.deepEqual(sent, [], "Delete sent a command of its own");

	press("Escape");
	assert.equal(removals(), 1, "Escape asked for a removal");
});

// A GM TYPING A GOBLIN'S NEW NAME IS RUBBING OUT A LETTER, NOT A GOBLIN. The
// keys are heard on the document, and the pawn window's form is on it too.
test("Delete inside a field is not Delete on the table", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, removals } = table([goblin]);

	controller.selection.set(["goblin"]);

	press("Delete", { tagName: "INPUT" });
	press("Delete", { tagName: "TEXTAREA" });
	press("Delete", { tagName: "SELECT" });
	press("Delete", { tagName: "DIV", isContentEditable: true });
	assert.equal(removals(), 0);

	press("Delete", { tagName: "DIV" });
	assert.equal(removals(), 1, "a key from the page at large is a key on the table");
});

// THE HANDLES ARE ONE SELECTED OBJECT'S AND NOBODY ELSE'S. A creature has a
// size category rather than a rectangle, and six selected things have six
// centres to scale about.
test("handles are drawn for one selected token and for nothing else", () => {
	const wagon = pawn({ id: "wagon", kind: "object", width: 128, height: 256, x: 0, y: 0 });
	const goblin = pawn({ id: "goblin", x: 400, y: 400 });
	const { controller } = table([wagon, goblin]);

	assert.deepEqual(controller.handles([]), [], "nothing is selected and there are handles");

	controller.selection.set(["goblin"]);
	assert.deepEqual(controller.handles([]), [], "a creature was given resize handles");

	controller.selection.set(["wagon", "goblin"]);
	assert.deepEqual(controller.handles([]), [], "a multiple selection was given handles");

	controller.selection.set(["wagon"]);

	const handles = controller.handles([]);
	assert.equal(handles.length, 9, "eight resize handles and one rotate");
	assert.equal(handles.filter((h) => h.turns).length, 1);

	// The corner handles are the picture's own corners: 64 across, 128 down.
	const corner = handles.find((h) => h.lx === 1 && h.ly === 1 && !h.turns);
	assert.deepEqual([corner?.x, corner?.y], [64, 128]);
});

// SCALING IS ABOUT THE CENTRE, so a resize is a change of size alone and the
// whole gesture is one idempotent command. An edge handle leaves the other axis
// exactly as it was.
test("dragging an edge handle resizes about the centre", () => {
	const wagon = pawn({ id: "wagon", kind: "object", width: 128, height: 256, x: 0, y: 0 });
	const { controller, sent } = table([wagon]);

	controller.selection.set(["wagon"]);

	// The middle of the right edge, dragged out to 100 from the centre.
	const claimed = controller.tool.press(at(64, 0), at(0, 0), NONE);
	assert.equal(claimed, true, "the camera was allowed to pan from a handle");

	controller.tool.drag(at(100, 0), at(36, 0), NONE);

	const ghost = controller.ghosts([])[0];
	assert.deepEqual([ghost?.width, ghost?.height], [200, 256], "the preview did not follow the hand");
	assert.deepEqual([ghost?.x, ghost?.y], [0, 0], "the token moved while it was being resized");

	controller.tool.release(at(100, 0), at(36, 0), NONE);

	assert.deepEqual(sent, [{ type: "pawn.update", id: "wagon", width: 200, height: 256 }]);
});

// THE ROTATE HANDLE MARKS THE BOTTOM EDGE -- above the token is where the pawn
// overlay sits -- so dragging it to the LEFT of the centre is a quarter turn
// clockwise. Shift steps it, which is the only way to get a token exactly
// square again once it has been turned by hand.
test("dragging the rotate handle turns the token about its centre", () => {
	const wagon = pawn({ id: "wagon", kind: "object", width: 128, height: 256, x: 0, y: 0 });
	const { controller, sent } = table([wagon]);

	controller.selection.set(["wagon"]);

	const spinner = controller.handles([]).find((h) => h.turns);
	assert.ok(spinner, "there is no rotate handle");

	assert.ok(spinner.y > 0, "the rotate handle is above the token, under the overlay");

	controller.tool.press(at(spinner.x, spinner.y), at(0, 0), NONE);
	controller.tool.drag(at(-300, -4), at(0, 0), NONE);

	// Just past the quarter turn, and the angle on the wire is whole degrees.
	assert.equal(controller.ghosts([])[0]?.rotation, 91);

	// Shift snaps to fifteen degree steps, which is what makes a right angle
	// reachable by hand.
	controller.tool.drag(at(-300, -4), at(0, 0), SHIFT);
	assert.equal(controller.ghosts([])[0]?.rotation, 90);

	controller.tool.release(at(-300, -4), at(0, 0), SHIFT);

	assert.deepEqual(sent, [{ type: "pawn.update", id: "wagon", rotation: 90 }]);
});

// A HANDLE THAT WAS PRESSED AND NOT DRAGGED SENDS NOTHING, the way a click on a
// pawn does not move it.
test("a handle pressed and released sends nothing", () => {
	const wagon = pawn({ id: "wagon", kind: "object", width: 128, height: 256, x: 0, y: 0 });
	const { controller, sent } = table([wagon]);

	controller.selection.set(["wagon"]);
	controller.tool.press(at(64, 128), at(0, 0), NONE);
	controller.tool.release(at(64, 128), at(0, 0), NONE);

	assert.deepEqual(sent, []);
});

// AND A RESIZE ABANDONED HALFWAY SENDS NOTHING AT ALL, which is where it is
// unlike a cancelled move: the proposal never left this client, so there is
// nothing anybody else has to be told to stop drawing.
test("a resize abandoned with the right button sends nothing", () => {
	const wagon = pawn({ id: "wagon", kind: "object", width: 128, height: 256, x: 0, y: 0 });
	const { controller, sent } = table([wagon]);

	controller.selection.set(["wagon"]);
	controller.tool.press(at(64, 0), at(0, 0), NONE);
	controller.tool.drag(at(300, 0), at(0, 0), NONE);

	controller.tool.secondary(at(300, 0), at(0, 0));
	assert.deepEqual(sent, []);
	assert.deepEqual(controller.ghosts([]), [], "the proposal outlived the gesture");
});

// THE FOG'S HALF OF THE TABLE. What is pinned here is the seam rather than the
// geometry, which fog.test.ts owns: which gesture reaches the fog, what leaves
// the client when one finishes, and the four places a player must not find a
// pawn that is under the cover.

const FOG_ON = { fogging: true, fogEnabled: true, fogPrefill: true } as const;

// A cleared square over the middle of a 64 pixel grid, so a pawn at 32,32 is
// inside it and one at 300,300 is not.
function cleared(): FogShape {
	return {
		id: "01CLEARED", layerId: GROUND, kind: "rect", mode: "reveal",
		points: [0, 0, 128, 128],
	};
}

test("a fog rectangle sends its corners snapped and normalised", () => {
	const { controller, sent } = table([], FOG_ON);

	// Dragged up and to the left, from inside one cell to inside another.
	controller.tool.press(at(200, 200), at(0, 0), NONE);
	controller.tool.drag(at(70, 70), at(0, 0), NONE);
	controller.tool.release(at(70, 70), at(0, 0), NONE);

	assert.deepEqual(sent, [{
		type: "fog.add", layer: GROUND, kind: "rect", mode: "reveal",
		points: [64, 64, 192, 192],
	}]);
});

// A CLICK IS NOT A RECTANGLE, and the test is on the snapped corners rather
// than on how far the hand moved: a drag across half a cell snaps to nothing
// and would send a shape nobody could see.
test("a fog rectangle that snaps to nothing sends nothing", () => {
	const { controller, sent } = table([], FOG_ON);

	controller.tool.press(at(70, 70), at(0, 0), NONE);
	controller.tool.drag(at(80, 80), at(0, 0), NONE);
	controller.tool.release(at(80, 80), at(0, 0), NONE);

	assert.deepEqual(sent, []);
});

// THE RIGHT BUTTON MEANS TWO THINGS INSIDE THIS ONE TOOL, and both are here. A
// rectangle has nothing half-made to commit, so it is abandoned; a polygon does,
// so it closes.
test("the right button abandons a fog rectangle", () => {
	const { controller, sent } = table([], FOG_ON);

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(200, 200), at(0, 0), NONE);
	controller.tool.secondary(at(200, 200), at(0, 0));
	controller.tool.release(at(200, 200), at(0, 0), NONE);

	assert.deepEqual(sent, []);
});

test("the right button closes a fog polygon", () => {
	const { controller, sent } = table([], { ...FOG_ON, shape: "poly" });

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.press(at(200, 0), at(0, 0), NONE);
	controller.tool.press(at(200, 200), at(0, 0), NONE);
	controller.tool.secondary(at(200, 200), at(0, 0));

	assert.deepEqual(sent, [{
		type: "fog.add", layer: GROUND, kind: "poly", mode: "reveal",
		points: [0, 0, 192, 0, 192, 192],
	}]);
});

test("a fog polygon of two corners is not a shape", () => {
	const { controller, sent } = table([], { ...FOG_ON, shape: "poly" });

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.press(at(200, 0), at(0, 0), NONE);
	controller.tool.secondary(at(200, 0), at(0, 0));

	assert.deepEqual(sent, []);
});

test("Escape drops a fog polygon that was half drawn", () => {
	const { controller, sent } = table([], { ...FOG_ON, shape: "poly" });

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.press(at(200, 0), at(0, 0), NONE);
	controller.tool.press(at(200, 200), at(0, 0), NONE);

	press("Escape");
	controller.tool.secondary(at(200, 200), at(0, 0));

	assert.deepEqual(sent, [], "the ring came back after it was abandoned");
});

test("Backspace takes back a fog polygon's last corner", () => {
	const { controller, sent } = table([], { ...FOG_ON, shape: "poly" });

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.press(at(200, 0), at(0, 0), NONE);
	controller.tool.press(at(200, 200), at(0, 0), NONE);
	press("Backspace");
	controller.tool.press(at(0, 200), at(0, 0), NONE);
	controller.tool.secondary(at(0, 200), at(0, 0));

	assert.deepEqual(sent, [{
		type: "fog.add", layer: GROUND, kind: "poly", mode: "reveal",
		points: [0, 0, 192, 0, 0, 192],
	}]);
});

// A CORNER ON TOP OF THE LAST ONE IS NOT A CORNER. Snapping makes this common
// rather than rare: two clicks in the same cell land on the same vertex.
test("two clicks in one cell are one fog corner", () => {
	const { controller, sent } = table([], { ...FOG_ON, shape: "poly" });

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.press(at(10, 10), at(0, 0), NONE);
	controller.tool.press(at(200, 0), at(0, 0), NONE);
	controller.tool.secondary(at(200, 0), at(0, 0));

	assert.deepEqual(sent, [], "a doubled corner made a triangle out of a line");
});

test("Ctrl-Z takes back the newest shape on the floor being viewed", () => {
	const mine = cleared();
	const upstairs = { ...cleared(), id: "01UPSTAIRS", layerId: CELLAR };
	const { sent } = table([], { ...FOG_ON, fog: [mine, upstairs] });

	press("z", null, { ctrlKey: true });

	assert.deepEqual(sent, [{ type: "fog.remove", id: "01CLEARED" }],
		"the undo reached across to another floor");
});

test("the fog takes no gesture while another tool is chosen", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, sent } = table([goblin], { fogEnabled: true, fogPrefill: true });

	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.release(at(32, 32), at(0, 0), NONE);

	assert.deepEqual(sent, []);
	assert.deepEqual(controller.selection.ids(), ["goblin"], "the select tool stopped selecting");
});

// THE FOUR CONCEALMENT GATES. A pawn under the cover is not drawn, not
// labelled, not clickable and not swept up -- and a gate that was forgotten is
// the failure this feature has, because the other three still look right.
test("a player finds nothing under the cover", () => {
	const goblin = pawn({ id: "goblin", x: 400, y: 400 });
	const { controller } = table([goblin], {
		role: "player", user: "01PLAYER", fogEnabled: true, fogPrefill: true,
		fog: [cleared()],
	});

	assert.equal(controller.concealed(goblin), true);

	controller.tool.hover(at(400, 400));
	assert.equal(controller.focus(), null, "a concealed pawn was labelled");

	controller.tool.press(at(400, 400), at(0, 0), NONE);
	controller.tool.release(at(400, 400), at(0, 0), NONE);
	assert.deepEqual(controller.selection.ids(), [], "a concealed pawn was clicked");
});

test("a player's marquee does not sweep up what it cannot see", () => {
	const mine = pawn({ id: "mine", x: 400, y: 400, ownerId: "01PLAYER" });
	const theirs = pawn({ id: "theirs", x: 420, y: 420 });
	const { controller } = table([mine, theirs], {
		role: "player", user: "01PLAYER", fogEnabled: true, fogPrefill: true,
		fog: [cleared()],
	});

	controller.tool.press(at(300, 300), at(0, 0), NONE);
	controller.tool.drag(at(500, 500), at(90, 90), NONE);
	controller.tool.release(at(500, 500), at(90, 90), NONE);

	// A PLAYER'S OWN PAWN IS NEVER CONCEALED FROM THEM. Somebody who walks into
	// an unlit room and watches their own token vanish has been told the app is
	// broken rather than that the room is dark.
	assert.deepEqual(controller.selection.ids(), ["mine"]);
});

test("a pawn on the cleared part of the floor is not concealed", () => {
	const goblin = pawn({ id: "goblin", x: 64, y: 64 });
	const { controller } = table([goblin], {
		role: "player", user: "01PLAYER", fogEnabled: true, fogPrefill: true,
		fog: [cleared()],
	});

	assert.equal(controller.concealed(goblin), false);
});

test("a floor whose fog is off conceals nothing", () => {
	const goblin = pawn({ id: "goblin", x: 400, y: 400 });
	const { controller } = table([goblin], {
		role: "player", user: "01PLAYER", fogEnabled: false, fogPrefill: true,
	});

	assert.equal(controller.concealed(goblin), false);
});

test("the GM is concealed from nothing", () => {
	const goblin = pawn({ id: "goblin", x: 400, y: 400 });
	const { controller } = table([goblin], { fogEnabled: true, fogPrefill: true });

	assert.equal(controller.concealed(goblin), false);
});

// THE RECTANGLE HAS TO BE VISIBLE WHILE IT IS BEING DRAGGED, which the polygon
// got for free -- its rubber band goes through marks() -- and the rectangle did
// not: fog.outline() existed and nothing called it, so the tool worked and drew
// nothing. Everything else about the gesture passed its tests.
test("a fog rectangle is previewed while it is dragged", () => {
	const { controller } = table([], FOG_ON);

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(200, 130), at(0, 0), NONE);

	const [box, ...rest] = controller.outlines([]);
	assert.equal(rest.length, 0, "something else is on the table as well");
	assert.ok(box, "the rectangle in hand is not drawn");

	// The snapped corners are 0,0 and 192,128, so the box is centred between
	// them and half as wide.
	assert.equal(box.rect, true);
	assert.deepEqual([box.x, box.y, box.halfW, box.halfH], [96, 64, 96, 64]);
});

test("a fog rectangle that has not left its first vertex is not drawn", () => {
	const { controller } = table([], FOG_ON);

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(10, 10), at(0, 0), NONE);

	assert.deepEqual(controller.outlines([]), []);
});

test("the fog rectangle goes when the gesture does", () => {
	const { controller } = table([], FOG_ON);

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(200, 130), at(0, 0), NONE);
	controller.tool.secondary(at(200, 130), at(0, 0));

	assert.deepEqual(controller.outlines([]), [], "an abandoned rectangle is still on the table");
});

// The preview says which way the gesture works, and it says it in hue: both
// tones are light, because a dark outline on a dark dungeon cannot be aimed
// with.
test("the fog rectangle is coloured by the mode it will send", () => {
	const { controller, chooseFog } = table([], FOG_ON);

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(200, 130), at(0, 0), NONE);
	const uncover = controller.outlines([])[0].color;

	chooseFog("rect", "hide");
	const cover = controller.outlines([])[0].color;

	assert.notDeepEqual(uncover, cover);
});

// THE PEN. What is under test here is the seam again -- which gesture reaches
// draw.ts and what leaves the client when one finishes -- and not the geometry,
// which draw.test.ts owns.

const PEN_ON = { inking: true } as const;

test("the pen is not the tool unless it is chosen", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, sent } = table([goblin]);

	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.release(at(32, 32), at(0, 0), NONE);

	assert.deepEqual(sent, [], "a press under the Select tool drew something");
	assert.deepEqual(controller.selection.ids(), ["goblin"]);
});

// A LINE BEGINS ON THE PRESS AND NOT ON THE RELEASE, which is the whole
// difference between this tool and the fog: everybody else at the table watches
// it form.
test("a pen stroke begins, extends and ends", () => {
	const { controller, sent } = table([], PEN_ON);

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(100, 0), at(0, 0), NONE);
	controller.tool.drag(at(100, 100), at(0, 0), NONE);
	controller.tool.release(at(100, 100), at(0, 0), NONE);

	assert.deepEqual(sent.map((c) => c.type), ["stroke.begin", "stroke.extend", "stroke.end"]);

	const begin = sent[0];
	assert.equal(begin.layer, GROUND);
	assert.equal(begin.kind, "free");
	assert.equal(begin.color, "#FF0000");
	assert.equal(begin.width, 4);
	assert.deepEqual(begin.points, [0, 0]);

	assert.deepEqual(sent[1].points, [100, 0, 100, 100]);

	// One id across all three, minted by the client before anything answered.
	assert.equal(typeof begin.id, "string");
	assert.equal(sent[1].id, begin.id);
	assert.equal(sent[2].id, begin.id);
});

// A tap is a dot: one point, no run of them, and no extend to carry one.
test("a pen click is a stroke of one point", () => {
	const { controller, sent } = table([], PEN_ON);

	controller.tool.press(at(48, 48), at(0, 0), NONE);
	controller.tool.release(at(48, 48), at(0, 0), NONE);

	assert.deepEqual(sent.map((c) => c.type), ["stroke.begin", "stroke.end"]);
	assert.deepEqual(sent[0].points, [48, 48]);
});

// DECIMATION IS MEASURED IN MAP PIXELS AT THE CURRENT ZOOM. A hand that has not
// moved a screen pixel has not drawn anything, however many times the pointer
// reported itself.
test("the pen drops points the hand did not really move", () => {
	const { controller, sent } = table([], { ...PEN_ON, scale: 20 });

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	for (let i = 1; i <= 8; i++) {
		controller.tool.drag(at(i, 0), at(0, 0), NONE);
	}
	controller.tool.release(at(8, 0), at(0, 0), NONE);

	// Every one of those eight was inside twenty map pixels of the start, so
	// the only point that survives is the one the hand lifted at.
	assert.deepEqual(sent.map((c) => c.type), ["stroke.begin", "stroke.extend", "stroke.end"]);
	assert.deepEqual(sent[1].points, [8, 0]);
});

// THE LAST POINT IS KEPT WHATEVER THE THRESHOLD SAYS, so a line ends where the
// hand lifted rather than at the last sample far enough from the one before it.
test("the pen keeps the point it was lifted at", () => {
	const { controller, sent } = table([], { ...PEN_ON, scale: 50 });

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(10, 0), at(0, 0), NONE);
	controller.tool.release(at(10, 0), at(0, 0), NONE);

	assert.deepEqual(sent[1].points, [10, 0]);
});

// ABANDONING A LINE IS ENDING IT AND THEN RUBBING IT OUT. The begin already
// went out on the press, so there is nothing to un-begin -- and ending it first
// is what makes the erase legal for a player, whose own stroke this is.
test("Escape mid-stroke ends the line and rubs it out", () => {
	const { controller, sent } = table([], PEN_ON);

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(100, 100), at(0, 0), NONE);
	press("Escape");

	assert.deepEqual(sent.map((c) => c.type), [
		"stroke.begin", "stroke.extend", "stroke.end", "stroke.erase",
	]);
	assert.deepEqual(sent[3].ids, [sent[0].id]);
});

test("the right button mid-stroke rubs the line out too", () => {
	const { controller, sent } = table([], PEN_ON);

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(100, 100), at(0, 0), NONE);
	controller.tool.secondary(at(100, 100), at(0, 0));

	assert.deepEqual(sent.map((c) => c.type).at(-1), "stroke.erase");
	assert.deepEqual(sent.at(-1)?.ids, [sent[0].id]);
});

// A pointer the browser took away mid-stroke is the same case, and it must not
// leave a line on everybody's table that nobody meant to draw.
test("a cancelled pointer rubs the line out", () => {
	const { controller, sent } = table([], PEN_ON);

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(100, 100), at(0, 0), NONE);
	controller.tool.cancel();

	assert.deepEqual(sent.map((c) => c.type).at(-1), "stroke.erase");
});

// THE INK UNDER THE PEN IS LOCAL AND THE ECHO IS IGNORED. inHand is what the
// renderer draws while a line is being made; the store's copy takes over the
// moment it is finished.
test("the stroke in hand is local until it is finished", () => {
	const { controller } = table([], PEN_ON);

	assert.equal(controller.inHand(), null);

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(100, 0), at(0, 0), NONE);

	const line = controller.inHand();
	assert.equal(line?.kind, "free");
	assert.equal(line?.layerId, GROUND);
	assert.deepEqual(line?.points, [0, 0, 100, 0]);

	controller.tool.release(at(100, 0), at(0, 0), NONE);
	assert.equal(controller.inHand(), null);
});

// A gesture that began under the pen finishes under it, which is the rule every
// other mode on this table follows.
test("a stroke survives the tool being switched away mid-gesture", () => {
	const { controller, sent, drawTool } = table([], PEN_ON);

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	drawTool(false);
	controller.tool.drag(at(100, 0), at(0, 0), NONE);
	controller.tool.release(at(100, 0), at(0, 0), NONE);

	assert.deepEqual(sent.map((c) => c.type), ["stroke.begin", "stroke.extend", "stroke.end"]);
});

// THE ERASER AND CTRL+Z. What is under test is the seam again: which lines a
// sweep picks up, which it must not, and what leaves the client. The distance
// test itself is draw.test.ts's.

const ERASE_ON = { inking: true, drawMode: "erase" } as const;

function drawn(over: Partial<Stroke> = {}): Stroke {
	return {
		id: "01LINE", by: GM, layerId: GROUND, kind: "free",
		color: "#FF0000", width: 4, points: [0, 0, 100, 0], done: true, ...over,
	};
}

// ONE COMMAND PER SWEEP AND NOT ONE PER LINE. A drag across a sketch crosses a
// dozen strokes, and a dozen erases would be a dozen broadcasts and a dozen
// re-renders on every screen at the table.
test("one sweep of the eraser sends one command with everything it crossed", () => {
	const a = drawn({ id: "01A", points: [0, 0, 0, 100] });
	const b = drawn({ id: "01B", points: [50, 0, 50, 100] });
	const away = drawn({ id: "01C", points: [900, 900, 900, 950] });

	const { controller, sent } = table([], { ...ERASE_ON, strokes: [a, b, away] });

	controller.tool.press(at(0, 50), at(0, 0), NONE);
	controller.tool.drag(at(50, 50), at(0, 0), NONE);
	controller.tool.release(at(50, 50), at(0, 0), NONE);

	assert.deepEqual(sent, [{ type: "stroke.erase", ids: ["01A", "01B"] }]);
});

// A SWEEP THAT TOUCHED NOTHING SAYS NOTHING, rather than sending an empty list
// for the server to answer with nothing.
test("an eraser sweep over empty floor sends nothing", () => {
	const { controller, sent } = table([], { ...ERASE_ON, strokes: [drawn()] });

	controller.tool.press(at(0, 400), at(0, 0), NONE);
	controller.tool.drag(at(100, 400), at(0, 0), NONE);
	controller.tool.release(at(100, 400), at(0, 0), NONE);

	assert.deepEqual(sent, []);
});

// A PLAYER'S ERASER PASSES OVER SOMEBODY ELSE'S LINE. The rule is the server's
// -- StrokeErase refuses it -- and this is the half that makes the tool feel
// like a tool rather than like a control that raises an alert modal.
test("a player's eraser only takes their own lines", () => {
	const mine = drawn({ id: "01MINE", by: PLAYER });
	const theirs = drawn({ id: "01THEIRS", by: GM });

	const { controller, sent } = table([], {
		...ERASE_ON, role: "player", user: PLAYER, strokes: [mine, theirs],
	});

	controller.tool.press(at(50, 0), at(0, 0), NONE);
	controller.tool.release(at(50, 0), at(0, 0), NONE);

	assert.deepEqual(sent, [{ type: "stroke.erase", ids: ["01MINE"] }]);
});

test("the GM's eraser takes anybody's line", () => {
	const mine = drawn({ id: "01MINE", by: GM });
	const theirs = drawn({ id: "01THEIRS", by: PLAYER });

	const { controller, sent } = table([], { ...ERASE_ON, strokes: [mine, theirs] });

	controller.tool.press(at(50, 0), at(0, 0), NONE);
	controller.tool.release(at(50, 0), at(0, 0), NONE);

	assert.deepEqual(sent, [{ type: "stroke.erase", ids: ["01MINE", "01THEIRS"] }]);
});

// AN UNFINISHED LINE IS NOBODY'S TO ERASE, its author's included: somebody is
// still drawing it, and taking it out from under their hand would leave their
// next chunk refused with "Stroke gone".
test("the eraser passes over a line still being drawn", () => {
	const growing = drawn({ id: "01GROWING", done: false });

	const { controller, sent } = table([], { ...ERASE_ON, strokes: [growing] });

	controller.tool.press(at(50, 0), at(0, 0), NONE);
	controller.tool.release(at(50, 0), at(0, 0), NONE);

	assert.deepEqual(sent, []);
});

test("the eraser ignores a line on another floor", () => {
	const upstairs = drawn({ id: "01UP", layerId: CELLAR });

	const { controller, sent } = table([], { ...ERASE_ON, strokes: [upstairs] });

	controller.tool.press(at(50, 0), at(0, 0), NONE);
	controller.tool.release(at(50, 0), at(0, 0), NONE);

	assert.deepEqual(sent, []);
});

// A SWEEP DROPPED PART-WAY SENDS NOTHING, which is the opposite of what
// abandoning a pen stroke does and right for the same reason: nothing has left
// this browser yet, so there is nothing on anybody else's table to take back.
test("Escape mid-sweep rubs nothing out", () => {
	const { controller, sent } = table([], { ...ERASE_ON, strokes: [drawn()] });

	controller.tool.press(at(50, 0), at(0, 0), NONE);
	press("Escape");
	controller.tool.release(at(50, 0), at(0, 0), NONE);

	assert.deepEqual(sent, []);
});

// THE ERASER SHOWS ITS REACH, because a radius of a few screen pixels is
// otherwise a tool aimed by guessing.
test("the eraser draws a ring under the pointer", () => {
	const { controller, chooseDraw } = table([], { ...ERASE_ON, scale: 2 });

	controller.tool.hover(at(80, 90));

	const ring = controller.outlines([]).at(-1);
	assert.equal(ring?.x, 80);
	assert.equal(ring?.y, 90);
	assert.equal(ring?.halfW, 12, "the reach is six CSS pixels at this zoom");
	assert.equal(ring?.rect, false);

	// The pen has no cursor: what it is about to do is draw where the pointer
	// already is.
	chooseDraw("pen");
	assert.deepEqual(controller.outlines([]), []);
});

test("the eraser's ring goes when the pointer leaves the table", () => {
	const { controller } = table([], ERASE_ON);

	controller.tool.hover(at(80, 90));
	controller.tool.hover(null);

	assert.deepEqual(controller.outlines([]), []);
});

// CTRL+Z IS THE VIEWER'S OWN NEWEST LINE, and "newest" is the last one in the
// store because ids are minted in order and the store is sorted by them.
test("Ctrl+Z takes back the viewer's newest finished line", () => {
	const first = drawn({ id: "01AAA", by: GM });
	const second = drawn({ id: "01BBB", by: GM });

	const { controller, sent } = table([], { inking: true, strokes: [first, second] });
	void controller;

	press("z", null, { ctrlKey: true });

	assert.deepEqual(sent, [{ type: "stroke.erase", ids: ["01BBB"] }]);
});

// EVEN FOR THE GM. Ctrl+Z means "take back what I just did", and a GM whose
// undo removed the line a player drew a second ago would have a key that
// reaches across the table.
test("Ctrl+Z skips somebody else's line", () => {
	const mine = drawn({ id: "01AAA", by: GM });
	const theirs = drawn({ id: "01ZZZ", by: PLAYER });

	const { controller, sent } = table([], { inking: true, strokes: [mine, theirs] });
	void controller;

	press("z", null, { ctrlKey: true });

	assert.deepEqual(sent, [{ type: "stroke.erase", ids: ["01AAA"] }]);
});

test("Ctrl+Z skips a line on another floor and one still being drawn", () => {
	const upstairs = drawn({ id: "01AAA", by: GM, layerId: CELLAR });
	const growing = drawn({ id: "01BBB", by: GM, done: false });

	const { controller, sent } = table([], { inking: true, strokes: [upstairs, growing] });
	void controller;

	press("z", null, { ctrlKey: true });

	assert.deepEqual(sent, []);
});

// AND IT BELONGS TO THIS TOOL WHILE IT IS CHOSEN AND TO NOTHING AT ALL
// OTHERWISE, which is the rule the fog's three keys follow.
test("Ctrl+Z under another tool is not the drawing's", () => {
	const { controller, sent } = table([], { inking: false, strokes: [drawn({ by: GM })] });
	void controller;

	press("z", null, { ctrlKey: true });

	assert.deepEqual(sent, []);
});

// THE PILL'S COLOUR AND WIDTH REACH THE WIRE, which is the whole of what the
// two folded panels are for. What they look like and how they fold is
// draw-tool.ts's, and it is DOM-bound the way mountTools is -- so what is
// pinned here is the seam: the options a pill hands over are the options a line
// is drawn with.
test("a stroke goes out in the colour and width the pill was left on", () => {
	const { controller, sent, chooseBrush } = table([], PEN_ON);

	chooseBrush("#00FF88", 21);

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.release(at(0, 0), at(0, 0), NONE);

	assert.equal(sent[0].color, "#00FF88");
	assert.equal(sent[0].width, 21);
});

// THE OPTIONS ARE READ WHEN A LINE STARTS AND NOT WHEN IT ENDS, which is the
// opposite of the fog's rule and right for the opposite reason: a fog shape is
// one message sent at the end, and a stroke's colour is already on everybody's
// screen by the time the hand lifts. Changing the pill mid-stroke must not
// recolour the line being drawn.
test("changing the pill mid-stroke does not recolour the line", () => {
	const { controller, sent, chooseBrush } = table([], PEN_ON);

	chooseBrush("#FF0000", 4);
	controller.tool.press(at(0, 0), at(0, 0), NONE);

	chooseBrush("#0000FF", 40);
	controller.tool.drag(at(100, 0), at(0, 0), NONE);
	controller.tool.release(at(100, 0), at(0, 0), NONE);

	assert.equal(sent[0].color, "#FF0000");
	assert.equal(sent[0].width, 4);
});

// AND THE WIDTH IS WHAT THE ERASER AIMS WITH TOO: a sixty-four pixel brush is
// hit where it is drawn rather than down its centre line, so a fat line is
// easier to rub out than a hairline. The distance test is draw.test.ts's; this
// is that it is asked with the stroke's own width.
test("a fat line is easier for the eraser to catch than a thin one", () => {
	const fat = drawn({ id: "01FAT", width: 64, points: [0, 0, 100, 0] });
	const thin = drawn({ id: "01THIN", width: 2, points: [0, 200, 100, 200] });

	const { controller, sent } = table([], { ...ERASE_ON, strokes: [fat, thin] });

	// Twenty pixels off each line: inside the fat one's ink, well outside the
	// thin one's.
	controller.tool.press(at(50, 20), at(0, 0), NONE);
	controller.tool.drag(at(50, 220), at(0, 0), NONE);
	controller.tool.release(at(50, 220), at(0, 0), NONE);

	assert.deepEqual(sent, [{ type: "stroke.erase", ids: ["01FAT"] }]);
});

// THE SHAPES. What is under test is the seam: which gesture reaches draw.ts,
// what leaves the client when one lands, and what the preview looks like while
// it is in hand. The geometry and the labels are draw.test.ts's.

const RECT_ON = { inking: true, drawMode: "rect" } as const;
const CIRCLE_ON = { inking: true, drawMode: "circle" } as const;

// ONE COMMAND AND NOT TWO. A shape arrives Done -- StrokeBegin marks every kind
// but free finished the moment it exists -- so there is no stroke.end to send,
// and no stroke.extend either: a shape never grows.
test("a rectangle lands in one command with its corners normalised", () => {
	const { controller, sent } = table([], RECT_ON);

	// Dragged up and to the left.
	controller.tool.press(at(300, 260), at(0, 0), NONE);
	controller.tool.drag(at(100, 60), at(0, 0), NONE);
	controller.tool.release(at(100, 60), at(0, 0), NONE);

	assert.deepEqual(sent, [{
		type: "stroke.begin", id: sent[0].id, layer: GROUND, kind: "rect",
		color: "#FF0000", width: 4, points: [100, 60, 300, 260],
	}]);
});

// A CIRCLE IS NOT NORMALISED. Its first point is the CENTRE and its second is
// on the rim; swapping them would turn the shape inside out.
test("a circle lands as its centre then a point on its rim", () => {
	const { controller, sent } = table([], CIRCLE_ON);

	controller.tool.press(at(200, 200), at(0, 0), NONE);
	controller.tool.drag(at(120, 140), at(0, 0), NONE);
	controller.tool.release(at(120, 140), at(0, 0), NONE);

	assert.deepEqual(sent.map((c) => c.type), ["stroke.begin"]);
	assert.equal(sent[0].kind, "circle");
	assert.deepEqual(sent[0].points, [200, 200, 120, 140]);
});

// A CLICK IS NOT A SHAPE. The test is on the ROUNDED coordinates, because those
// are what go on the wire: a drag of half a pixel rounds to the same place and
// would send something nobody could see or point at to rub out.
test("a shape with no size sends nothing", () => {
	for (const over of [RECT_ON, CIRCLE_ON]) {
		const { controller, sent } = table([], over);

		controller.tool.press(at(50, 50), at(0, 0), NONE);
		controller.tool.release(at(50.4, 50.4), at(0, 0), NONE);

		assert.deepEqual(sent, [], over.drawMode);
	}
});

// A rectangle needs BOTH axes: one of them alone is a line, which the server
// would take and nobody meant to draw.
test("a rectangle with only one axis sends nothing", () => {
	const { controller, sent } = table([], RECT_ON);

	controller.tool.press(at(0, 100), at(0, 0), NONE);
	controller.tool.drag(at(300, 100), at(0, 0), NONE);
	controller.tool.release(at(300, 100), at(0, 0), NONE);

	assert.deepEqual(sent, []);
});

// ABANDONING A SHAPE SENDS NOTHING, which is the opposite of abandoning a pen
// stroke and right for the opposite reason: nothing has left this browser, so
// there is nothing on anybody else's table to take back.
test("Escape and the right button both drop a shape silently", () => {
	for (const over of [RECT_ON, CIRCLE_ON]) {
		const escaped = table([], over);
		escaped.controller.tool.press(at(0, 0), at(0, 0), NONE);
		escaped.controller.tool.drag(at(200, 200), at(0, 0), NONE);
		press("Escape");
		escaped.controller.tool.release(at(200, 200), at(0, 0), NONE);
		assert.deepEqual(escaped.sent, [], `Escape mid-${over.drawMode}`);

		const clicked = table([], over);
		clicked.controller.tool.press(at(0, 0), at(0, 0), NONE);
		clicked.controller.tool.drag(at(200, 200), at(0, 0), NONE);
		clicked.controller.tool.secondary(at(200, 200), at(0, 0));
		clicked.controller.tool.release(at(200, 200), at(0, 0), NONE);
		assert.deepEqual(clicked.sent, [], `right button mid-${over.drawMode}`);
	}
});

// THE PREVIEW IS WHAT MAKES A SHAPE AIMABLE, and it is the ring pass's own two
// shapes: a box for the rectangle and an ellipse for the circle.
test("a rectangle previews as a box between its corners", () => {
	const { controller } = table([], RECT_ON);

	controller.tool.press(at(100, 60), at(0, 0), NONE);
	controller.tool.drag(at(300, 260), at(0, 0), NONE);

	const box = controller.outlines([])[0];
	assert.equal(box?.rect, true);
	assert.deepEqual([box?.x, box?.y], [200, 160], "not centred between the corners");
	assert.deepEqual([box?.halfW, box?.halfH], [100, 100]);
});

// A CIRCLE GROWS AROUND THE PRESS. That is the gesture, and it is why the
// preview is anchored at the first point rather than between the two.
test("a circle previews as a ring around where it was pressed", () => {
	const { controller } = table([], CIRCLE_ON);

	controller.tool.press(at(200, 200), at(0, 0), NONE);
	controller.tool.drag(at(200 + 60, 200 + 80), at(0, 0), NONE);

	const ring = controller.outlines([])[0];
	assert.equal(ring?.rect, false);
	assert.deepEqual([ring?.x, ring?.y], [200, 200], "the ring moved off the press");
	assert.equal(ring?.halfW, 100, "a 3-4-5 rim is not a radius of 100");
	assert.equal(ring?.halfW, ring?.halfH, "the circle is an ellipse");
});

test("a shape that has not left its first point previews nothing", () => {
	for (const over of [RECT_ON, CIRCLE_ON]) {
		const { controller } = table([], over);

		controller.tool.press(at(50, 50), at(0, 0), NONE);
		controller.tool.drag(at(50, 50), at(0, 0), NONE);

		assert.deepEqual(controller.outlines([]), [], over.drawMode);
	}
});

test("the preview is the colour the shape will land in", () => {
	const { controller, chooseBrush } = table([], RECT_ON);

	chooseBrush("#00FF00", 4);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(200, 200), at(0, 0), NONE);

	assert.deepEqual(controller.outlines([])[0].color, [0, 1, 0]);
});

test("an abandoned shape takes its preview off the table", () => {
	const { controller } = table([], RECT_ON);

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(200, 200), at(0, 0), NONE);
	controller.tool.secondary(at(200, 200), at(0, 0));

	assert.deepEqual(controller.outlines([]), []);
});

// THE NUMBER IS ON THE TABLE WHILE THE SHAPE IS IN HAND, which is the whole
// point of it: a GM drags until it reads thirty feet and lets go.
test("a shape being dragged carries its distance", () => {
	const { controller } = table([], CIRCLE_ON);

	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(256, 0), at(0, 0), NONE);

	assert.deepEqual(controller.labels([]).map((l) => l.text), ["20 ft."]);
});

// AND IT STAYS ON IT AFTERWARDS. A circle sitting on the table for ten rounds
// is what the party is standing in, and the number is what says how big it is.
test("shapes on the floor carry their distance and the pen does not", () => {
	const strokes = [
		drawn({ id: "01CIRCLE", kind: "circle", points: [0, 0, 256, 0] }),
		drawn({ id: "01RECT", kind: "rect", points: [0, 0, 384, 128] }),
		drawn({ id: "01PEN", kind: "free", points: [0, 0, 50, 50, 90, 20] }),
		drawn({ id: "01ELSEWHERE", kind: "circle", layerId: CELLAR, points: [0, 0, 256, 0] }),
	];

	const { controller } = table([], { inking: true, strokes });

	assert.deepEqual(controller.labels([]).map((l) => l.text), ["20 ft.", "30 ft.", "10 ft."]);
});
