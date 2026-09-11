import type { Ruler } from "../../pawns.ts";
import type { Stage, StageFactory } from "./stage.ts";
import { cellCentre } from "../../model/grid.ts";
import { createPathPass } from "../path-pass.ts";
const CELL_ALPHA = 0.16;
export const floorMarksStage: StageFactory = (gl, resources): Stage => {
	const pass = createPathPass(gl, resources.atlas);
	const rulers: Ruler[] = [];
	return {
		build(frame) {
			pass.begin(frame.worldPerCssPixel);
			const grid = frame.state.table.grid;
			for (const ruler of frame.overlay.rulers(rulers)) {
				for (let i = 0; i < ruler.cells.length; i += 2) {
					const [cx, cy] = cellCentre(grid, ruler.cells[i], ruler.cells[i + 1]);
					pass.cell(cx - frame.cell / 2, cy - frame.cell / 2, frame.cell, ruler.color, CELL_ALPHA);
				}
			}
		},
		draw(frame) {
			pass.draw(frame);
		},
		dispose: () => pass.dispose(),
	};
};
