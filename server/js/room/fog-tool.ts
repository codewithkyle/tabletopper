import type { FogMode, ShapeKind } from "./protocol.ts";
import type { Tools } from "./tools.ts";
import type { FogOptions } from "./fog.ts";
export interface FogTool {
	options(): FogOptions;
	stop(): void;
}
const DEFAULT_SHAPE: ShapeKind = "rect";
const DEFAULT_MODE: FogMode = "reveal";
export function mountFogTool(mount: HTMLElement, tools: Tools | null): FogTool {
	const options: FogOptions = { shape: DEFAULT_SHAPE, mode: DEFAULT_MODE };
	const found = mount.querySelector("[data-fog-options]");
	if (!(found instanceof HTMLElement)) {
		return { options: () => options, stop() {} };
	}
	const root: HTMLElement = found;
	const groups = [
		{ attr: "data-fog-shape", key: "shape" },
		{ attr: "data-fog-mode", key: "mode" },
	] as const;
	function paint(): void {
		for (const group of groups) {
			for (const button of root.querySelectorAll("[" + group.attr + "]")) {
				const value = button.getAttribute(group.attr);
				button.setAttribute("aria-pressed", String(value === options[group.key]));
			}
		}
	}
	function onClick(e: Event): void {
		if (!(e.target instanceof Element)) {
			return;
		}
		for (const group of groups) {
			const button = e.target.closest("[" + group.attr + "]");
			const value = button?.getAttribute(group.attr);
			if (!value) {
				continue;
			}
			if (group.key === "shape") {
				options.shape = value as ShapeKind;
			} else {
				options.mode = value as FogMode;
			}
			paint();
			return;
		}
	}
	function follow(): void {
		root.hidden = !(tools?.fogging() ?? false);
	}
	root.addEventListener("click", onClick);
	tools?.onChange(follow);
	paint();
	follow();
	return {
		options: () => options,
		stop() {
			root.removeEventListener("click", onClick);
		},
	};
}
