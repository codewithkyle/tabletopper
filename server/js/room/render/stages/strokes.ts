import type { Stage, StageFactory } from "./stage.ts";
import { createStrokePass } from "../stroke-pass.ts";
import { watching } from "../../model/revisions.ts";
export const strokesStage: StageFactory = (gl): Stage => {
	const pass = createStrokePass(gl);
	const inked = watching(["strokes"]);
	let layer = "";
	return {
		build(frame) {
			if (inked.changed(frame.revisions) || frame.viewedID !== layer) {
				layer = frame.viewedID;
				pass.sync(frame.state.strokes, frame.viewedID);
			}
			pass.live(frame.state.strokes, frame.viewedID, frame.overlay.inHand);
		},
		draw(frame) {
			pass.draw(frame);
		},
		dispose: () => pass.dispose(),
	};
};
