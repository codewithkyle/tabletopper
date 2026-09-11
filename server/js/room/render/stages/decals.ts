import { ROOM_BLOOD } from "../../../../public/js/events.js";
import type { Stage, StageFactory } from "./stage.ts";
import { createDecalPass } from "../decal-pass.ts";
import { newDecals } from "../decals.ts";
export const decalsStage: StageFactory = (gl, resources): Stage => {
	const pass = createDecalPass(gl);
	const decals = newDecals();
	const wipe = (): void => {
		decals.wipe();
		resources.invalidate();
	};
	window.addEventListener(ROOM_BLOOD, wipe);
	return {
		build(frame) {
			if (frame.rebuild) {
				decals.watch(frame.state.pawns, frame.viewedID, frame.cell, frame.now);
			}
			decals.build(frame.viewedID, frame.now, resources.sprites, pass);
		},
		draw(frame) {
			pass.draw(frame, resources.sprites.texture());
		},
		settling: (frame) => decals.settling(frame.now),
		event(event) {
			if (event.type === "stroke.cleared") {
				decals.clear(event.layer);
				resources.invalidate();
			}
			if (event.type === "snapshot") {
				decals.resync();
			}
		},
		showBlood(on) {
			decals.show(on);
			resources.invalidate();
		},
		dispose() {
			window.removeEventListener(ROOM_BLOOD, wipe);
			pass.dispose();
		},
	};
};
