import assert from "node:assert/strict";
import { test } from "node:test";
import { CELLAR, GM, GROUND, NONE, PLAYER, at, drawn, pawn, press, table } from "./testing.ts";
const PEN_ON = { mode: "draw" } as const;
const ERASE_ON = { mode: "draw", drawMode: "erase" } as const;
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
	const { controller, inHand } = table([], PEN_ON);
	assert.equal(inHand(), null);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(100, 0), at(0, 0), NONE);
	const line = inHand();
	assert.equal(line?.kind, "free");
	assert.equal(line?.layerId, GROUND);
	assert.deepEqual(line?.points, [0, 0, 100, 0]);
	controller.tool.release(at(100, 0), at(0, 0), NONE);
	assert.equal(inHand(), null);
});
test("a stroke survives the tool being switched away mid-gesture", () => {
	const { controller, sent, choose } = table([], PEN_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	choose("select");
	controller.tool.drag(at(100, 0), at(0, 0), NONE);
	controller.tool.release(at(100, 0), at(0, 0), NONE);
	assert.deepEqual(sent.map((c) => c.type), ["stroke.begin", "stroke.extend", "stroke.end"]);
});
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
test("a fat line is easier for the eraser to catch than a thin one", () => {
	const fat = drawn({ id: "01FAT", width: 24, points: [0, 0, 100, 0] });
	const thin = drawn({ id: "01THIN", width: 2, points: [0, 200, 100, 200] });
	const { controller, sent } = table([], { ...ERASE_ON, strokes: [fat, thin] });
	controller.tool.press(at(50, 15), at(0, 0), NONE);
	controller.tool.drag(at(50, 215), at(0, 0), NONE);
	controller.tool.release(at(50, 215), at(0, 0), NONE);
	assert.deepEqual(sent, [{ type: "stroke.erase", ids: ["01FAT"] }]);
});
test("Ctrl+Z takes back the viewer's newest finished line", () => {
	const first = drawn({ id: "01AAA", by: GM });
	const second = drawn({ id: "01BBB", by: GM });
	const { sent } = table([], { ...PEN_ON, strokes: [first, second] });
	press("z", null, { ctrlKey: true });
	assert.deepEqual(sent, [{ type: "stroke.erase", ids: ["01BBB"] }]);
});
test("Ctrl+Z skips somebody else's line", () => {
	const mine = drawn({ id: "01AAA", by: GM });
	const theirs = drawn({ id: "01ZZZ", by: PLAYER });
	const { sent } = table([], { ...PEN_ON, strokes: [mine, theirs] });
	press("z", null, { ctrlKey: true });
	assert.deepEqual(sent, [{ type: "stroke.erase", ids: ["01AAA"] }]);
});
test("Ctrl+Z skips a line on another floor and one still being drawn", () => {
	const upstairs = drawn({ id: "01AAA", by: GM, layerId: CELLAR });
	const growing = drawn({ id: "01BBB", by: GM, done: false });
	const { sent } = table([], { ...PEN_ON, strokes: [upstairs, growing] });
	press("z", null, { ctrlKey: true });
	assert.deepEqual(sent, []);
});
test("Ctrl+Z under another tool is not the drawing's", () => {
	const { sent } = table([], { strokes: [drawn({ by: GM })] });
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
