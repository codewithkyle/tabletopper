import assert from "node:assert/strict";
import { test } from "node:test";
import type { Grid, MapRef } from "../protocol.ts";
import { axialOf, cellName, newNumbering, numberingFor, offsetOf } from "./numbering.ts";
import { cellCentre } from "./grid.ts";
function newGrid(over: Partial<Grid> = {}): Grid {
	return {
		type: "square", lines: "solid", cellSize: 64, offsetX: 0, offsetY: 0,
		color: "#000000FF", snap: "cells", feetPerCell: 5, units: "feet",
		diagonals: "equal", numbered: true, ...over,
	};
}
function newMap(width: number, height: number): MapRef {
	return { assetId: "01BX5ZZKBKACTAV9WEVGEMMVRZ", gen: "1", width, height, tileSize: 256, maxZoom: 1 };
}
const TYPES: Grid["type"][] = ["square", "hexPointy", "hexFlat"];
function numbered(grid: Grid, width: number, height: number): { q: number; r: number; n: number }[] {
	const found: { q: number; r: number; n: number }[] = [];
	const numbering = newNumbering(grid, width, height);
	for (let q = -40; q <= 40; q++) {
		for (let r = -40; r <= 40; r++) {
			const n = numbering.of(q, r);
			if (n !== null) {
				found.push({ q, r, n });
			}
		}
	}
	return found;
}
test("a square map of three cells by two is numbered down each column in turn", () => {
	const numbering = newNumbering(newGrid(), 192, 128);
	assert.equal(numbering.total(), 6);
	assert.deepEqual(
		[[0, 0], [0, 1], [1, 0], [1, 1], [2, 0], [2, 1]].map(([q, r]) => numbering.of(q, r)),
		[1, 2, 3, 4, 5, 6],
	);
});
test("a cell that is not over the map is not numbered, so counting starts and stops with the picture", () => {
	const numbering = newNumbering(newGrid(), 192, 128);
	for (const [q, r] of [[-1, 0], [3, 0], [0, 2], [0, -1], [99, 99]]) {
		assert.equal(numbering.of(q, r), null, `${q}, ${r} is over nothing and still got a number`);
	}
});
test("every numbered cell sits over the map and every cell over the map is numbered", () => {
	for (const type of TYPES) {
		const grid = newGrid({ type, offsetX: -17, offsetY: 9 });
		const found = numbered(grid, 500, 400);
		assert.ok(found.length > 20, `${type}: a 500 by 400 map holds more than twenty cells`);
		for (const { q, r } of found) {
			const [x, y] = cellCentre(grid, q, r);
			assert.ok(x >= 0 && x <= 500 && y >= 0 && y <= 400, `${type}: ${q}, ${r} is numbered off the map`);
		}
		const numbering = newNumbering(grid, 500, 400);
		for (let q = -40; q <= 40; q++) {
			for (let r = -40; r <= 40; r++) {
				const [x, y] = cellCentre(grid, q, r);
				if (x >= 0 && x <= 500 && y >= 0 && y <= 400) {
					assert.notEqual(numbering.of(q, r), null, `${type}: ${q}, ${r} is over the map and was passed over`);
				}
			}
		}
		const numbers = found.map((cell) => cell.n).sort((a, b) => a - b);
		assert.deepEqual(numbers, numbers.map((_, i) => i + 1), `${type}: the numbers are not 1 upwards with no gaps`);
	}
});
function placed(grid: Grid, width: number, height: number): { n: number; x: number; y: number }[] {
	return numbered(grid, width, height)
		.map((cell) => {
			const [x, y] = cellCentre(grid, cell.q, cell.r);
			return { n: cell.n, x, y };
		})
		.sort((a, b) => a.n - b.n);
}
test("the count runs down a column before it moves to the next one", () => {
	for (const type of TYPES) {
		const grid = newGrid({ type });
		const cells = placed(grid, 400, 300);
		const stagger = grid.cellSize / 2 + 1;
		for (let i = 1; i < cells.length; i++) {
			const was = cells[i - 1];
			const now = cells[i];
			const down = now.y > was.y && Math.abs(now.x - was.x) <= stagger;
			const over = now.y < was.y && now.x >= was.x - stagger;
			assert.ok(down || over, `${type}: ${was.n} to ${now.n} is neither a step down nor the top of the next column`);
		}
	}
});
test("a column is a strip of the map, and each one is to the right of the last", () => {
	for (const type of TYPES) {
		const grid = newGrid({ type });
		const cells = placed(grid, 400, 300);
		const columns: { n: number; x: number; y: number }[][] = [[]];
		for (const cell of cells) {
			const column = columns[columns.length - 1];
			if (column.length > 0 && cell.y < column[column.length - 1].y) {
				columns.push([]);
			}
			columns[columns.length - 1].push(cell);
		}
		assert.ok(columns.length > 3, `${type}: a 400 wide map holds more than three columns`);
		let last = -Infinity;
		for (const column of columns) {
			const middle = column.reduce((sum, cell) => sum + cell.x, 0) / column.length;
			for (const cell of column) {
				assert.ok(
					Math.abs(cell.x - middle) <= grid.cellSize / 2 + 1,
					`${type}: cell ${cell.n} is not in the column it was counted in`,
				);
			}
			assert.ok(middle > last, `${type}: a column is not to the right of the one before it`);
			last = middle;
		}
	}
});
test("cell one is the top left cell of the map", () => {
	for (const type of TYPES) {
		const grid = newGrid({ type });
		const placed = numbered(grid, 400, 300).map((cell) => {
			const [x, y] = cellCentre(grid, cell.q, cell.r);
			return { ...cell, x, y };
		});
		const first = placed.find((cell) => cell.n === 1);
		assert.ok(first, `${type}: nothing is numbered one`);
		for (const cell of placed) {
			assert.ok(cell.x >= first.x - 1, `${type}: ${cell.n} is left of cell one`);
		}
	}
});
test("offset and axial coordinates are the same cell said two ways", () => {
	for (const type of TYPES) {
		const grid = newGrid({ type });
		for (let q = -8; q <= 8; q++) {
			for (let r = -8; r <= 8; r++) {
				const [col, row] = offsetOf(grid, q, r);
				assert.deepEqual(axialOf(grid, col, row), [q, r], `${type}: ${q}, ${r} did not survive the round trip`);
			}
		}
	}
});
test("walking a rectangle visits the numbered cells inside it and no others", () => {
	for (const type of TYPES) {
		const grid = newGrid({ type });
		const numbering = newNumbering(grid, 400, 300);
		const seen: number[] = [];
		numbering.each(100, 100, 200, 200, (q, r, n) => {
			const [x, y] = cellCentre(grid, q, r);
			assert.equal(numbering.of(q, r), n, `${type}: ${q}, ${r} was visited with the wrong number`);
			const reach = grid.cellSize * 2;
			assert.ok(
				x >= 100 - reach && x <= 200 + reach && y >= 100 - reach && y <= 200 + reach,
				`${type}: ${q}, ${r} is more than two cells outside the rectangle`,
			);
			seen.push(n);
		});
		assert.ok(seen.length > 0, `${type}: nothing was visited`);
		assert.equal(new Set(seen).size, seen.length, `${type}: a cell was visited twice`);
	}
});
test("a hex is named by its number only when the table is numbered and there is a map under it", () => {
	const grid = newGrid();
	const map = newMap(192, 128);
	assert.equal(cellName(grid, map, 1, 1), "4");
	assert.equal(cellName(grid, map, 9, 9), "9, 9", "a hex off the map keeps its coordinates");
	assert.equal(cellName(newGrid({ numbered: false }), map, 1, 1), "1, 1");
	assert.equal(cellName(grid, null, 1, 1), "1, 1", "a floor with no map has nothing to count over");
});
test("numbering is only built for a table that asked for it", () => {
	const map = newMap(192, 128);
	assert.equal(numberingFor(newGrid({ numbered: false }), map), null);
	assert.equal(numberingFor(newGrid(), null), null);
	assert.ok(numberingFor(newGrid(), map));
});
test("the same grid and map hand back the same numbering rather than building it again", () => {
	const map = newMap(192, 128);
	assert.equal(numberingFor(newGrid(), map), numberingFor(newGrid(), map));
	assert.notEqual(numberingFor(newGrid(), map), numberingFor(newGrid({ cellSize: 32 }), map));
});
