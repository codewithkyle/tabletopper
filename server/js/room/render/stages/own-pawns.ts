import type { Drawn } from "../../model/overlay.ts";
import type { PawnPulse } from "../pawn-pass.ts";
import type { FrameContext } from "../frame-context.ts";
import type { Pawn } from "../../protocol.ts";
import type { Stage, StageFactory } from "./stage.ts";
import { lifted } from "./lifted.ts";
import { createPawnPass } from "../pawn-pass.ts";
import { fastBeat, slowBeat } from "../../model/health.ts";
import { visiblePawns } from "./pawns.ts";
export const ownPawnsStage: StageFactory = (gl, resources): Stage => {
	const pass = createPawnPass(gl, resources.pawnProgram);
	const pulse: PawnPulse = { slow: 0, heart: 0 };
	const drawn: Drawn[] = [];
	let frame: FrameContext | null = null;
	const elsewhere = (pawn: Pawn): boolean => {
		if (!frame) {
			return true;
		}
		return !lifted(frame, pawn);
	};
	return {
		build(now) {
			frame = now;
			if (!now.rebuild) {
				return;
			}
			visiblePawns(now.state.pawns, now.viewedID, drawn, elsewhere);
			pass.build(drawn, now.state.table.grid, resources.sprites);
		},
		draw(now) {
			pulse.slow = slowBeat(now.now);
			pulse.heart = fastBeat(now.now);
			pass.draw(now, pulse);
		},
		settling: () => pass.beating(),
		dispose: () => pass.dispose(),
	};
};
