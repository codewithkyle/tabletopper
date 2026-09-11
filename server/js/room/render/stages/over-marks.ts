import type { Label, Ruler, Segment } from "../../pawns.ts";
import type { Stage, StageFactory } from "./stage.ts";
import { createPathPass } from "../path-pass.ts";
const RULER_WIDTH = 2;
const RULER_ALPHA = 0.9;
export const overMarksStage: StageFactory = (gl, resources): Stage => {
	const pass = createPathPass(gl, resources.atlas);
	const segments: Segment[] = [];
	const labels: Label[] = [];
	const rulers: Ruler[] = [];
	return {
		build(frame) {
			pass.begin(frame.worldPerCssPixel);
			for (const segment of frame.overlay.marks(segments)) {
				pass.line(segment.x0, segment.y0, segment.x1, segment.y1, segment.width, segment.color, segment.alpha);
			}
			for (const label of frame.overlay.labels(labels)) {
				pass.label(label.text, label.x, label.y, label.color, label.alpha);
			}
			for (const ruler of frame.overlay.rulers(rulers)) {
				pass.line(ruler.x0, ruler.y0, ruler.x1, ruler.y1, RULER_WIDTH, ruler.color, RULER_ALPHA);
				pass.label(ruler.label, (ruler.x0 + ruler.x1) / 2, (ruler.y0 + ruler.y1) / 2, ruler.color, 1);
			}
		},
		draw(frame) {
			pass.draw(frame);
		},
		dispose: () => pass.dispose(),
	};
};
