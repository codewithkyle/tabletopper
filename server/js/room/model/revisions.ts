import type { Event } from "../protocol.ts";
export interface Revisions {
	pawns: number;
	fog: number;
	strokes: number;
	table: number;
	initiative: number;
}
export function revisions(): Revisions {
	return { pawns: 0, fog: 0, strokes: 0, table: 0, initiative: 0 };
}
export function revise(rev: Revisions, event: Event): void {
	switch (event.type) {
		case "snapshot":
			rev.pawns++;
			rev.fog++;
			rev.strokes++;
			rev.table++;
			rev.initiative++;
			return;
		case "table.updated":
			rev.table++;
			return;
		case "initiative.updated":
			rev.initiative++;
			return;
		case "pawn.spawned":
		case "pawn.updated":
		case "pawn.removed":
		case "pawn.moved":
			rev.pawns++;
			return;
		case "fog.added":
		case "fog.removed":
			rev.fog++;
			return;
		case "stroke.began":
		case "stroke.extended":
		case "stroke.ended":
		case "stroke.erased":
			rev.strokes++;
			return;
		case "error":
		case "pinged":
		case "pawn.dragging":
		case "player.joined":
		case "player.updated":
		case "player.left":
		case "player.kicked":
		case "room.updated":
		case "room.closed":
			return;
		default: {
			const unrevised: never = event;
			throw new Error(`room: no revision for ${(unrevised as Event).type}`);
		}
	}
}
export interface Watch {
	changed(rev: Revisions): boolean;
	reset(): void;
}
export function watching(slices: readonly (keyof Revisions)[]): Watch {
	const seen = slices.map(() => -1);
	return {
		changed(rev) {
			let moved = false;
			for (let i = 0; i < slices.length; i++) {
				if (seen[i] !== rev[slices[i]]) {
					seen[i] = rev[slices[i]];
					moved = true;
				}
			}
			return moved;
		},
		reset() {
			seen.fill(-1);
		},
	};
}
