import type { Stage, StageFactory } from "./stage.ts";
import { SELF_COLOR } from "../../model/color.ts";
import { createPathPass } from "../path-pass.ts";
const START_ALPHA = 0.22;
export const partyStartStage: StageFactory = (gl, resources): Stage => {
	const pass = createPathPass(gl, resources.atlas);
	return {
		build(frame) {
			pass.begin(frame.worldPerCssPixel);
			const start = frame.viewed?.partyStart;
			if (!start) {
				return;
			}
			const grid = frame.state.table.grid;
			const size = Math.max(1, grid.cellSize);
			pass.cell(start.x - size / 2, start.y - size / 2, size, grid.type, SELF_COLOR, START_ALPHA);
		},
		draw(frame) {
			pass.draw(frame);
		},
		dispose: () => pass.dispose(),
	};
};
