import type { RingTarget } from "../pings.ts";
import type { Stage, StageFactory } from "./stage.ts";
import { RING_ELLIPSE, createRingPass } from "../ring-pass.ts";
import { actorColor } from "../../model/color.ts";
import { newPings } from "../pings.ts";
export const pingsStage: StageFactory = (gl, resources): Stage => {
	const pass = createRingPass(gl, resources.ringProgram);
	const pings = newPings();
	const target: RingTarget = {
		ellipse(x, y, radius, color, alpha, thickness) {
			pass.add(x, y, radius, radius, color, alpha, thickness, RING_ELLIPSE);
		},
	};
	return {
		build(frame) {
			pass.begin();
			pings.build(frame.viewedID, frame.now, frame.cell, target);
		},
		draw(frame) {
			pass.draw(frame);
		},
		settling: (frame) => pings.settling(frame.now),
		event(event) {
			if (event.type === "pinged") {
				pings.add(event.layer, event.x, event.y, actorColor(event.by ?? ""), performance.now());
				resources.invalidate();
			}
		},
		dispose: () => pass.dispose(),
	};
};
