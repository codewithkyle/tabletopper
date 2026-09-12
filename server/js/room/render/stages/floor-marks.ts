import type { Stage, StageFactory } from "./stage.ts";
import { createPathPass } from "../path-pass.ts";
export const floorMarksStage: StageFactory = (gl, resources): Stage => {
	const pass = createPathPass(gl, resources.atlas);
	return {
		build(frame) {
			pass.begin(frame.worldPerCssPixel);
			for (const cell of frame.overlay.cells) {
				pass.cell(cell.x, cell.y, cell.size, cell.color, cell.alpha);
			}
		},
		draw(frame) {
			pass.draw(frame);
		},
		dispose: () => pass.dispose(),
	};
};
