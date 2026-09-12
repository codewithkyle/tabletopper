import assert from "node:assert/strict";
import { test } from "node:test";
import { NONE, SHIFT, at, pawn, table, wait } from "./testing.ts";
type Harness = ReturnType<typeof table>;
function click(t: Harness, x: number, y: number, mods = NONE): void {
	t.controller.tool.press(at(x, y), at(x, y), mods);
	t.controller.tool.release(at(x, y), at(x, y), mods);
}
test("a double click opens the pawn's window and one click does not", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const t = table([goblin]);
	click(t, 32, 32);
	assert.deepEqual(t.opened, [], "one click opened a window");
	click(t, 32, 32);
	assert.deepEqual(t.opened, ["goblin"]);
	assert.deepEqual(t.controller.selection.ids(), ["goblin"]);
});
test("a third click is not a second double click", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const t = table([goblin]);
	click(t, 32, 32);
	click(t, 32, 32);
	click(t, 32, 32);
	assert.deepEqual(t.opened, ["goblin"]);
});
test("two clicks far enough apart are two clicks", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const t = table([goblin]);
	click(t, 32, 32);
	wait(500);
	click(t, 32, 32);
	assert.deepEqual(t.opened, []);
});
test("two clicks on two pawns are two clicks", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const orc = pawn({ id: "orc", x: 400, y: 400 });
	const t = table([goblin, orc]);
	click(t, 32, 32);
	click(t, 400, 400);
	assert.deepEqual(t.opened, []);
});
test("a drag between two clicks breaks the pair", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const t = table([goblin]);
	click(t, 32, 32);
	t.controller.tool.press(at(32, 32), at(0, 0), NONE);
	t.controller.tool.drag(at(200, 200), at(168, 168), NONE);
	t.controller.tool.release(at(200, 200), at(168, 168), NONE);
	click(t, 32, 32);
	assert.deepEqual(t.opened, []);
});
test("a player opens a monster they may not move", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const t = table([goblin], { role: "player", user: "01PLAYER" });
	click(t, 32, 32);
	click(t, 32, 32);
	assert.deepEqual(t.opened, ["goblin"]);
	assert.deepEqual(t.controller.selection.ids(), [], "a player selected a monster they may not move");
});
test("shift clicks never open anything", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const t = table([goblin]);
	click(t, 32, 32, SHIFT);
	click(t, 32, 32, SHIFT);
	assert.deepEqual(t.opened, []);
	assert.deepEqual(t.controller.selection.ids(), [], "the second shift click did not toggle it back out");
});
test("handles are drawn for one selected token and for nothing else", () => {
	const wagon = pawn({ id: "wagon", kind: "object", width: 128, height: 256, x: 0, y: 0 });
	const goblin = pawn({ id: "goblin", x: 400, y: 400 });
	const { controller, handles } = table([wagon, goblin]);
	assert.deepEqual(handles(), [], "nothing is selected and there are handles");
	controller.selection.set(["goblin"]);
	assert.deepEqual(handles(), [], "a creature was given resize handles");
	controller.selection.set(["wagon", "goblin"]);
	assert.deepEqual(handles(), [], "a multiple selection was given handles");
	controller.selection.set(["wagon"]);
	const found = handles();
	assert.equal(found.length, 9, "eight resize handles and one rotate");
	assert.equal(found.filter((h) => h.turns).length, 1);
	const corner = found.find((h) => h.lx === 1 && h.ly === 1 && !h.turns);
	assert.deepEqual([corner?.x, corner?.y], [64, 128]);
});
test("dragging an edge handle resizes about the centre", () => {
	const wagon = pawn({ id: "wagon", kind: "object", width: 128, height: 256, x: 0, y: 0 });
	const { controller, sent, ghosts } = table([wagon]);
	controller.selection.set(["wagon"]);
	const claimed = controller.tool.press(at(64, 0), at(0, 0), NONE);
	assert.equal(claimed, true, "the camera was allowed to pan from a handle");
	controller.tool.drag(at(100, 0), at(36, 0), NONE);
	const ghost = ghosts()[0];
	assert.deepEqual([ghost?.width, ghost?.height], [200, 256], "the preview did not follow the hand");
	assert.deepEqual([ghost?.x, ghost?.y], [0, 0], "the token moved while it was being resized");
	controller.tool.release(at(100, 0), at(36, 0), NONE);
	assert.deepEqual(sent, [{ type: "pawn.update", id: "wagon", width: 200, height: 256 }]);
});
test("dragging the rotate handle turns the token about its centre", () => {
	const wagon = pawn({ id: "wagon", kind: "object", width: 128, height: 256, x: 0, y: 0 });
	const { controller, sent, ghosts, handles } = table([wagon]);
	controller.selection.set(["wagon"]);
	const spinner = handles().find((h) => h.turns);
	assert.ok(spinner, "there is no rotate handle");
	assert.ok(spinner.y > 0, "the rotate handle is above the token, under the overlay");
	controller.tool.press(at(spinner.x, spinner.y), at(0, 0), NONE);
	controller.tool.drag(at(-300, -4), at(0, 0), NONE);
	assert.equal(ghosts()[0]?.rotation, 91);
	controller.tool.drag(at(-300, -4), at(0, 0), SHIFT);
	assert.equal(ghosts()[0]?.rotation, 90);
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
	const { controller, sent, ghosts } = table([wagon]);
	controller.selection.set(["wagon"]);
	controller.tool.press(at(64, 0), at(0, 0), NONE);
	controller.tool.drag(at(300, 0), at(0, 0), NONE);
	controller.tool.secondary(at(300, 0), at(0, 0));
	assert.deepEqual(sent, []);
	assert.deepEqual(ghosts(), [], "the proposal outlived the gesture");
});
