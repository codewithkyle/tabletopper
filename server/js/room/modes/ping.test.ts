import assert from "node:assert/strict";
import { test } from "node:test";
import { GROUND, NONE, at, pawn, table } from "./testing.ts";
const PINGING = { mode: "ping" } as const;
test("a press points at the square under it and sends nothing else", () => {
	const { controller, sent } = table([], PINGING);
	controller.tool.press(at(199.6, -40.2), at(0, 0), NONE);
	controller.tool.drag(at(240, -40), at(0, 0), NONE);
	controller.tool.release(at(240, -40), at(0, 0), NONE);
	assert.deepEqual(sent, [{ type: "ping", layer: GROUND, x: 200, y: -40 }]);
});
test("the camera keeps out of a ping", () => {
	const { controller } = table([], PINGING);
	assert.equal(controller.tool.press(at(10, 10), at(0, 0), NONE), true);
});
test("pointing at a goblin neither moves it nor picks it out", () => {
	const { controller, sent, ghosts } = table([pawn({ x: 0, y: 0 })], PINGING);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(200, 200), at(0, 0), NONE);
	controller.tool.release(at(200, 200), at(0, 0), NONE);
	assert.deepEqual(sent.map((c) => c.type), ["ping"]);
	assert.deepEqual(controller.selection.ids(), []);
	assert.equal(ghosts().length, 0);
});
test("a ping leaves a selection where it was", () => {
	const t = table([pawn({ x: 0, y: 0 })], {});
	t.controller.tool.press(at(0, 0), at(0, 0), NONE);
	t.controller.tool.release(at(0, 0), at(0, 0), NONE);
	assert.deepEqual(t.controller.selection.ids(), ["01PAWN"]);
	t.choose("ping");
	t.controller.tool.press(at(400, 400), at(0, 0), NONE);
	assert.deepEqual(t.controller.selection.ids(), ["01PAWN"]);
});
test("with the pointer unchosen a press pings nothing", () => {
	const { controller, sent } = table([], {});
	controller.tool.press(at(10, 10), at(0, 0), NONE);
	controller.tool.release(at(10, 10), at(0, 0), NONE);
	assert.deepEqual(sent.filter((c) => c.type === "ping"), []);
});
