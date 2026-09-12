import type { Stage, StageFactory } from "./stage.ts";
import { createPathPass } from "../path-pass.ts";
export const overMarksStage: StageFactory = (gl, resources): Stage => {
	const pass = createPathPass(gl, resources.atlas);
	return {
		build(frame) {
			pass.begin(frame.worldPerCssPixel);
			for (const segment of frame.overlay.segments) {
				pass.line(segment.x0, segment.y0, segment.x1, segment.y1, segment.width, segment.color, segment.alpha);
			}
			for (const label of frame.overlay.labels) {
				pass.label(label.text, label.x, label.y, label.color, label.alpha);
			}
		},
		draw(frame) {
			pass.draw(frame);
		},
		dispose: () => pass.dispose(),
	};
};
