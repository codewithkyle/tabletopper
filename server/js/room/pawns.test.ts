import assert from "node:assert/strict";
import { test } from "node:test";
import type { FogMode, FogShape, Grid, Pawn, ShapeKind, State, Stroke } from "./protocol.ts";
import type { DrawMode } from "./draw.ts";
import { empty } from "./store.ts";
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
function press(key: string, target: unknown = null, mods: Record<string, boolean> = {}): void {
	for (const fn of keydown) {
		(fn as (e: unknown) => void)({ key, target, ...mods });
	}
}
const NONE = { shift: false, alt: false };
const SHIFT = { shift: true, alt: false };
const ALT = { shift: false, alt: true };
const at = (x: number, y: number) => ({ x, y });
test("hit testing uses a disc for a creature", () => {
	const pawns = [pawn({ id: "goblin", x: 100, y: 100 })];
	assert.equal(hitTest(pawns, GROUND, grid(), 100, 100)?.id, "goblin");
	assert.equal(hitTest(pawns, GROUND, grid(), 125, 100)?.id, "goblin");
	assert.equal(hitTest(pawns, GROUND, grid(), 131, 131), null);
});
test("hit testing uses a rectangle for an object", () => {
	const wagon = pawn({ id: "wagon", kind: "object", width: 128, height: 256, x: 0, y: 0 });
	assert.equal(hitTest([wagon], GROUND, grid(), 60, 120)?.id, "wagon");
	assert.equal(hitTest([wagon], GROUND, grid(), 63, 127)?.id, "wagon", "a corner of the wagon is the wagon");
	assert.equal(hitTest([wagon], GROUND, grid(), 70, 0), null);
});
test("hit testing prefers the topmost pawn", () => {
	const pawns = [
		pawn({ id: "under", z: 1 }),
		pawn({ id: "over", z: 5 }),
		pawn({ id: "middle", z: 3 }),
	];
	assert.equal(hitTest(pawns, GROUND, grid(), 0, 0)?.id, "over");
});
test("a creature is picked over a token it is standing on", () => {
	const rug = pawn({ id: "rug", kind: "object", width: 256, height: 256, x: 0, y: 0, z: 99 });
	const goblin = pawn({ id: "goblin", x: 0, y: 0, z: 1 });
	assert.equal(hitTest([rug, goblin], GROUND, grid(), 0, 0)?.id, "goblin");
	assert.equal(hitTest([rug, goblin], GROUND, grid(), 100, 100)?.id, "rug");
});
test("hit testing turns with the token", () => {
	const flat = pawn({ id: "beam", kind: "object", width: 256, height: 32, x: 0, y: 0 });
	const upright = pawn({ ...flat, rotation: 90 });
	assert.equal(hitTest([flat], GROUND, grid(), 120, 0)?.id, "beam");
	assert.equal(hitTest([flat], GROUND, grid(), 0, 120), null);
	assert.equal(hitTest([upright], GROUND, grid(), 120, 0), null);
	assert.equal(hitTest([upright], GROUND, grid(), 0, 120)?.id, "beam");
});
test("hit testing ignores another floor", () => {
	const pawns = [pawn({ id: "upstairs", layerId: CELLAR })];
	assert.equal(hitTest(pawns, GROUND, grid(), 0, 0), null);
});
function table(
	pawns: Pawn[],
	over: Partial<{
		role: "gm" | "player"; user: string; grid: Grid;
		scale: number; panning: boolean; measuring: boolean;
		fogging: boolean; shape: ShapeKind; mode: FogMode;
		fogEnabled: boolean; fogPrefill: boolean; fog: FogShape[];
		inking: boolean; drawMode: DrawMode; strokes: Stroke[];
		pinging: boolean;
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
	let panning = over.panning ?? false;
	let measuring = over.measuring ?? false;
	let fogging = over.fogging ?? false;
	let inking = over.inking ?? false;
	let pinging = over.pinging ?? false;
	const drawOptions = { mode: over.drawMode ?? "pen", color: "#FF0000", width: 4 };
	const options = { shape: over.shape ?? "rect", mode: over.mode ?? "reveal" };
	const send = (command: unknown) => {
		sent.push(command as Record<string, unknown>);
	};
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
		pinging: () => pinging,
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
		pingTool: (on: boolean) => {
			pinging = on;
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
test("a click on empty table clears the selection", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller } = table([goblin]);
	controller.selection.set(["goblin"]);
	const claimed = controller.tool.press(at(900, 900), at(0, 0), NONE);
	controller.tool.release(at(900, 900), at(0, 0), NONE);
	assert.equal(claimed, true, "the camera took a press the marquee needs");
	assert.deepEqual(controller.selection.ids(), []);
});
test("a drag from empty table marquees", () => {
	const a = pawn({ id: "a", x: 100, y: 100 });
	const b = pawn({ id: "b", x: 900, y: 900 });
	const { controller } = table([a, b]);
	assert.equal(controller.tool.press(at(0, 0), at(0, 0), NONE), true);
	controller.tool.drag(at(200, 200), at(200, 200), NONE);
	controller.tool.release(at(200, 200), at(200, 200), NONE);
	assert.deepEqual(controller.selection.ids(), ["a"]);
});
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
test("a marquee that found nothing clears the selection", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller } = table([goblin]);
	controller.selection.set(["goblin"]);
	controller.tool.press(at(900, 900), at(0, 0), NONE);
	controller.tool.drag(at(1200, 1200), at(300, 300), NONE);
	controller.tool.release(at(1200, 1200), at(300, 300), NONE);
	assert.deepEqual(controller.selection.ids(), []);
});
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
test("the move tool keeps the selection", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller } = table([goblin], { panning: true });
	controller.selection.set(["goblin"]);
	controller.tool.press(at(900, 900), at(0, 0), NONE);
	controller.tool.release(at(900, 900), at(0, 0), NONE);
	assert.deepEqual(controller.selection.ids(), ["goblin"]);
});
test("a drag survives the tool changing under it", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, sent, pan } = table([goblin]);
	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.drag(at(90, 90), at(58, 58), NONE);
	pan(true);
	controller.tool.release(at(90, 90), at(58, 58), NONE);
	assert.equal(sent[sent.length - 1]?.type, "pawn.move", "the drag was abandoned mid-flight");
});
test("the measure tool puts a point down and runs a line to the pointer", () => {
	const { controller } = table([], { measuring: true });
	assert.equal(controller.tool.press(at(100, 100), at(0, 0), NONE), true, "the ruler gave the press away");
	controller.tool.release(at(100, 100), at(0, 0), NONE);
	controller.tool.hover(at(292, 100));
	const [ruler, ...rest] = controller.rulers([]);
	assert.deepEqual(rest, [], "one measurement drew more than one ruler");
	assert.deepEqual([ruler?.x0, ruler?.y0], [100, 100]);
	assert.deepEqual([ruler?.x1, ruler?.y1], [292, 100]);
	assert.equal(ruler?.label, "15 ft.");
	const [point, ...others] = controller.outlines([]);
	assert.deepEqual(others, [], "the ruler drew more than the one point");
	assert.deepEqual([point?.x, point?.y], [100, 100]);
	assert.equal(point?.rect, false, "the point is not a ring");
});
test("a measurement is a straight line and not a square count", () => {
	const { controller } = table([], { measuring: true });
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.hover(at(192, 192));
	const [ruler] = controller.rulers([]);
	assert.equal(ruler?.label, "21 ft.");
	assert.deepEqual(ruler?.cells, [], "the free ruler tinted cells");
});
test("a measurement snaps to nothing at either end", () => {
	const { controller } = table([], { measuring: true });
	controller.tool.press(at(37, 91), at(0, 0), NONE);
	controller.tool.hover(at(52, 103));
	const [ruler] = controller.rulers([]);
	assert.deepEqual([ruler?.x0, ruler?.y0, ruler?.x1, ruler?.y1], [37, 91, 52, 103]);
});
test("a second press ends the measurement", () => {
	const { controller } = table([], { measuring: true });
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.hover(at(64, 0));
	assert.equal(controller.tool.press(at(320, 0), at(0, 0), NONE), true, "the second press was given away");
	assert.deepEqual(controller.rulers([]), [], "the second press left the ruler up");
	assert.deepEqual(controller.outlines([]), [], "the second press left the point down");
	controller.tool.hover(at(640, 0));
	assert.deepEqual(controller.rulers([]), [], "the ruler came back on its own");
});
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
test("Escape puts the ruler away", () => {
	const { controller } = table([], { measuring: true });
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.hover(at(192, 0));
	press("Escape");
	assert.deepEqual(controller.rulers([]), [], "Escape left the ruler up");
	assert.deepEqual(controller.outlines([]), [], "Escape left the point down");
});
test("leaving the measure tool forgets the measurement", () => {
	const { controller, measure } = table([], { measuring: true });
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.hover(at(192, 0));
	measure(false);
	assert.deepEqual(controller.rulers([]), [], "another tool kept the ruler up");
	measure(true);
	assert.deepEqual(controller.rulers([]), [], "the old ruler came back");
});
test("panning with the space bar does not put the ruler away", () => {
	const { controller, pan } = table([], { measuring: true });
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.hover(at(192, 0));
	pan(true);
	assert.equal(controller.tool.press(at(400, 400), at(0, 0), NONE), false, "the hold did not reach the camera");
	const [ruler] = controller.rulers([]);
	assert.deepEqual([ruler?.x0, ruler?.x1], [0, 192], "the pan moved or dropped the ruler");
});
test("the others move by the anchor's snapped delta and nothing else", () => {
	const anchor = pawn({ id: "anchor", x: 32, y: 32, z: 1 });
	const rider = pawn({ id: "rider", x: 100, y: 200, z: 2 });
	const { controller } = table([anchor, rider]);
	controller.selection.set(["anchor", "rider"]);
	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.drag(at(90, 90), at(58, 58), NONE);
	const ghosts = controller.ghosts([]);
	const byID = new Map(ghosts.map((g) => [g.id, g]));
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
	assert.deepEqual(controller.ghosts([]), []);
});
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
	assert.deepEqual([move?.x, move?.y], [64, 0]);
});
test("a press on a rider grabs the rider and not the wagon", () => {
	const { controller, sent } = table(wagonAndRider());
	controller.tool.press(at(10, 10), at(0, 0), NONE);
	controller.tool.drag(at(74, 10), at(64, 0), NONE);
	controller.tool.release(at(74, 10), at(64, 0), NONE);
	assert.equal(sent[sent.length - 1]?.anchor, "rider");
});
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
test("a shift click on empty table keeps the selection", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller } = table([goblin]);
	controller.selection.set(["goblin"]);
	controller.tool.press(at(900, 900), at(0, 0), SHIFT);
	controller.tool.release(at(900, 900), at(0, 0), SHIFT);
	assert.deepEqual(controller.selection.ids(), ["goblin"]);
});
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
test("a player pressing somebody else's pawn selects nothing", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32, ownerId: null });
	const { controller, sent } = table([goblin], { role: "player", user: "01ME" });
	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.drag(at(200, 200), at(168, 168), NONE);
	controller.tool.release(at(200, 200), at(168, 168), NONE);
	assert.deepEqual(sent, [], "a player moved a pawn that is not theirs");
	assert.deepEqual(controller.selection.ids(), []);
});
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
test("a client ignores its own dragging event", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller } = table([goblin]);
	controller.preview({
		type: "pawn.dragging", seq: 4, by: GM,
		pawns: [{ id: "goblin", x: 160, y: 160 }],
	});
	assert.deepEqual(controller.ghosts([]), []);
});
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
test("the right button on a pawn asks for its menu and opens nothing", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, menus, opened } = table([goblin]);
	controller.tool.secondary(at(32, 32), at(0, 0));
	assert.deepEqual(menus, ["goblin"]);
	assert.deepEqual(opened, [], "the right button opened the window behind the menu");
	controller.tool.secondary(at(900, 900), at(0, 0));
	assert.deepEqual(menus, ["goblin"]);
});
test("the right button follows the draw order", () => {
	const rug = pawn({ id: "rug", kind: "object", width: 256, height: 256, x: 0, y: 0, z: 9 });
	const goblin = pawn({ id: "goblin", x: 0, y: 0, z: 1 });
	const { controller, menus } = table([rug, goblin]);
	controller.tool.secondary(at(0, 0), at(0, 0));
	assert.deepEqual(menus, ["goblin"]);
});
test("the right button leaves the selection alone", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const wagon = pawn({ id: "wagon", kind: "object", width: 64, height: 64, x: 400, y: 400 });
	const { controller } = table([goblin, wagon]);
	controller.selection.set(["wagon"]);
	controller.tool.secondary(at(32, 32), at(0, 0));
	assert.deepEqual(controller.selection.ids(), ["wagon"]);
});
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
	assert.deepEqual(controller.selection.ids(), ["goblin"]);
});
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
test("a player opens a monster they may not move", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, opened } = table([goblin], { role: "player", user: "01PLAYER" });
	click(controller, 32, 32);
	click(controller, 32, 32);
	assert.deepEqual(opened, ["goblin"]);
	assert.deepEqual(controller.selection.ids(), [], "a player selected a monster they may not move");
});
test("shift clicks never open anything", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, opened } = table([goblin]);
	click(controller, 32, 32, SHIFT);
	click(controller, 32, 32, SHIFT);
	assert.deepEqual(opened, []);
	assert.deepEqual(controller.selection.ids(), [], "the second shift click did not toggle it back out");
});
test("the label is about what is hovered and never what is selected", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const orc = pawn({ id: "orc", x: 400, y: 400 });
	const { controller } = table([goblin, orc]);
	controller.tool.hover(at(32, 32));
	assert.equal(controller.focus()?.id, "goblin");
	controller.selection.set(["goblin"]);
	assert.equal(controller.focus()?.id, "goblin", "hovering the selected pawn still labels it");
	controller.tool.hover(null);
	assert.equal(controller.focus(), null);
	assert.equal(controller.bounds(), null);
	controller.tool.hover(at(400, 400));
	assert.equal(controller.focus()?.id, "orc");
});
test("a token is never labelled", () => {
	const wagon = pawn({ id: "wagon", kind: "object", width: 128, height: 256, x: 0, y: 0 });
	const { controller } = table([wagon]);
	controller.tool.hover(at(0, 0));
	assert.equal(controller.focus(), null);
	assert.equal(controller.bounds(), null);
	controller.selection.set(["wagon"]);
	assert.equal(controller.focus(), null, "selecting a token labelled it");
	assert.equal(controller.handles([]).length, 9);
});
test("a group is boxed by the whole selection", () => {
	const a = pawn({ id: "a", x: 0, y: 0 });
	const b = pawn({ id: "b", x: 400, y: 0 });
	const { controller } = table([a, b]);
	controller.selection.set(["a", "b"]);
	controller.tool.hover(null);
	const box = controller.bounds();
	assert.deepEqual([box?.x1, box?.x2], [-32, 432], "the group's box is not both of them");
});
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
	const corner = handles.find((h) => h.lx === 1 && h.ly === 1 && !h.turns);
	assert.deepEqual([corner?.x, corner?.y], [64, 128]);
});
test("dragging an edge handle resizes about the centre", () => {
	const wagon = pawn({ id: "wagon", kind: "object", width: 128, height: 256, x: 0, y: 0 });
	const { controller, sent } = table([wagon]);
	controller.selection.set(["wagon"]);
	const claimed = controller.tool.press(at(64, 0), at(0, 0), NONE);
	assert.equal(claimed, true, "the camera was allowed to pan from a handle");
	controller.tool.drag(at(100, 0), at(36, 0), NONE);
	const ghost = controller.ghosts([])[0];
	assert.deepEqual([ghost?.width, ghost?.height], [200, 256], "the preview did not follow the hand");
	assert.deepEqual([ghost?.x, ghost?.y], [0, 0], "the token moved while it was being resized");
	controller.tool.release(at(100, 0), at(36, 0), NONE);
	assert.deepEqual(sent, [{ type: "pawn.update", id: "wagon", width: 200, height: 256 }]);
});
test("dragging the rotate handle turns the token about its centre", () => {
	const wagon = pawn({ id: "wagon", kind: "object", width: 128, height: 256, x: 0, y: 0 });
	const { controller, sent } = table([wagon]);
	controller.selection.set(["wagon"]);
	const spinner = controller.handles([]).find((h) => h.turns);
	assert.ok(spinner, "there is no rotate handle");
	assert.ok(spinner.y > 0, "the rotate handle is above the token, under the overlay");
	controller.tool.press(at(spinner.x, spinner.y), at(0, 0), NONE);
	controller.tool.drag(at(-300, -4), at(0, 0), NONE);
	assert.equal(controller.ghosts([])[0]?.rotation, 91);
	controller.tool.drag(at(-300, -4), at(0, 0), SHIFT);
	assert.equal(controller.ghosts([])[0]?.rotation, 90);
	controller.tool.release(at(-300, -4), at(0, 0), SHIFT);
	assert.deepEqual(sent, [{ type: "pawn.update", id: "wagon", rotation: 90 }]);
});
test("a handle pressed and released sends nothing", () => {
	const wagon = pawn({ id: "wagon", kind: "object", width: 128, height: 256, x: 0, y: 0 });
	const { controller, sent } = table([wagon]);
	controller.selection.set(["wagon"]);
	controller.tool.press(at(64, 128), at(0, 0), NONE);
	controller.tool.release(at(64, 128), at(0, 0), NONE);
	assert.deepEqual(sent, []);
});
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
const FOG_ON = { fogging: true, fogEnabled: true, fogPrefill: true } as const;
function cleared(): FogShape {
	return {
		id: "01CLEARED", layerId: GROUND, kind: "rect", mode: "reveal",
		points: [0, 0, 128, 128],
	};
}
test("a fog rectangle sends its corners snapped and normalised", () => {
	const { controller, sent } = table([], FOG_ON);
	controller.tool.press(at(200, 200), at(0, 0), NONE);
	controller.tool.drag(at(70, 70), at(0, 0), NONE);
	controller.tool.release(at(70, 70), at(0, 0), NONE);
	assert.deepEqual(sent, [{
		type: "fog.add", layer: GROUND, kind: "rect", mode: "reveal",
		points: [64, 64, 192, 192],
	}]);
});
test("a fog rectangle that snaps to nothing sends nothing", () => {
	const { controller, sent } = table([], FOG_ON);
	controller.tool.press(at(70, 70), at(0, 0), NONE);
	controller.tool.drag(at(80, 80), at(0, 0), NONE);
	controller.tool.release(at(80, 80), at(0, 0), NONE);
	assert.deepEqual(sent, []);
});
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
test("a fog rectangle is previewed while it is dragged", () => {
	const { controller } = table([], FOG_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(200, 130), at(0, 0), NONE);
	const [box, ...rest] = controller.outlines([]);
	assert.equal(rest.length, 0, "something else is on the table as well");
	assert.ok(box, "the rectangle in hand is not drawn");
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
test("the fog rectangle is coloured by the mode it will send", () => {
	const { controller, chooseFog } = table([], FOG_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(200, 130), at(0, 0), NONE);
	const uncover = controller.outlines([])[0].color;
	chooseFog("rect", "hide");
	const cover = controller.outlines([])[0].color;
	assert.notDeepEqual(uncover, cover);
});
const PEN_ON = { inking: true } as const;
test("the pen is not the tool unless it is chosen", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, sent } = table([goblin]);
	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.release(at(32, 32), at(0, 0), NONE);
	assert.deepEqual(sent, [], "a press under the Select tool drew something");
	assert.deepEqual(controller.selection.ids(), ["goblin"]);
});
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
	assert.equal(typeof begin.id, "string");
	assert.equal(sent[1].id, begin.id);
	assert.equal(sent[2].id, begin.id);
});
test("a pen click is a stroke of one point", () => {
	const { controller, sent } = table([], PEN_ON);
	controller.tool.press(at(48, 48), at(0, 0), NONE);
	controller.tool.release(at(48, 48), at(0, 0), NONE);
	assert.deepEqual(sent.map((c) => c.type), ["stroke.begin", "stroke.end"]);
	assert.deepEqual(sent[0].points, [48, 48]);
});
test("the pen drops points the hand did not really move", () => {
	const { controller, sent } = table([], { ...PEN_ON, scale: 20 });
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	for (let i = 1; i <= 8; i++) {
		controller.tool.drag(at(i, 0), at(0, 0), NONE);
	}
	controller.tool.release(at(8, 0), at(0, 0), NONE);
	assert.deepEqual(sent.map((c) => c.type), ["stroke.begin", "stroke.extend", "stroke.end"]);
	assert.deepEqual(sent[1].points, [8, 0]);
});
test("the pen keeps the point it was lifted at", () => {
	const { controller, sent } = table([], { ...PEN_ON, scale: 50 });
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(10, 0), at(0, 0), NONE);
	controller.tool.release(at(10, 0), at(0, 0), NONE);
	assert.deepEqual(sent[1].points, [10, 0]);
});
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
test("a cancelled pointer rubs the line out", () => {
	const { controller, sent } = table([], PEN_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(100, 100), at(0, 0), NONE);
	controller.tool.cancel();
	assert.deepEqual(sent.map((c) => c.type).at(-1), "stroke.erase");
});
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
test("a stroke survives the tool being switched away mid-gesture", () => {
	const { controller, sent, drawTool } = table([], PEN_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	drawTool(false);
	controller.tool.drag(at(100, 0), at(0, 0), NONE);
	controller.tool.release(at(100, 0), at(0, 0), NONE);
	assert.deepEqual(sent.map((c) => c.type), ["stroke.begin", "stroke.extend", "stroke.end"]);
});
const ERASE_ON = { inking: true, drawMode: "erase" } as const;
function drawn(over: Partial<Stroke> = {}): Stroke {
	return {
		id: "01LINE", by: GM, layerId: GROUND, kind: "free",
		color: "#FF0000", width: 4, points: [0, 0, 100, 0], done: true, ...over,
	};
}
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
test("an eraser sweep over empty floor sends nothing", () => {
	const { controller, sent } = table([], { ...ERASE_ON, strokes: [drawn()] });
	controller.tool.press(at(0, 400), at(0, 0), NONE);
	controller.tool.drag(at(100, 400), at(0, 0), NONE);
	controller.tool.release(at(100, 400), at(0, 0), NONE);
	assert.deepEqual(sent, []);
});
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
test("Escape mid-sweep rubs nothing out", () => {
	const { controller, sent } = table([], { ...ERASE_ON, strokes: [drawn()] });
	controller.tool.press(at(50, 0), at(0, 0), NONE);
	press("Escape");
	controller.tool.release(at(50, 0), at(0, 0), NONE);
	assert.deepEqual(sent, []);
});
test("the eraser draws a ring under the pointer", () => {
	const { controller, chooseDraw } = table([], { ...ERASE_ON, scale: 2 });
	controller.tool.hover(at(80, 90));
	const ring = controller.outlines([]).at(-1);
	assert.equal(ring?.x, 80);
	assert.equal(ring?.y, 90);
	assert.equal(ring?.halfW, 12, "the reach is six CSS pixels at this zoom");
	assert.equal(ring?.rect, false);
	chooseDraw("pen");
	assert.deepEqual(controller.outlines([]), []);
});
test("the eraser's ring goes when the pointer leaves the table", () => {
	const { controller } = table([], ERASE_ON);
	controller.tool.hover(at(80, 90));
	controller.tool.hover(null);
	assert.deepEqual(controller.outlines([]), []);
});
test("Ctrl+Z takes back the viewer's newest finished line", () => {
	const first = drawn({ id: "01AAA", by: GM });
	const second = drawn({ id: "01BBB", by: GM });
	const { controller, sent } = table([], { inking: true, strokes: [first, second] });
	void controller;
	press("z", null, { ctrlKey: true });
	assert.deepEqual(sent, [{ type: "stroke.erase", ids: ["01BBB"] }]);
});
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
test("Ctrl+Z under another tool is not the drawing's", () => {
	const { controller, sent } = table([], { inking: false, strokes: [drawn({ by: GM })] });
	void controller;
	press("z", null, { ctrlKey: true });
	assert.deepEqual(sent, []);
});
test("a stroke goes out in the colour and width the pill was left on", () => {
	const { controller, sent, chooseBrush } = table([], PEN_ON);
	chooseBrush("#00FF88", 21);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.release(at(0, 0), at(0, 0), NONE);
	assert.equal(sent[0].color, "#00FF88");
	assert.equal(sent[0].width, 21);
});
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
test("a fat line is easier for the eraser to catch than a thin one", () => {
	const fat = drawn({ id: "01FAT", width: 24, points: [0, 0, 100, 0] });
	const thin = drawn({ id: "01THIN", width: 2, points: [0, 200, 100, 200] });
	const { controller, sent } = table([], { ...ERASE_ON, strokes: [fat, thin] });
	controller.tool.press(at(50, 15), at(0, 0), NONE);
	controller.tool.drag(at(50, 215), at(0, 0), NONE);
	controller.tool.release(at(50, 215), at(0, 0), NONE);
	assert.deepEqual(sent, [{ type: "stroke.erase", ids: ["01FAT"] }]);
});
const RECT_ON = { inking: true, drawMode: "rect" } as const;
const CIRCLE_ON = { inking: true, drawMode: "circle" } as const;
test("a rectangle lands in one command with its corners normalised", () => {
	const { controller, sent } = table([], RECT_ON);
	controller.tool.press(at(300, 260), at(0, 0), NONE);
	controller.tool.drag(at(100, 60), at(0, 0), NONE);
	controller.tool.release(at(100, 60), at(0, 0), NONE);
	assert.deepEqual(sent, [{
		type: "stroke.begin", id: sent[0].id, layer: GROUND, kind: "rect",
		color: "#FF0000", width: 4, points: [100, 60, 300, 260],
	}]);
});
test("a circle lands as its centre then a point on its rim", () => {
	const { controller, sent } = table([], CIRCLE_ON);
	controller.tool.press(at(200, 200), at(0, 0), NONE);
	controller.tool.drag(at(120, 140), at(0, 0), NONE);
	controller.tool.release(at(120, 140), at(0, 0), NONE);
	assert.deepEqual(sent.map((c) => c.type), ["stroke.begin"]);
	assert.equal(sent[0].kind, "circle");
	assert.deepEqual(sent[0].points, [200, 200, 120, 140]);
});
test("a shape with no size sends nothing", () => {
	for (const over of [RECT_ON, CIRCLE_ON]) {
		const { controller, sent } = table([], over);
		controller.tool.press(at(50, 50), at(0, 0), NONE);
		controller.tool.release(at(50.4, 50.4), at(0, 0), NONE);
		assert.deepEqual(sent, [], over.drawMode);
	}
});
test("a rectangle with only one axis sends nothing", () => {
	const { controller, sent } = table([], RECT_ON);
	controller.tool.press(at(0, 100), at(0, 0), NONE);
	controller.tool.drag(at(300, 100), at(0, 0), NONE);
	controller.tool.release(at(300, 100), at(0, 0), NONE);
	assert.deepEqual(sent, []);
});
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
test("a rectangle previews as a box between its corners", () => {
	const { controller } = table([], RECT_ON);
	controller.tool.press(at(100, 60), at(0, 0), NONE);
	controller.tool.drag(at(300, 260), at(0, 0), NONE);
	const box = controller.outlines([])[0];
	assert.equal(box?.rect, true);
	assert.deepEqual([box?.x, box?.y], [200, 160], "not centred between the corners");
	assert.deepEqual([box?.halfW, box?.halfH], [100, 100]);
});
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
test("a shape being dragged carries its distance", () => {
	const { controller } = table([], CIRCLE_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(256, 0), at(0, 0), NONE);
	assert.deepEqual(controller.labels([]).map((l) => l.text), ["20 ft."]);
});
test("a shape on the floor carries no distance", () => {
	const strokes = [
		drawn({ id: "01CIRCLE", kind: "circle", points: [0, 0, 256, 0] }),
		drawn({ id: "01RECT", kind: "rect", points: [0, 0, 384, 128] }),
		drawn({ id: "01CONE", kind: "cone", points: [0, 0, 0, 384] }),
	];
	const { controller } = table([], { inking: true, strokes });
	assert.deepEqual(controller.labels([]), []);
});
test("a shape's distance goes the moment it lands", () => {
	const { controller } = table([], CIRCLE_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(256, 0), at(0, 0), NONE);
	assert.equal(controller.labels([]).length, 1, "no number while it is in hand");
	controller.tool.release(at(256, 0), at(0, 0), NONE);
	assert.deepEqual(controller.labels([]), [], "the number outlived the drag");
});
test("an abandoned shape's distance goes with it", () => {
	const { controller } = table([], RECT_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(200, 200), at(0, 0), NONE);
	controller.tool.secondary(at(200, 200), at(0, 0));
	assert.deepEqual(controller.labels([]), []);
});
const CONE_ON = { inking: true, drawMode: "cone" } as const;
test("a cone lands as its point then the middle of its base", () => {
	const { controller, sent } = table([], CONE_ON);
	controller.tool.press(at(100, 100), at(0, 0), NONE);
	controller.tool.drag(at(100, 400), at(0, 0), NONE);
	controller.tool.release(at(100, 400), at(0, 0), NONE);
	assert.deepEqual(sent, [{
		type: "stroke.begin", id: sent[0].id, layer: GROUND, kind: "cone",
		color: "#FF0000", width: 4, points: [100, 100, 100, 400],
	}]);
});
test("a cone with no length sends nothing", () => {
	const { controller, sent } = table([], CONE_ON);
	controller.tool.press(at(50, 50), at(0, 0), NONE);
	controller.tool.release(at(50.3, 50.3), at(0, 0), NONE);
	assert.deepEqual(sent, []);
});
test("Escape and the right button drop a cone silently", () => {
	const escaped = table([], CONE_ON);
	escaped.controller.tool.press(at(0, 0), at(0, 0), NONE);
	escaped.controller.tool.drag(at(0, 200), at(0, 0), NONE);
	press("Escape");
	escaped.controller.tool.release(at(0, 200), at(0, 0), NONE);
	assert.deepEqual(escaped.sent, []);
	const clicked = table([], CONE_ON);
	clicked.controller.tool.press(at(0, 0), at(0, 0), NONE);
	clicked.controller.tool.drag(at(0, 200), at(0, 0), NONE);
	clicked.controller.tool.secondary(at(0, 200), at(0, 0));
	clicked.controller.tool.release(at(0, 200), at(0, 0), NONE);
	assert.deepEqual(clicked.sent, []);
});
test("a cone previews as three lines and no outline", () => {
	const { controller } = table([], CONE_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(0, 100), at(0, 0), NONE);
	assert.deepEqual(controller.outlines([]), [], "a cone took the ring pass");
	const sides = controller.marks([]);
	assert.equal(sides.length, 3);
	assert.deepEqual(
		sides.map((s) => [s.x0, s.y0, s.x1, s.y1]),
		[[0, 0, -50, 100], [-50, 100, 50, 100], [50, 100, 0, 0]],
	);
});
test("the cone preview is the colour it will land in", () => {
	const { controller, chooseBrush } = table([], CONE_ON);
	chooseBrush("#0000FF", 4);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(0, 100), at(0, 0), NONE);
	assert.deepEqual(controller.marks([])[0].color, [0, 0, 1]);
});
test("a cone that has not left its point previews nothing", () => {
	const { controller } = table([], CONE_ON);
	controller.tool.press(at(50, 50), at(0, 0), NONE);
	controller.tool.drag(at(50, 50), at(0, 0), NONE);
	assert.deepEqual(controller.marks([]), []);
	assert.deepEqual(controller.outlines([]), []);
});
test("an abandoned cone takes its preview off the table", () => {
	const { controller } = table([], CONE_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(0, 200), at(0, 0), NONE);
	controller.tool.secondary(at(0, 200), at(0, 0));
	assert.deepEqual(controller.marks([]), []);
});
test("a cone being dragged carries its length", () => {
	const { controller } = table([], CONE_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(0, 384), at(0, 0), NONE);
	assert.deepEqual(controller.labels([]).map((l) => l.text), ["30 ft."]);
});
test("the fog's marks and the cone's do not tread on each other", () => {
	const { controller } = table([], { ...FOG_ON, shape: "poly" });
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.press(at(200, 0), at(0, 0), NONE);
	controller.tool.hover(at(200, 200));
	assert.ok(controller.marks([]).length > 0, "the fog's polygon lost its lines");
});
const PING_ON = { pinging: true } as const;
test("a press points at the square under it and sends nothing else", () => {
	const { controller, sent } = table([], PING_ON);
	controller.tool.press(at(199.6, -40.2), at(0, 0), NONE);
	controller.tool.drag(at(240, -40), at(0, 0), NONE);
	controller.tool.release(at(240, -40), at(0, 0), NONE);
	assert.deepEqual(sent, [{ type: "ping", layer: GROUND, x: 200, y: -40 }]);
});
test("the camera keeps out of a ping", () => {
	const { controller } = table([], PING_ON);
	assert.equal(controller.tool.press(at(10, 10), at(0, 0), NONE), true);
});
test("pointing at a goblin neither moves it nor picks it out", () => {
	const { controller, sent } = table([pawn({ x: 0, y: 0 })], PING_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(200, 200), at(0, 0), NONE);
	controller.tool.release(at(200, 200), at(0, 0), NONE);
	assert.deepEqual(sent.map((c) => c.type), ["ping"]);
	assert.deepEqual(controller.selection.ids(), []);
	assert.equal(controller.ghosts([]).length, 0);
});
test("a ping leaves a selection where it was", () => {
	const harness = table([pawn({ x: 0, y: 0 })], {});
	harness.controller.tool.press(at(0, 0), at(0, 0), NONE);
	harness.controller.tool.release(at(0, 0), at(0, 0), NONE);
	assert.deepEqual(harness.controller.selection.ids(), ["01PAWN"]);
	harness.pingTool(true);
	harness.controller.tool.press(at(400, 400), at(0, 0), NONE);
	assert.deepEqual(harness.controller.selection.ids(), ["01PAWN"]);
});
test("with the pointer unchosen a press pings nothing", () => {
	const { controller, sent } = table([], {});
	controller.tool.press(at(10, 10), at(0, 0), NONE);
	controller.tool.release(at(10, 10), at(0, 0), NONE);
	assert.deepEqual(sent.filter((c) => c.type === "ping"), []);
});
