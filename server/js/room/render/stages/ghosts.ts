import type { Drawn } from "../pawn-pass.ts";
import type { Stage, StageFactory } from "./stage.ts";
import { createPawnPass } from "../pawn-pass.ts";
import { GHOST_ALPHA } from "../../pawns.ts";
export const ghostsStage: StageFactory = (gl, resources): Stage => {
	const pass = createPawnPass(gl, resources.pawnProgram);
	const ghosts: Drawn[] = [];
	return {
		build(frame) {
			pass.build(frame.overlay.ghosts(ghosts), frame.state.table.grid, resources.sprites, GHOST_ALPHA);
		},
		draw(frame) {
			pass.draw(frame);
		},
		dispose: () => pass.dispose(),
	};
};
