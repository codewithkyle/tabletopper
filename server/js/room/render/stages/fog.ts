import type { Stage, StageFactory } from "./stage.ts";
import { createFogPass } from "../fog-pass.ts";
const PLAYER_FOG_ALPHA = 1;
const GM_FOG_ALPHA = 0.5;
export const fogStage: StageFactory = (gl): Stage => {
	const pass = createFogPass(gl);
	return {
		build(frame) {
			const viewed = frame.viewed;
			if (!viewed?.fogEnabled) {
				return;
			}
			pass.sync(frame.state.fog, viewed.id, viewed.map, viewed.fogPrefill, frame.state.table.grid.cellSize);
		},
		draw(frame) {
			if (!frame.viewed?.fogEnabled) {
				return;
			}
			pass.draw(frame, frame.role === "gm" ? GM_FOG_ALPHA : PLAYER_FOG_ALPHA);
		},
		dispose: () => pass.dispose(),
	};
};
