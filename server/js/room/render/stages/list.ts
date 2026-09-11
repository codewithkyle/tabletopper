import type { Event as RoomEvent, Role } from "../../protocol.ts";
import type { FrameContext } from "../frame-context.ts";
import type { Resources } from "../resources.ts";
import type { Stage } from "./stage.ts";
import { createResources } from "../resources.ts";
import { stagesFor } from "./order.ts";
export interface StageList {
	resources(): Resources;
	build(frame: FrameContext): void;
	draw(frame: FrameContext): void;
	settling(frame: FrameContext): boolean;
	event(event: RoomEvent): void;
	stress(count: number): number;
	showBlood(on: boolean): void;
	fetched(): number;
	reset(): void;
	dispose(): void;
}
export function newStageList(
	gl: WebGL2RenderingContext, role: Role, invalidate: () => void,
): StageList {
	let held = createResources(gl, invalidate);
	let stages = stagesFor(role).map((make) => make(gl, held));
	function drop(): void {
		for (const stage of stages) {
			stage.dispose();
		}
		held.dispose();
	}
	return {
		resources: () => held,
		build(frame) {
			for (const stage of stages) {
				stage.build?.(frame);
			}
		},
		draw(frame) {
			for (const stage of stages) {
				stage.draw(frame);
			}
		},
		settling(frame) {
			let again = false;
			for (const stage of stages) {
				if (stage.settling?.(frame)) {
					again = true;
				}
			}
			return again;
		},
		event(event) {
			for (const stage of stages) {
				stage.event?.(event);
			}
		},
		stress(count) {
			let added = 0;
			for (const stage of stages) {
				added += stage.stress?.(count) ?? 0;
			}
			return added;
		},
		showBlood(on) {
			for (const stage of stages) {
				stage.showBlood?.(on);
			}
		},
		fetched() {
			let total = 0;
			for (const stage of stages) {
				total += stage.fetched?.() ?? 0;
			}
			return total;
		},
		reset() {
			held = createResources(gl, invalidate);
			stages = stagesFor(role).map((make) => make(gl, held));
		},
		dispose: drop,
	};
}
