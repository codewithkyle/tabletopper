import type { Stage, StageFactory } from "./stage.ts";
import { createPawnPass } from "../pawn-pass.ts";
const GHOST_ALPHA = 0.5;
export const ghostsStage: StageFactory = (gl, resources): Stage => {
	const pass = createPawnPass(gl, resources.pawnProgram);
	return {
		build(frame) {
			pass.build(frame.overlay.ghosts, frame.state.table.grid, resources.sprites, GHOST_ALPHA);
		},
		draw(frame) {
			pass.draw(frame);
		},
		dispose: () => pass.dispose(),
	};
};
