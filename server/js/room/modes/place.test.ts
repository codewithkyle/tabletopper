import assert from "node:assert/strict";
import { test } from "node:test";
import { GROUND, NONE, at, pawn, press, table } from "./testing.ts";
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
	const { controller, sent } = table([], { mode: "pan" });
	controller.arm({
		kind: "monster", id: "01MONSTER", name: "Goblin", image: "",
		visible: false, size: "medium", width: 0, height: 0,
		hp: 0, maxHp: 0, ac: 0,
	});
	assert.equal(controller.tool.press(at(90, 90), at(0, 0), NONE), false);
	assert.deepEqual(sent, []);
	assert.equal(controller.isArmed(), true, "the camera's mode disarmed what was held");
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
test("what is armed follows the pointer until it is put down", () => {
	const { controller, ghosts } = table([]);
	assert.deepEqual(ghosts(), [], "an unarmed table drew a ghost");
	controller.arm({
		kind: "monster", id: "01MONSTER", name: "Goblin", image: "",
		visible: true, size: "medium", width: 0, height: 0,
		hp: 0, maxHp: 0, ac: 0,
	});
	controller.tool.hover(at(90, 90));
	const [ghost, ...rest] = ghosts();
	assert.deepEqual(rest, [], "one armed encounter drew more than one ghost");
	assert.deepEqual([ghost?.x, ghost?.y], [96, 96], "the ghost was not snapped");
	controller.arm(null);
	assert.deepEqual(ghosts(), []);
});
