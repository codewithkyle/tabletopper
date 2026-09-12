import type { Event as RoomEvent, Role } from "../../protocol.ts";
import type { FrameContext } from "../frame-context.ts";
import type { Resources } from "../resources.ts";
import type { Stage } from "./stage.ts";
import type { TextureStats } from "../stats.ts";
import { addLoaderStats } from "../../gl/loader.ts";
import { createResources } from "../resources.ts";
import { noTextureStats } from "../stats.ts";
import { nameOf, stagesFor } from "./order.ts";
const SMOOTHING = 0.1;
export interface StageTiming {
	name: string;
	build: number;
	draw: number;
}
export interface StageList {
	resources(): Resources;
	timing(on: boolean): void;
	timings(): readonly StageTiming[];
	build(frame: FrameContext): void;
	draw(frame: FrameContext): void;
	settling(frame: FrameContext): boolean;
	event(event: RoomEvent): void;
	stress(count: number): number;
	showBlood(on: boolean): void;
	fetched(): number;
	textures(): TextureStats;
	level(): number;
	visible(): number;
	reset(): void;
	dispose(): void;
}
export function newStageList(
	gl: WebGL2RenderingContext, role: Role, invalidate: () => void,
): StageList {
	let held = createResources(gl, invalidate);
	let stages = stagesFor(role).map((make) => make(gl, held));
	let timings: StageTiming[] = stagesFor(role).map((make) => ({ name: nameOf(make), build: 0, draw: 0 }));
	let timed = false;
	function drop(): void {
		for (const stage of stages) {
			stage.dispose();
		}
		held.dispose();
	}
	function smooth(was: number, now: number): number {
		return was === 0 ? now : was * (1 - SMOOTHING) + now * SMOOTHING;
	}
	return {
		resources: () => held,
		timing(on) {
			timed = on;
			if (!on) {
				for (const timing of timings) {
					timing.build = 0;
					timing.draw = 0;
				}
			}
		},
		timings: () => timings,
		build(frame) {
			if (!timed) {
				for (const stage of stages) {
					stage.build?.(frame);
				}
				return;
			}
			for (let i = 0; i < stages.length; i++) {
				const started = performance.now();
				stages[i].build?.(frame);
				timings[i].build = smooth(timings[i].build, performance.now() - started);
			}
		},
		draw(frame) {
			if (!timed) {
				for (const stage of stages) {
					stage.draw(frame);
				}
				return;
			}
			for (let i = 0; i < stages.length; i++) {
				const started = performance.now();
				stages[i].draw(frame);
				timings[i].draw = smooth(timings[i].draw, performance.now() - started);
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
		textures() {
			const total = noTextureStats();
			for (const stage of stages) {
				const one = stage.textures?.();
				if (!one) {
					continue;
				}
				total.resident += one.resident;
				total.capacity += one.capacity;
				total.evictions += one.evictions;
				addLoaderStats(total.loader, one.loader);
			}
			return total;
		},
		level() {
			for (const stage of stages) {
				const one = stage.level?.();
				if (one !== undefined) {
					return one;
				}
			}
			return 0;
		},
		visible() {
			let total = 0;
			for (const stage of stages) {
				total += stage.visible?.() ?? 0;
			}
			return total;
		},
		reset() {
			held = createResources(gl, invalidate);
			stages = stagesFor(role).map((make) => make(gl, held));
			timings = stagesFor(role).map((make) => ({ name: nameOf(make), build: 0, draw: 0 }));
		},
		dispose: drop,
	};
}
