import assert from "node:assert/strict";
import { test } from "node:test";
import { NONE, at, pawn, press, table } from "./testing.ts";
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
test("a drag survives the tool changing under it", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, sent, choose } = table([goblin]);
	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.drag(at(90, 90), at(58, 58), NONE);
	choose("pan");
	controller.tool.release(at(90, 90), at(58, 58), NONE);
	assert.equal(sent[sent.length - 1]?.type, "pawn.move", "the drag was abandoned mid-flight");
});
test("the tool the toolbar moved to takes the next press", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, sent, choose } = table([goblin]);
	choose("ping");
	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.release(at(32, 32), at(0, 0), NONE);
	assert.deepEqual(sent.map((c) => c.type), ["ping"]);
	assert.deepEqual(controller.selection.ids(), [], "the pointer selected as well as pointing");
});
test("the right button falls back to the pawn menu under another tool", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, menus } = table([goblin], { mode: "fog", fogEnabled: true });
	controller.tool.secondary(at(32, 32), at(0, 0));
	assert.deepEqual(menus, ["goblin"], "the fog swallowed a right click it had no use for");
});
test("a key reaches the tool that is chosen and no other", () => {
	const { controller, sent, choose } = table([], {
		mode: "fog", fogEnabled: true, shape: "poly",
	});
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.press(at(200, 0), at(0, 0), NONE);
	controller.tool.release(at(200, 0), at(0, 0), NONE);
	choose("select");
	press("Backspace");
	press("Enter");
	assert.deepEqual(sent, [], "the fog answered a key meant for the select tool");
});
