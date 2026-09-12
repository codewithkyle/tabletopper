import assert from "node:assert/strict";
import { test } from "node:test";
import { NONE, at, pawn, press, table } from "./testing.ts";
const MEASURING = { mode: "measure" } as const;
test("the measure tool puts a point down and runs a line to the pointer", () => {
	const { controller, segments, labels, outlines } = table([], MEASURING);
	assert.equal(controller.tool.press(at(100, 100), at(0, 0), NONE), true, "the ruler gave the press away");
	controller.tool.release(at(100, 100), at(0, 0), NONE);
	controller.tool.hover(at(292, 100));
	const [line, ...rest] = segments();
	assert.deepEqual(rest, [], "one measurement drew more than one line");
	assert.deepEqual([line?.x0, line?.y0], [100, 100]);
	assert.deepEqual([line?.x1, line?.y1], [292, 100]);
	assert.deepEqual(labels().map((l) => l.text), ["15 ft."]);
	const [point, ...others] = outlines();
	assert.deepEqual(others, [], "the ruler drew more than the one point");
	assert.deepEqual([point?.x, point?.y], [100, 100]);
	assert.equal(point?.rect, false, "the point is not a ring");
});
test("a measurement is a straight line and not a square count", () => {
	const { controller, labels, cells } = table([], MEASURING);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.hover(at(192, 192));
	assert.deepEqual(labels().map((l) => l.text), ["21 ft."]);
	assert.deepEqual(cells(), [], "the free ruler tinted cells");
});
test("a measurement snaps to nothing at either end", () => {
	const { controller, segments } = table([], MEASURING);
	controller.tool.press(at(37, 91), at(0, 0), NONE);
	controller.tool.hover(at(52, 103));
	const [line] = segments();
	assert.deepEqual([line?.x0, line?.y0, line?.x1, line?.y1], [37, 91, 52, 103]);
});
test("a second press ends the measurement", () => {
	const { controller, segments, outlines } = table([], MEASURING);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.hover(at(64, 0));
	assert.equal(controller.tool.press(at(320, 0), at(0, 0), NONE), true, "the second press was given away");
	assert.deepEqual(segments(), [], "the second press left the ruler up");
	assert.deepEqual(outlines(), [], "the second press left the point down");
	controller.tool.hover(at(640, 0));
	assert.deepEqual(segments(), [], "the ruler came back on its own");
});
test("a press after the end starts a fresh measurement", () => {
	const { controller, segments, labels } = table([], MEASURING);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.press(at(320, 0), at(0, 0), NONE);
	controller.tool.hover(at(384, 0));
	const [line, ...rest] = segments();
	assert.deepEqual(rest, [], "the new measurement drew more than one line");
	assert.deepEqual([line?.x0, line?.x1], [320, 384]);
	assert.deepEqual(labels().map((l) => l.text), ["5 ft."]);
});
test("the measure tool moves nothing and selects nothing", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const ogre = pawn({ id: "ogre", x: 900, y: 900 });
	const { controller, sent } = table([goblin, ogre], MEASURING);
	controller.selection.set(["ogre"]);
	assert.equal(controller.tool.press(at(32, 32), at(0, 0), NONE), true, "the ruler gave a pawn away");
	controller.tool.drag(at(200, 200), at(168, 168), NONE);
	controller.tool.release(at(200, 200), at(168, 168), NONE);
	assert.deepEqual(sent, [], "the ruler moved something");
	assert.deepEqual(controller.selection.ids(), ["ogre"], "the ruler changed the selection");
});
test("Escape puts the ruler away", () => {
	const { controller, segments, outlines } = table([], MEASURING);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.hover(at(192, 0));
	press("Escape");
	assert.deepEqual(segments(), [], "Escape left the ruler up");
	assert.deepEqual(outlines(), [], "Escape left the point down");
});
test("leaving the measure tool forgets the measurement", () => {
	const { controller, choose, segments } = table([], MEASURING);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.release(at(0, 0), at(0, 0), NONE);
	controller.tool.hover(at(192, 0));
	choose("select");
	assert.deepEqual(segments(), [], "another tool kept the ruler up");
	choose("measure");
	assert.deepEqual(segments(), [], "the old ruler came back");
});
test("panning with the space bar does not put the ruler away", () => {
	const { controller, hold, segments } = table([], MEASURING);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.hover(at(192, 0));
	hold(true);
	assert.equal(controller.tool.press(at(400, 400), at(0, 0), NONE), false, "the hold did not reach the camera");
	const [line] = segments();
	assert.deepEqual([line?.x0, line?.x1], [0, 192], "the pan moved or dropped the ruler");
});
