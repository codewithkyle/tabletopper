import type { Grid, Pawn, Role, State } from "../protocol.ts";
import type { Outgoing } from "../socket.ts";
import type { Placed } from "../model/shape.ts";
import type { Rect } from "../model/types.ts";
import { boundsOf } from "../model/shape.ts";
export interface Board {
	state: State;
	role: Role;
	user: string;
	viewed(): string;
	grid(): Grid;
	pawn(id: string): Pawn | null;
	onFloor(pawn: Pawn): boolean;
	send(command: Outgoing): void;
	announce(): void;
}
export interface BoardDeps {
	state: State;
	role: Role;
	user: string;
	viewed: () => string;
	send: (command: Outgoing) => void;
	announce: () => void;
}
export function newBoard(deps: BoardDeps): Board {
	return {
		state: deps.state,
		role: deps.role,
		user: deps.user,
		viewed: deps.viewed,
		grid: () => deps.state.table.grid,
		pawn: (id) => deps.state.pawns.find((p) => p.id === id) ?? null,
		onFloor: (pawn) => pawn.layerId === deps.viewed(),
		send: deps.send,
		announce: deps.announce,
	};
}
export function boxAround(
	board: Board, ids: readonly string[], shaped: (pawn: Pawn) => Placed,
): Rect | null {
	const cell = board.grid().cellSize;
	let found: Rect | null = null;
	for (const id of ids) {
		const p = board.pawn(id);
		if (!p || !board.onFloor(p)) {
			continue;
		}
		const [halfW, halfH] = boundsOf(shaped(p), cell);
		if (!found) {
			found = { x1: p.x - halfW, y1: p.y - halfH, x2: p.x + halfW, y2: p.y + halfH };
			continue;
		}
		found.x1 = Math.min(found.x1, p.x - halfW);
		found.y1 = Math.min(found.y1, p.y - halfH);
		found.x2 = Math.max(found.x2, p.x + halfW);
		found.y2 = Math.max(found.y2, p.y + halfH);
	}
	return found;
}
