import type { Board } from "./board.ts";
import type { Drawn, Overlay, Pool } from "../model/overlay.ts";
import type { Pawn } from "../protocol.ts";
import type { Pens } from "./ruler.ts";
import type { Rgb } from "../model/types.ts";
import { ghostOf } from "../model/overlay.ts";
import { walkRuler } from "./ruler.ts";
const PREVIEW_TIMEOUT = 3000;
export interface Preview {
	positions: { id: string; x: number; y: number }[];
	color: Rgb;
	at: number;
}
export function remember(
	previews: Map<string, Preview>,
	by: string,
	pawns: readonly { id: string; x: number; y: number }[],
	color: Rgb,
	at: number,
): void {
	previews.set(by, {
		positions: pawns.map((pawn) => ({ id: pawn.id, x: pawn.x, y: pawn.y })),
		color,
		at,
	});
}
export function forgetPreviews(previews: Map<string, Preview>, touched: readonly string[]): void {
	for (const [by, preview] of previews) {
		if (preview.positions.some((at) => touched.includes(at.id))) {
			previews.delete(by);
		}
	}
}
export function expirePreviews(previews: Map<string, { at: number }>, now: number): boolean {
	let dropped = false;
	for (const [by, preview] of previews) {
		if (now - preview.at > PREVIEW_TIMEOUT) {
			previews.delete(by);
			dropped = true;
		}
	}
	return dropped;
}
export function showPreviews(
	out: Overlay,
	previews: Map<string, Preview>,
	board: Board,
	pens: Pens,
	ghosts: Pool<Drawn>,
	concealed: (pawn: Pawn) => boolean,
): void {
	for (const preview of previews.values()) {
		for (const at of preview.positions) {
			const p = board.pawn(at.id);
			if (p && board.onFloor(p) && !concealed(p)) {
				out.ghosts.push(ghostOf(p, at.x, at.y, ghosts.take()));
			}
		}
		const anchor = preview.positions[0];
		const p = anchor ? board.pawn(anchor.id) : null;
		if (anchor && p && board.onFloor(p) && !concealed(p)) {
			walkRuler(out, pens, board.grid(), p.x, p.y, anchor.x, anchor.y, preview.color);
		}
	}
}
