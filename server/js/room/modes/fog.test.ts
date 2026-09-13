import assert from "node:assert/strict";
import { test } from "node:test";
import type { FogShape, Grid } from "../protocol.ts";
import { snapCorner } from "./fog.ts";
import { coveredBy, insideShape, maskRect, rectTriangles, triangulate } from "../model/polygon.ts";
import { NONE, at, pawn, press, table } from "./testing.ts";
import { hexCorners } from "../model/hex.ts";
const GROUND = "01LAYERGROUND";
const CELLAR = "01LAYERCELLAR";
function grid(over: Partial<Grid> = {}): Grid {
	return {
		type: "square",
		lines: "solid",
		cellSize: 64,
		offsetX: 0,
		offsetY: 0,
		color: "#000000FF",
		snap: "cells",
		feetPerCell: 5,
		units: "feet",
		diagonals: "equal",
		...over,
	};
}
function shape(over: Partial<FogShape> = {}): FogShape {
	return {
		id: "01SHAPE",
		layerId: GROUND,
		kind: "rect",
		mode: "reveal",
		points: [0, 0, 100, 100],
		...over,
	};
}
function area(triangles: readonly number[]): number {
	let total = 0;
	for (let i = 0; i + 5 < triangles.length; i += 6) {
		total += Math.abs(
			(triangles[i + 2] - triangles[i]) * (triangles[i + 5] - triangles[i + 1])
			- (triangles[i + 3] - triangles[i + 1]) * (triangles[i + 4] - triangles[i]),
		) / 2;
	}
	return total;
}
test("a rectangle is two triangles with its corners normalised", () => {
	const out: number[] = [];
	rectTriangles([100, 80, 20, 10], out);
	assert.equal(out.length, 12, "a rectangle is six vertices");
	assert.equal(area(out), 80 * 70);
	assert.equal(Math.min(...out.filter((_, i) => i % 2 === 0)), 20);
	assert.equal(Math.max(...out.filter((_, i) => i % 2 === 0)), 100);
});
test("a convex polygon triangulates to its own area", () => {
	const out: number[] = [];
	triangulate([0, 0, 100, 0, 100, 100, 0, 100], out);
	assert.equal(out.length, 12, "a quadrilateral is two triangles");
	assert.equal(area(out), 100 * 100);
});
test("a concave polygon triangulates without filling its notch", () => {
	const out: number[] = [];
	triangulate([0, 0, 100, 0, 100, 50, 50, 50, 50, 100, 0, 100], out);
	assert.equal(out.length, 4 * 6, "six corners are four triangles");
	assert.equal(area(out), 100 * 100 - 50 * 50);
});
test("a polygon triangulates the same wound either way round", () => {
	const clockwise: number[] = [];
	const other: number[] = [];
	triangulate([0, 0, 100, 0, 100, 50, 50, 50, 50, 100, 0, 100], clockwise);
	triangulate([0, 100, 50, 100, 50, 50, 100, 50, 100, 0, 0, 0], other);
	assert.equal(area(clockwise), area(other));
});
test("a self-crossing polygon still terminates", () => {
	const out: number[] = [];
	triangulate([0, 0, 100, 100, 100, 0, 0, 100], out);
	assert.ok(out.length > 0, "a bow tie produced nothing at all");
});
test("fewer than three corners is no polygon", () => {
	const out: number[] = [];
	assert.equal(triangulate([0, 0, 100, 100], out).length, 0);
});
test("a point is inside a rectangle whichever way its corners were given", () => {
	const backwards = shape({ points: [100, 100, 0, 0] });
	assert.equal(insideShape(backwards, 50, 50), true);
	assert.equal(insideShape(backwards, 150, 50), false);
});
test("a point is inside a polygon by its crossings and not by its bounds", () => {
	const ell = shape({ kind: "poly", points: [0, 0, 100, 0, 100, 50, 50, 50, 50, 100, 0, 100] });
	assert.equal(insideShape(ell, 25, 25), true);
	assert.equal(insideShape(ell, 75, 75), false, "the notch reads as inside");
});
test("the last shape over a point is the one that decides", () => {
	const reveal = shape({ id: "a", points: [0, 0, 100, 100] });
	const hide = shape({ id: "b", mode: "hide", points: [40, 40, 60, 60] });
	assert.equal(coveredBy([], GROUND, true, 50, 50), true, "an empty covered floor is covered");
	assert.equal(coveredBy([], GROUND, false, 50, 50), false, "an empty clear floor is clear");
	assert.equal(coveredBy([reveal], GROUND, true, 50, 50), false);
	assert.equal(coveredBy([reveal, hide], GROUND, true, 50, 50), true, "the hide did not close it back up");
	assert.equal(coveredBy([hide, reveal], GROUND, true, 50, 50), false);
});
test("a shape on another floor is not this floor's business", () => {
	const upstairs = shape({ layerId: CELLAR, points: [0, 0, 100, 100] });
	assert.equal(coveredBy([upstairs], GROUND, true, 50, 50), true);
});
test("a corner snaps to the grid's vertices", () => {
	assert.deepEqual(snapCorner(grid(), 70, 70, false), [64, 64]);
	assert.deepEqual(snapCorner(grid(), 100, 100, false), [128, 128]);
});
test("a corner snaps at half a cell when the grid does", () => {
	assert.deepEqual(snapCorner(grid({ snap: "halfCells" }), 70, 70, false), [64, 64]);
	assert.deepEqual(snapCorner(grid({ snap: "halfCells" }), 90, 90, false), [96, 96]);
});
test("a corner snaps to the grid's own offset", () => {
	assert.deepEqual(snapCorner(grid({ offsetX: 10, offsetY: 10 }), 70, 70, false), [74, 74]);
});
test("Alt and a grid that does not snap both leave a corner where it fell", () => {
	assert.deepEqual(snapCorner(grid(), 70.4, 71.6, true), [70, 72]);
	assert.deepEqual(snapCorner(grid({ snap: "off" }), 70.4, 71.6, false), [70, 72]);
});
test("a corner is a whole number under every snapping mode", () => {
	for (const snap of ["cells", "halfCells", "off"] as const) {
		for (const alt of [false, true]) {
			const [x, y] = snapCorner(grid({ snap, cellSize: 65, offsetX: 3 }), 70.4, 71.6, alt);
			assert.ok(Number.isInteger(x), `${snap} alt=${alt} answered x=${x}`);
			assert.ok(Number.isInteger(y), `${snap} alt=${alt} answered y=${y}`);
		}
	}
});
test("the mask covers the map and the shapes together", () => {
	const out = { x: 0, y: 0, width: 0, height: 0 };
	const outside = shape({ points: [1200, -400, 1400, -200] });
	const both = maskRect({ width: 1000, height: 800 }, [outside], GROUND, 64, out);
	assert.ok(both);
	assert.equal(both.x, 0 - 64);
	assert.equal(both.x + both.width, 1400 + 64, "the shape beyond the map did not widen the mask");
	assert.equal(both.y, -400 - 64, "the shape above the map did not raise the mask");
	assert.equal(both.y + both.height, 800 + 64);
});
test("a floor with no map is sized by its shapes alone", () => {
	const out = { x: 0, y: 0, width: 0, height: 0 };
	const only = maskRect(null, [shape({ points: [0, 0, 100, 100] })], GROUND, 10, out);
	assert.ok(only);
	assert.equal(only.x, -10);
	assert.equal(only.width, 120);
});
test("a floor with neither has no mask at all", () => {
	const out = { x: 0, y: 0, width: 0, height: 0 };
	assert.equal(maskRect(null, [], GROUND, 64, out), null);
	assert.equal(maskRect(null, [shape({ layerId: CELLAR })], GROUND, 64, out), null);
});
const FOG_ON = { mode: "fog", fogEnabled: true, fogPrefill: true } as const;
function covering(): FogShape {
	return shape({ id: "01CLEARED", layerId: GROUND, kind: "rect", points: [0, 0, 128, 128] });
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
	const mine = covering();
	const upstairs = { ...covering(), id: "01UPSTAIRS", layerId: CELLAR };
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
test("a fog rectangle is previewed while it is dragged", () => {
	const { controller, outlines } = table([], FOG_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(200, 130), at(0, 0), NONE);
	const [box, ...rest] = outlines();
	assert.equal(rest.length, 0, "something else is on the table as well");
	assert.ok(box, "the rectangle in hand is not drawn");
	assert.equal(box.rect, true);
	assert.deepEqual([box.x, box.y, box.halfW, box.halfH], [96, 64, 96, 64]);
});
test("a fog rectangle that has not left its first vertex is not drawn", () => {
	const { controller, outlines } = table([], FOG_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(10, 10), at(0, 0), NONE);
	assert.deepEqual(outlines(), []);
});
test("the fog rectangle goes when the gesture does", () => {
	const { controller, outlines } = table([], FOG_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(200, 130), at(0, 0), NONE);
	controller.tool.secondary(at(200, 130), at(0, 0));
	assert.deepEqual(outlines(), [], "an abandoned rectangle is still on the table");
});
test("the fog rectangle is coloured by the mode it will send", () => {
	const { controller, chooseFog, outlines } = table([], FOG_ON);
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.drag(at(200, 130), at(0, 0), NONE);
	const uncover = outlines()[0].color;
	chooseFog("rect", "hide");
	const cover = outlines()[0].color;
	assert.notDeepEqual(uncover, cover);
});
test("a polygon in hand runs a line back to the pointer", () => {
	const { controller, segments } = table([], { ...FOG_ON, shape: "poly" });
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.press(at(200, 0), at(0, 0), NONE);
	controller.tool.hover(at(200, 200));
	assert.ok(segments().length > 0, "the fog's polygon lost its lines");
});
test("leaving the fog tool drops the polygon it was holding", () => {
	const { controller, choose, segments } = table([], { ...FOG_ON, shape: "poly" });
	controller.tool.press(at(0, 0), at(0, 0), NONE);
	controller.tool.press(at(200, 0), at(0, 0), NONE);
	controller.tool.release(at(200, 0), at(0, 0), NONE);
	controller.tool.hover(at(200, 200));
	choose("select");
	assert.deepEqual(segments(), [], "a half-drawn polygon outlived the tool that drew it");
});
const CELLS_ON = { ...FOG_ON, fogCells: true } as const;
function fogAdd(kind: string, points: number[]): Record<string, unknown> {
	return { type: "fog.add", layer: GROUND, kind, mode: "reveal", points };
}
test("the cell brush paints one square per cell it crosses", () => {
	const { controller, sent } = table([], CELLS_ON);
	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.drag(at(96, 32), at(0, 0), NONE);
	controller.tool.drag(at(160, 32), at(0, 0), NONE);
	controller.tool.release(at(160, 32), at(0, 0), NONE);
	assert.deepEqual(sent, [
		fogAdd("rect", [0, 0, 64, 64]),
		fogAdd("rect", [64, 0, 128, 64]),
		fogAdd("rect", [128, 0, 192, 64]),
	]);
});
test("the cell brush does not paint one loose rectangle across the drag", () => {
	const { controller, sent } = table([], CELLS_ON);
	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.drag(at(160, 160), at(0, 0), NONE);
	controller.tool.release(at(160, 160), at(0, 0), NONE);
	assert.ok(sent.length > 1, "the whole drag came back as one shape");
	for (const command of sent) {
		assert.equal((command.points as number[]).length, 4);
	}
});
test("dragging back over a cell it painted does not send it twice", () => {
	const { controller, sent } = table([], CELLS_ON);
	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.drag(at(96, 32), at(0, 0), NONE);
	controller.tool.drag(at(32, 32), at(0, 0), NONE);
	controller.tool.release(at(32, 32), at(0, 0), NONE);
	assert.deepEqual(sent, [
		fogAdd("rect", [0, 0, 64, 64]),
		fogAdd("rect", [64, 0, 128, 64]),
	]);
});
test("a second gesture paints a cell the first one already had", () => {
	const { controller, sent } = table([], CELLS_ON);
	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.release(at(32, 32), at(0, 0), NONE);
	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.release(at(32, 32), at(0, 0), NONE);
	assert.equal(sent.length, 2);
});
test("on a hex grid the cell brush paints hexagons", () => {
	const hex = grid({ type: "hexPointy" });
	const { controller, sent } = table([], { ...CELLS_ON, grid: hex });
	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.release(at(32, 32), at(0, 0), NONE);
	assert.deepEqual(sent, [fogAdd("poly", hexCorners(hex, 0, 0, []))]);
	assert.equal((sent[0].points as number[]).length, 12);
});
test("the cell brush covers as well as it uncovers", () => {
	const { controller, sent } = table([], { ...CELLS_ON, fogMode: "hide" });
	controller.tool.press(at(32, 32), at(0, 0), NONE);
	controller.tool.release(at(32, 32), at(0, 0), NONE);
	assert.equal(sent[0].mode, "hide");
});
test("the cell brush outlines the cell it would paint, not the point under the pointer", () => {
	const t = table([], CELLS_ON);
	t.controller.tool.hover(at(100, 40));
	const [marked] = t.cells();
	assert.ok(marked, "nothing is marked under the pointer");
	assert.deepEqual([marked.x, marked.y, marked.size], [64, 0, 64]);
	assert.equal(marked.type, "square");
});
test("the cell brush's outline goes when another tool is chosen", () => {
	const t = table([], CELLS_ON);
	t.controller.tool.hover(at(100, 40));
	assert.equal(t.cells().length, 1, "nothing is outlined while the fog tool is up");
	t.choose("select");
	t.controller.tool.hover(at(100, 40));
	assert.equal(t.cells().length, 0, "the brush outlived the fog tool");
	t.choose("fog");
	t.controller.tool.hover(at(100, 40));
	assert.equal(t.cells().length, 1, "the brush did not come back with the fog tool");
});
test("the outlined cell is the one the brush sends", () => {
	for (const type of ["square", "hexPointy", "hexFlat"] as const) {
		const t = table([], { ...CELLS_ON, grid: grid({ type }) });
		t.controller.tool.hover(at(100, 40));
		const [marked] = t.cells();
		t.controller.tool.press(at(100, 40), at(0, 0), NONE);
		t.controller.tool.release(at(100, 40), at(0, 0), NONE);
		const points = t.sent[0].points as number[];
		let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
		for (let i = 0; i < points.length; i += 2) {
			minX = Math.min(minX, points[i]);
			minY = Math.min(minY, points[i + 1]);
			maxX = Math.max(maxX, points[i]);
			maxY = Math.max(maxY, points[i + 1]);
		}
		const centre = [(minX + maxX) / 2, (minY + maxY) / 2];
		const outlined = [marked.x + marked.size / 2, marked.y + marked.size / 2];
		assert.ok(
			Math.abs(centre[0] - outlined[0]) <= 1 && Math.abs(centre[1] - outlined[1]) <= 1,
			`on a ${type} grid the outline sits at ${outlined} and the shape at ${centre}`,
		);
	}
});
