import type { WindowSpec } from "./window.ts";
export const PAWN_WINDOW = "pawn:";
const WIDTH = 320;
const HEIGHT = 520;
export interface Named {
	id: string;
	name: string;
}
export function pawnWindow(roomID: string, pawn: Named): WindowSpec {
	return {
		id: PAWN_WINDOW + pawn.id,
		url: `/fragment/room/pawn?room=${roomID}&pawn=${pawn.id}`,
		title: pawn.name,
		width: WIDTH,
		height: HEIGHT,
	};
}
