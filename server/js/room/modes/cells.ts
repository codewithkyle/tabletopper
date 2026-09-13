import type { Grid } from "../protocol.ts";
import { cellAt, cellPath } from "../model/grid.ts";
export interface CellWalker {
	enter(grid: Grid, x: number, y: number): number[];
	reset(): void;
}
export function newCellWalker(): CellWalker {
	let lastQ = 0;
	let lastR = 0;
	let started = false;
	const seen = new Set<string>();
	const path: number[] = [];
	const out: number[] = [];
	function take(q: number, r: number): void {
		const key = q + "," + r;
		if (seen.has(key)) {
			return;
		}
		seen.add(key);
		out.push(q, r);
	}
	return {
		enter(grid, x, y) {
			out.length = 0;
			const [q, r] = cellAt(grid, x, y);
			if (!started) {
				started = true;
				lastQ = q;
				lastR = r;
				take(q, r);
				return out;
			}
			if (q === lastQ && r === lastR) {
				return out;
			}
			cellPath(grid, lastQ, lastR, q, r, path);
			for (let i = 0; i < path.length; i += 2) {
				take(path[i], path[i + 1]);
			}
			lastQ = q;
			lastR = r;
			return out;
		},
		reset() {
			started = false;
			seen.clear();
			out.length = 0;
		},
	};
}
