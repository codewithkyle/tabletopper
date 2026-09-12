import type { Stage, StageFactory } from "./stage.ts";
import { RING_ELLIPSE, RING_RECT, createRingPass } from "../ring-pass.ts";
import { SELECT_COLOR } from "../../model/color.ts";
const HANDLE_WIDTH = 2;
const HANDLE_HALF = 4;
export const handlesStage: StageFactory = (gl, resources): Stage => {
	const pass = createRingPass(gl, resources.ringProgram);
	return {
		build(frame) {
			pass.begin();
			const half = HANDLE_HALF * frame.worldPerCssPixel;
			for (const handle of frame.overlay.handles) {
				pass.add(
					handle.x, handle.y, half, half,
					SELECT_COLOR, 1, HANDLE_WIDTH,
					handle.turns ? RING_ELLIPSE : RING_RECT, handle.rotation,
				);
			}
		},
		draw(frame) {
			pass.draw(frame);
		},
		dispose: () => pass.dispose(),
	};
};
