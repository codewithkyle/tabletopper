import type { Event, State } from "./protocol.ts";
import type { Rect } from "./render/camera.ts";
import { actingPawnIds } from "./render/scene.ts";
import { boundsOf } from "./render/path.ts";
export interface Follow {
	event(event: Event): void;
	following(on: boolean): void;
}
export interface FollowOptions {
	viewed(): string;
	focus(rect: Rect): void;
}
export function mountFollow(state: State, options: FollowOptions): Follow {
	let seen: string | null = null;
	let on = true;
	function event(e: Event): void {
		if (e.type === "snapshot") {
			seen = state.initiative.active;
			return;
		}
		if (e.type !== "initiative.updated") {
			return;
		}
		const active = state.initiative.active;
		if (active === seen) {
			return;
		}
		seen = active;
		if (!on) {
			return;
		}
		const box = actingBounds(state, options.viewed());
		if (box) {
			options.focus(box);
		}
	}
	function following(next: boolean): void {
		on = next;
	}
	return { event, following };
}
export function actingBounds(state: State, viewed: string): Rect | null {
	const acting = actingPawnIds(state.initiative);
	if (acting.length === 0) {
		return null;
	}
	const cell = state.table.grid.cellSize;
	let box: Rect | null = null;
	for (const pawn of state.pawns) {
		if (pawn.layerId !== viewed || !acting.includes(pawn.id)) {
			continue;
		}
		const [halfW, halfH] = boundsOf(pawn, cell);
		if (!box) {
			box = { x1: pawn.x - halfW, y1: pawn.y - halfH, x2: pawn.x + halfW, y2: pawn.y + halfH };
			continue;
		}
		box.x1 = Math.min(box.x1, pawn.x - halfW);
		box.y1 = Math.min(box.y1, pawn.y - halfH);
		box.x2 = Math.max(box.x2, pawn.x + halfW);
		box.y2 = Math.max(box.y2, pawn.y + halfH);
	}
	return box;
}
