import type { Stage, StageFactory } from "./stage.ts";
import { AURA_DISC, AURA_RECT, auraTurn, createAuraPass } from "../aura-pass.ts";
import { HIDDEN_ALPHA } from "../pawn-pass.ts";
import { actingPawnIds } from "../../model/stack.ts";
import { auraColor } from "../../model/color.ts";
import { healthOf } from "../../model/health.ts";
import { pawnExtents } from "../../model/shape.ts";
export const aurasStage: StageFactory = (gl): Stage => {
	const pass = createAuraPass(gl);
	let glowing = false;
	return {
		build(frame) {
			glowing = false;
			pass.begin();
			const acting = actingPawnIds(frame.state.initiative);
			if (acting.length === 0) {
				return;
			}
			for (const pawn of frame.state.pawns) {
				if (pawn.layerId !== frame.viewedID || !acting.includes(pawn.id)) {
					continue;
				}
				const [halfW, halfH] = pawnExtents(pawn, frame.cell);
				pass.add(
					pawn.x, pawn.y, halfW, halfH,
					auraColor(healthOf(pawn)), pawn.visible ? 1 : HIDDEN_ALPHA,
					pawn.kind === "object" ? AURA_RECT : AURA_DISC,
					pawn.kind === "object" ? pawn.rotation : 0,
				);
				glowing = true;
			}
		},
		draw(frame) {
			pass.draw(frame, auraTurn(frame.now));
		},
		settling: () => glowing,
		dispose: () => pass.dispose(),
	};
};
