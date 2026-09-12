import type { Stage, StageFactory } from "./stage.ts";
import { createStrokePass } from "../stroke-pass.ts";
export const strokesStage: StageFactory = (gl): Stage => {
	const pass = createStrokePass(gl);
	return {
		build(frame) {
			pass.sync(frame.state.strokes, frame.viewedID);
			pass.live(frame.state.strokes, frame.viewedID, frame.overlay.inHand);
		},
		draw(frame) {
			pass.draw(frame);
		},
		dispose: () => pass.dispose(),
	};
};
