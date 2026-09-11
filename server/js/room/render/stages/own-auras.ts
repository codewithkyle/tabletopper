import type { FrameContext } from "../frame-context.ts";
import type { Pawn } from "../../protocol.ts";
import type { Stage, StageFactory } from "./stage.ts";
import { auraTurn, createAuraPass } from "../aura-pass.ts";
import { fillAuras } from "./auras.ts";
import { lifted } from "./lifted.ts";
export const ownAurasStage: StageFactory = (gl, resources): Stage => {
	const pass = createAuraPass(gl, resources.auraProgram);
	let glowing = false;
	let frame: FrameContext | null = null;
	const raised = (pawn: Pawn): boolean => frame !== null && lifted(frame, pawn);
	return {
		build(now) {
			frame = now;
			pass.begin();
			glowing = fillAuras(pass, now, raised);
		},
		draw(now) {
			pass.draw(now, auraTurn(now.now));
		},
		settling: () => glowing,
		dispose: () => pass.dispose(),
	};
};
