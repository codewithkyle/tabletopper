import type { Grid, Layer, Pawn, Role, State } from "../protocol.ts";
import { compareStack } from "../model/stack.ts";
import { concealed } from "../model/polygon.ts";
import { containsPoint } from "../model/shape.ts";
export function hitTest(
	pawns: readonly Pawn[],
	layerID: string,
	grid: Grid,
	x: number,
	y: number,
	hidden?: (pawn: Pawn) => boolean,
): Pawn | null {
	let best: Pawn | null = null;
	for (const p of pawns) {
		if (p.layerId !== layerID) {
			continue;
		}
		if (hidden?.(p)) {
			continue;
		}
		if (best && compareStack(p, best) < 0) {
			continue;
		}
		if (containsPoint(p, x, y, grid.cellSize)) {
			best = p;
		}
	}
	return best;
}
export function conceals(
	state: State, role: Role, user: string, viewed: () => string,
): (pawn: Pawn) => boolean {
	return (pawn) => concealed(pawn, state.fog, floorOf(state, viewed()), role, user);
}
function floorOf(state: State, id: string): Layer | null {
	for (const layer of state.table.layers) {
		if (layer.id === id) {
			return layer;
		}
	}
	return null;
}
