// The fog tool's second pill: what a gesture draws, and which way round it
// works.
//
// IT IS THE TOOL'S STATE AND NOT THE ROOM'S. Nothing here is sent anywhere or
// stored anywhere: the shape and the mode are read when a gesture FINISHES and
// written into fog.add, and a GM who reloads gets the pill's defaults back. That
// is why it is a pill rather than a window -- a window is a refetching fragment
// and would reset a choice every time the table updated underneath it.
//
// IT IS HIDDEN WHILE ANOTHER TOOL IS CHOSEN, on tools.onChange, which is the one
// thing it subscribes to. A permanent second pill would be four controls that do
// nothing on a table nobody is fogging.
//
// NO CLASS NAME IS WRITTEN IN THIS FILE, for the reason tools.ts writes none:
// server/js is not a Tailwind source, so a class named here would never be
// emitted. What it writes is aria-pressed and [hidden], both of which room.templ
// styles.

import type { FogMode, ShapeKind } from "./protocol.ts";
import type { Tools } from "./tools.ts";
import type { FogOptions } from "./fog.ts";

export interface FogTool {
	options(): FogOptions;
	stop(): void;
}

// The defaults, and they are the markup's too: room.templ renders the first
// button of each group pressed. A rectangle that uncovers is what a GM reaches
// for nine times in ten.
const DEFAULT_SHAPE: ShapeKind = "rect";
const DEFAULT_MODE: FogMode = "reveal";

export function mountFogTool(mount: HTMLElement, tools: Tools | null): FogTool {
	const options: FogOptions = { shape: DEFAULT_SHAPE, mode: DEFAULT_MODE };

	const found = mount.querySelector("[data-fog-options]");
	if (!(found instanceof HTMLElement)) {
		// A player's page has no pill, and the tool that would read this has no
		// button to be chosen either. The defaults answer for a table where
		// nothing can ask.
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

			// THE VALUE IS THE PROTOCOL'S OWN and it is taken as written. The
			// closed set is validated in Go, which is where a closed set is
			// validated; a browser is not where rules live, and a value this
			// page did not render cannot get here without a devtools console
			// that could have sent the command directly.
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
