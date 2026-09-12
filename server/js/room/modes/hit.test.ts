import assert from "node:assert/strict";
import { test } from "node:test";
import { CELLAR, GROUND, NONE, PLAYER, at, cleared, grid, pawn, table } from "./testing.ts";
import { hitTest } from "./hit.ts";
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
const DARK = {
	role: "player", user: PLAYER, fogEnabled: true, fogPrefill: true,
} as const;
test("a player finds nothing under the cover", () => {
	const goblin = pawn({ id: "goblin", x: 400, y: 400 });
	const { controller } = table([goblin], { ...DARK, fog: [cleared()] });
	controller.tool.hover(at(400, 400));
	assert.equal(controller.focus(), null, "a concealed pawn was labelled");
	controller.tool.press(at(400, 400), at(0, 0), NONE);
	controller.tool.release(at(400, 400), at(0, 0), NONE);
	assert.deepEqual(controller.selection.ids(), [], "a concealed pawn was clicked");
});
test("a player's marquee does not sweep up what it cannot see", () => {
	const mine = pawn({ id: "mine", x: 400, y: 400, ownerId: PLAYER });
	const theirs = pawn({ id: "theirs", x: 420, y: 420 });
	const { controller } = table([mine, theirs], { ...DARK, fog: [cleared()] });
	controller.tool.press(at(300, 300), at(0, 0), NONE);
	controller.tool.drag(at(500, 500), at(90, 90), NONE);
	controller.tool.release(at(500, 500), at(90, 90), NONE);
	assert.deepEqual(controller.selection.ids(), ["mine"]);
});
test("a pawn on the cleared part of the floor is found", () => {
	const goblin = pawn({ id: "goblin", x: 64, y: 64 });
	const { controller } = table([goblin], { ...DARK, fog: [cleared()] });
	controller.tool.hover(at(64, 64));
	assert.equal(controller.focus()?.id, "goblin");
});
test("a floor whose fog is off conceals nothing", () => {
	const goblin = pawn({ id: "goblin", x: 400, y: 400 });
	const { controller } = table([goblin], { ...DARK, fogEnabled: false });
	controller.tool.hover(at(400, 400));
	assert.equal(controller.focus()?.id, "goblin");
});
test("the GM is concealed from nothing", () => {
	const goblin = pawn({ id: "goblin", x: 400, y: 400 });
	const { controller } = table([goblin], { fogEnabled: true, fogPrefill: true });
	controller.tool.hover(at(400, 400));
	assert.equal(controller.focus()?.id, "goblin");
});
test("a player keeps hold of their own pawn when it walks into the dark", () => {
	const mine = pawn({ id: "mine", kind: "player", x: 400, y: 400, ownerId: PLAYER });
	const { controller } = table([mine], { ...DARK, fog: [cleared()] });
	controller.tool.hover(at(400, 400));
	assert.equal(controller.focus()?.id, "mine", "the player could not reach their own token");
});
