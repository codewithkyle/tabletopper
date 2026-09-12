import assert from "node:assert/strict";
import { test } from "node:test";
import { ALT, NONE, SHIFT, at, pawn, press, table } from "./testing.ts";
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
	const { controller, outlines } = table([goblin]);
	controller.selection.set(["goblin"]);
	controller.tool.press(at(900, 900), at(400, 400), NONE);
	controller.tool.drag(at(902, 902), at(402, 402), NONE);
	controller.tool.release(at(902, 902), at(402, 402), NONE);
	assert.deepEqual(controller.selection.ids(), [], "a click on empty table did not clear the selection");
	assert.deepEqual(outlines(), [], "a click drew a marquee");
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
	const { controller, sent, outlines } = table([goblin], { mode: "pan" });
	assert.equal(controller.tool.press(at(32, 32), at(0, 0), NONE), false, "a press on a pawn was kept");
	controller.tool.drag(at(200, 200), at(168, 168), NONE);
	controller.tool.release(at(200, 200), at(168, 168), NONE);
	assert.equal(controller.tool.press(at(900, 900), at(0, 0), NONE), false, "a press on empty table was kept");
	controller.tool.drag(at(1200, 1200), at(300, 300), NONE);
	controller.tool.release(at(1200, 1200), at(300, 300), NONE);
	assert.deepEqual(sent, [], "the camera's mode moved something");
	assert.deepEqual(controller.selection.ids(), [], "the camera's mode selected something");
	assert.deepEqual(outlines(), [], "the camera's mode drew a marquee");
});
test("the move tool keeps the selection", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller } = table([goblin], { mode: "pan" });
	controller.selection.set(["goblin"]);
	controller.tool.press(at(900, 900), at(0, 0), NONE);
	controller.tool.release(at(900, 900), at(0, 0), NONE);
	assert.deepEqual(controller.selection.ids(), ["goblin"]);
});
test("the others move by the anchor's snapped delta and nothing else", () => {
	const anchor = pawn({ id: "anchor", x: 32, y: 32, z: 1 });
	const rider = pawn({ id: "rider", x: 100, y: 200, z: 2 });
	const { controller, ghosts } = table([anchor, rider]);
	controller.selection.set(["anchor", "rider"]);
	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.drag(at(90, 90), at(58, 58), NONE);
	const byID = new Map(ghosts().map((g) => [g.id, { x: g.x, y: g.y }]));
	assert.deepEqual(byID.get("anchor"), { x: 96, y: 96 });
	assert.deepEqual(byID.get("rider"), { x: 164, y: 264 });
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
	const { controller, sent, ghosts } = table([anchor, rider]);
	controller.selection.set(["anchor", "rider"]);
	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.drag(at(300, 300), at(268, 268), NONE);
	press("Escape");
	const move = sent[sent.length - 1];
	assert.equal(move?.type, "pawn.move");
	assert.deepEqual([move?.x, move?.y], [32, 32], "Escape did not put the anchor back");
	assert.deepEqual(move?.others, ["rider"]);
	assert.deepEqual(ghosts(), []);
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
test("a player pressing somebody else's pawn selects nothing", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32, ownerId: null });
	const { controller, sent } = table([goblin], { role: "player", user: "01ME" });
	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.drag(at(200, 200), at(168, 168), NONE);
	controller.tool.release(at(200, 200), at(168, 168), NONE);
	assert.deepEqual(sent, [], "a player moved a pawn that is not theirs");
	assert.deepEqual(controller.selection.ids(), []);
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
	const { controller, handles } = table([wagon]);
	controller.tool.hover(at(0, 0));
	assert.equal(controller.focus(), null);
	assert.equal(controller.bounds(), null);
	controller.selection.set(["wagon"]);
	assert.equal(controller.focus(), null, "selecting a token labelled it");
	assert.equal(handles().length, 9);
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
