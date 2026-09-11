import type { Stage, StageFactory } from "./stage.ts";
import { createTilePass } from "../tile-pass.ts";
export const tilesStage: StageFactory = (gl, resources): Stage => {
	let uploading = false;
	const pass = createTilePass(gl, resources.invalidate);
	return {
		draw(frame) {
			pass.begin();
			for (const layer of frame.painted) {
				pass.draw(frame, layer.map, layer.alpha);
			}
			uploading = pass.end();
		},
		settling: () => uploading,
		fetched: () => pass.fetched(),
		dispose: () => pass.dispose(),
	};
};
