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
		case "layers.updated":
			rev.table++;
			return;
		case "initiative.updated":
			rev.initiative++;
			return;
		case "pawns.upserted":
		case "pawns.removed":
		case "pawns.moved":
			rev.pawns++;
			return;
		case "fog.upserted":
		case "fog.removed":
			rev.fog++;
			return;
		case "strokes.upserted":
		case "strokes.removed":
		case "strokes.extended":
		case "strokes.ended":
			rev.strokes++;
			return;
		case "rolls.upserted":
		case "rolls.removed":
		case "music.updated":
		case "rolled":
		case "error":
		case "pinged":
		case "pawn.dragging":
		case "players.upserted":
		case "players.removed":
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
