import type { WindowSpec } from "./window.ts";
export const NOTE_WINDOW = "hex:";
const WIDTH = 300;
const HEIGHT = 380;
export function noteWindowID(layer: string, q: number, r: number): string {
	return `${NOTE_WINDOW}${layer}:${q}:${r}`;
}
export function noteWindow(roomID: string, layer: string, q: number, r: number): WindowSpec {
	return {
		id: noteWindowID(layer, q, r),
		url: `/fragment/room/hex?room=${roomID}&layer=${layer}&q=${q}&r=${r}`,
		title: `Hex ${q}, ${r}`,
		width: WIDTH,
		height: HEIGHT,
	};
}
