import assert from "node:assert/strict";
import { test } from "node:test";
import { GROUND, NONE, at, drawn, press, table } from "./testing.ts";
const RECT_ON = { mode: "draw", drawMode: "rect" } as const;
const CIRCLE_ON = { mode: "draw", drawMode: "circle" } as const;
const CONE_ON = { mode: "draw", drawMode: "cone" } as const;
const ERASE_ON = { mode: "draw", drawMode: "erase" } as const;
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
	const { controller, outlines } = table([], RECT_ON);
	controller.tool.press(at(100, 60), at(0, 0), NONE);
	controller.tool.drag(at(300, 260), at(0, 0), NONE);
	const box = outlines()[0];
	assert.equal(box?.rect, true);
	assert.deepEqual([box?.x, box?.y], [200, 160], "not centred between the corners");
	assert.deepEqual([box?.halfW, box?.halfH], [100, 100]);
});
test("a circle previews as a ring around where it was pressed", () => {
	const { controller, outlines } = table([], CIRCLE_ON);
	controller.tool.press(at(200, 200), at(0, 0), NONE);
	controller.tool.drag(at(200 + 60, 200 + 80), at(0, 0), NONE);
	const ring = outlines()[0];
	assert.equal(ring?.rect, false);
	assert.deepEqual([ring?.x, ring?.y], [200, 200], "the ring moved off the press");
	assert.equal(ring?.halfW, 100, "a 3-4-5 rim is not a radius of 100");
	assert.equal(ring?.halfW, ring?.halfH, "the circle is an ellipse");
});
test("a shape that has not left its first point previews nothing", () => {
	for (const over of [RECT_ON, CIRCLE_ON]) {
		const { controller, outlines } = table([], over);
		controller.tool.press(at(50, 50), at(0, 0), NONE);
		controller.tool.drag(at(50, 50), at(0, 0), NONE);
		assert.deepEqual(outlines(), [], over.drawMode);
	}
});
test("the preview is the colour the shape will land in", () => {
	const { controller, chooseBrush, outlines } = table([], RECT_ON);
	chooseBrush("#00FF00", 4);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(200, 200), at(0, 0), NONE);
	assert.deepEqual(outlines()[0].color, [0, 1, 0]);
});
test("an abandoned shape takes its preview off the table", () => {
	const { controller, outlines } = table([], RECT_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(200, 200), at(0, 0), NONE);
	controller.tool.secondary(at(200, 200), at(0, 0));
	assert.deepEqual(outlines(), []);
});
test("a shape being dragged carries its distance", () => {
	const { controller, labels } = table([], CIRCLE_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(256, 0), at(0, 0), NONE);
	assert.deepEqual(labels().map((l) => l.text), ["20 ft."]);
});
test("a shape on the floor carries no distance", () => {
	const strokes = [
		drawn({ id: "01CIRCLE", kind: "circle", points: [0, 0, 256, 0] }),
		drawn({ id: "01RECT", kind: "rect", points: [0, 0, 384, 128] }),
		drawn({ id: "01CONE", kind: "cone", points: [0, 0, 0, 384] }),
	];
	const { labels } = table([], { mode: "draw", strokes });
	assert.deepEqual(labels(), []);
});
test("a shape's distance goes the moment it lands", () => {
	const { controller, labels } = table([], CIRCLE_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(256, 0), at(0, 0), NONE);
	assert.equal(labels().length, 1, "no number while it is in hand");
	controller.tool.release(at(256, 0), at(0, 0), NONE);
	assert.deepEqual(labels(), [], "the number outlived the drag");
});
test("an abandoned shape's distance goes with it", () => {
	const { controller, labels } = table([], RECT_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(200, 200), at(0, 0), NONE);
	controller.tool.secondary(at(200, 200), at(0, 0));
	assert.deepEqual(labels(), []);
});
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
	const { controller, outlines, segments } = table([], CONE_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(0, 100), at(0, 0), NONE);
	assert.deepEqual(outlines(), [], "a cone took the ring pass");
	const sides = segments();
	assert.equal(sides.length, 3);
	assert.deepEqual(
		sides.map((s) => [s.x0, s.y0, s.x1, s.y1]),
		[[0, 0, -50, 100], [-50, 100, 50, 100], [50, 100, 0, 0]],
	);
});
test("the cone preview is the colour it will land in", () => {
	const { controller, chooseBrush, segments } = table([], CONE_ON);
	chooseBrush("#0000FF", 4);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(0, 100), at(0, 0), NONE);
	assert.deepEqual(segments()[0].color, [0, 0, 1]);
});
test("a cone that has not left its point previews nothing", () => {
	const { controller, outlines, segments } = table([], CONE_ON);
	controller.tool.press(at(50, 50), at(0, 0), NONE);
	controller.tool.drag(at(50, 50), at(0, 0), NONE);
	assert.deepEqual(segments(), []);
	assert.deepEqual(outlines(), []);
});
test("an abandoned cone takes its preview off the table", () => {
	const { controller, segments } = table([], CONE_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(0, 200), at(0, 0), NONE);
	controller.tool.secondary(at(0, 200), at(0, 0));
	assert.deepEqual(segments(), []);
});
test("a cone being dragged carries its length", () => {
	const { controller, labels } = table([], CONE_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(0, 384), at(0, 0), NONE);
	assert.deepEqual(labels().map((l) => l.text), ["30 ft."]);
});
test("the eraser draws a ring under the pointer", () => {
	const { controller, chooseDraw, outlines } = table([], { ...ERASE_ON, scale: 2 });
	controller.tool.hover(at(80, 90));
	const ring = outlines().at(-1);
	assert.equal(ring?.x, 80);
	assert.equal(ring?.y, 90);
	assert.equal(ring?.halfW, 12, "the reach is six CSS pixels at this zoom");
	assert.equal(ring?.rect, false);
	chooseDraw("pen");
	assert.deepEqual(outlines(), []);
});
test("the eraser's ring goes when the pointer leaves the table", () => {
	const { controller, outlines } = table([], ERASE_ON);
	controller.tool.hover(at(80, 90));
	controller.tool.hover(null);
	assert.deepEqual(outlines(), []);
});
test("the eraser's ring goes when another tool is chosen", () => {
	const { controller, choose, outlines } = table([], ERASE_ON);
	controller.tool.hover(at(80, 90));
	assert.equal(outlines().length, 1, "the ring was never drawn");
	choose("select");
	controller.tool.hover(at(80, 90));
	assert.deepEqual(outlines(), [], "the eraser kept its ring under another tool");
});
