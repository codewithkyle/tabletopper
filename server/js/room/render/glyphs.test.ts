import assert from "node:assert/strict";
import { test } from "node:test";
import type { Grid } from "../protocol.ts";
import { GLYPHS } from "./glyphs.ts";
import { distanceLabel } from "../model/grid.ts";
function grid(type: Grid["type"], units: Grid["units"]): Grid {
	return {
		type,
		lines: "solid",
		cellSize: 64,
		offsetX: 0,
		offsetY: 0,
		color: "#000000FF",
		snap: "cells",
		feetPerCell: 5,
		units,
		diagonals: "equal",
	};
}
test("the atlas covers every character a distance label can emit", () => {
	const types: Grid["type"][] = ["square", "hexPointy", "hexFlat"];
	const units: Grid["units"][] = ["feet", "miles", "kilometres", "cells"];
	const held = new Set([...GLYPHS]);
	for (const type of types) {
		for (const unit of units) {
			for (const value of [0, 1, 7, 15, 120, 1005, 999999]) {
				const label = distanceLabel(value, grid(type, unit));
				for (const char of label) {
					assert.ok(held.has(char), `${type}/${unit} reads ${label}, and the atlas has no ${JSON.stringify(char)}`);
				}
			}
		}
	}
});
test("the atlas holds no character twice", () => {
	assert.equal(new Set([...GLYPHS]).size, GLYPHS.length, `GLYPHS repeats a character: ${GLYPHS}`);
});
