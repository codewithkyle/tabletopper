import type { AuraPass } from "../aura-pass.ts";
import type { FrameContext } from "../frame-context.ts";
import type { Pawn } from "../../protocol.ts";
import type { Stage, StageFactory } from "./stage.ts";
import { AURA_DISC, AURA_RECT, auraTurn, createAuraPass } from "../aura-pass.ts";
import { HIDDEN_ALPHA } from "../pawn-pass.ts";
import { actingPawnIds } from "../../model/stack.ts";
import { auraColor } from "../../model/color.ts";
import { healthOf } from "../../model/health.ts";
import { lifted } from "./lifted.ts";
import { pawnExtents } from "../../model/shape.ts";
export function fillAuras(
	pass: AuraPass, frame: FrameContext, wanted: (pawn: Pawn) => boolean,
): boolean {
	const acting = actingPawnIds(frame.state.initiative);
	if (acting.length === 0) {
		return false;
	}
	let glowing = false;
	for (const pawn of frame.state.pawns) {
		if (pawn.layerId !== frame.viewedID || !acting.includes(pawn.id)) {
			continue;
		}
		if (!wanted(pawn)) {
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
	return glowing;
}
export const aurasStage: StageFactory = (gl, resources): Stage => {
	const pass = createAuraPass(gl, resources.auraProgram);
	let glowing = false;
	let frame: FrameContext | null = null;
	const grounded = (pawn: Pawn): boolean => frame !== null && !lifted(frame, pawn);
	return {
		build(now) {
			frame = now;
			pass.begin();
			glowing = fillAuras(pass, now, grounded);
		},
		draw(now) {
			pass.draw(now, auraTurn(now.now));
		},
		settling: () => glowing,
		dispose: () => pass.dispose(),
	};
};
