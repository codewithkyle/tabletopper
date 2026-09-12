import type { Mode } from "../tools.ts";
import type { Place } from "./place.ts";
import type { Select } from "./select.ts";
import type { Tool } from "../render/input.ts";
import { typing } from "../keys.ts";
export interface SwitchDeps {
	mode: () => Mode;
	chosen: () => Mode;
	remove: () => void;
	select: Select;
	place: Place;
	pan: Tool;
	measure: Tool;
	ping: Tool;
	fog: Tool;
	draw: Tool;
}
export interface ToolSwitch extends Tool {
	stop(): void;
}
export function newToolSwitch(deps: SwitchDeps): ToolSwitch {
	const byMode: Record<Mode, Tool> = {
		select: deps.select,
		pan: deps.pan,
		measure: deps.measure,
		fog: deps.fog,
		draw: deps.draw,
		ping: deps.ping,
	};
	const every: readonly Tool[] = [
		deps.select, deps.measure, deps.fog, deps.draw, deps.ping, deps.pan, deps.place,
	];
	let seated: Mode = deps.chosen();
	let held: Tool | null = null;
	byMode[seated].enter?.();
	function routed(): Tool {
		const on = deps.mode();
		return on !== "pan" && deps.place.active() ? deps.place : byMode[on];
	}
	function settle(): void {
		if (held !== null) {
			return;
		}
		const next = deps.chosen();
		if (next === seated) {
			return;
		}
		const from = byMode[seated];
		from.abandon();
		from.leave?.();
		seated = next;
		byMode[seated].enter?.();
	}
	function current(): Tool {
		settle();
		return held ?? routed();
	}
	function onKeyDown(e: KeyboardEvent): void {
		if (typing(e.target)) {
			return;
		}
		const tool = current();
		if (tool.key(e)) {
			return;
		}
		if (e.key === "Escape") {
			tool.abandon();
			return;
		}
		if (e.key === "Delete" && deps.select.selection.size > 0) {
			deps.remove();
		}
	}
	document.addEventListener("keydown", onKeyDown);
	return {
		press(map, screen, mods) {
			held = null;
			settle();
			held = routed();
			return held.press(map, screen, mods);
		},
		drag(map, screen, mods) {
			current().drag(map, screen, mods);
		},
		release(map, screen, mods) {
			const tool = current();
			held = null;
			tool.release(map, screen, mods);
		},
		cancel() {
			const tool = current();
			held = null;
			tool.cancel();
		},
		secondary(map, screen) {
			const tool = current();
			if (tool.secondary(map, screen)) {
				return true;
			}
			return tool !== deps.select && deps.select.secondary(map, screen);
		},
		hover(map) {
			settle();
			for (const tool of every) {
				tool.hover(map);
			}
		},
		key: (e) => current().key(e),
		abandon: () => current().abandon(),
		active: () => every.some((tool) => tool.active()),
		contribute(out) {
			settle();
			for (const tool of every) {
				tool.contribute(out);
			}
		},
		stop() {
			document.removeEventListener("keydown", onKeyDown);
		},
	};
}
