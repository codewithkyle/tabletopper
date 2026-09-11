import type { FrameContext } from "../frame-context.ts";
import type { Pawn } from "../../protocol.ts";
import type { Stage, StageFactory } from "./stage.ts";
import { createRingPass } from "../ring-pass.ts";
import { fillConditionRings } from "./rings.ts";
import { lifted } from "./lifted.ts";
export const ownRingsStage: StageFactory = (gl, resources): Stage => {
	const pass = createRingPass(gl, resources.ringProgram);
	let frame: FrameContext | null = null;
	const raised = (pawn: Pawn): boolean => frame !== null && lifted(frame, pawn);
	return {
		build(now) {
			frame = now;
			pass.begin();
			fillConditionRings(pass, now, raised);
		},
		draw(now) {
			pass.draw(now);
		},
		dispose: () => pass.dispose(),
	};
};
