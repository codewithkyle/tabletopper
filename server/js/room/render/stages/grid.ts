import type { Stage, StageFactory } from "./stage.ts";
import { createGridPass } from "../grid-pass.ts";
export const gridStage: StageFactory = (gl): Stage => {
	const pass = createGridPass(gl);
	return {
		draw(frame) {
			pass.draw(frame, frame.state.table.grid);
		},
		dispose: () => pass.dispose(),
	};
};
