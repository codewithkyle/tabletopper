import type { Outline } from "../../pawns.ts";
import type { Stage, StageFactory } from "./stage.ts";
import { RING_ELLIPSE, RING_RECT, createRingPass } from "../ring-pass.ts";

import { CONDITION_COLORS } from "../../model/color.ts";
import { HIDDEN_ALPHA } from "../pawn-pass.ts";
import { pawnExtents } from "../../model/shape.ts";
export const CONDITION_RINGS_MAX = 16;
export const RING_GAP = 3;
export const RING_WIDTH = 2;
export function ringRadius(half: number, index: number, worldPerDevicePixel: number): number {
	return half + (RING_GAP + index * (RING_WIDTH + RING_GAP)) * worldPerDevicePixel;
}
export const ringsStage: StageFactory = (gl, resources): Stage => {
	const pass = createRingPass(gl, resources.ringProgram);
	const outlines: Outline[] = [];
	return {
		build(frame) {
			pass.begin();
			for (const pawn of frame.state.pawns) {
				if (pawn.layerId !== frame.viewedID || pawn.kind === "object" || pawn.conditions.length === 0) {
					continue;
				}
				const [halfW] = pawnExtents(pawn, frame.cell);
				const shown = Math.min(pawn.conditions.length, CONDITION_RINGS_MAX);
				const visible = pawn.visible ? 1 : HIDDEN_ALPHA;
				for (let i = 0; i < shown; i++) {
					const colour = CONDITION_COLORS[pawn.conditions[i].color] ?? CONDITION_COLORS.white;
					const radius = ringRadius(halfW, i, frame.worldPerDevicePixel);
					pass.add(pawn.x, pawn.y, radius, radius, colour, visible, RING_WIDTH, RING_ELLIPSE);
				}
			}
			for (const outline of frame.overlay.outlines(outlines)) {
				pass.add(
					outline.x, outline.y, outline.halfW, outline.halfH,
					outline.color, outline.alpha, outline.thickness,
					outline.rect ? RING_RECT : RING_ELLIPSE, outline.rotation,
				);
			}
		},
		draw(frame) {
			pass.draw(frame);
		},
		dispose: () => pass.dispose(),
	};
};
