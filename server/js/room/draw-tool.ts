// The drawing tool's second pill: what a gesture on the table does.
//
// IT IS THE TOOL'S STATE AND NOT THE ROOM'S, which is fog-tool.ts's decision
// and holds here for the same reason: nothing in it is sent anywhere or stored
// anywhere -- the mode is read when a gesture starts and written into what goes
// out -- so a viewer who reloads gets the defaults back. That is also why it is
// a pill rather than a window: a window is a refetching fragment and would
// reset the choice every time the table updated underneath it.
//
// IT IS HIDDEN WHILE ANOTHER TOOL IS CHOSEN, on tools.onChange, which is the one
// thing it subscribes to.
//
// IT IS EVERY ROLE'S, unlike the fog's. A player's page renders this pill and a
// player may draw unless the GM has said otherwise, which is the core's refusal
// rather than a missing control.
//
// NO CLASS NAME IS WRITTEN IN THIS FILE, for the reason fog-tool.ts writes none:
// server/js is not a Tailwind source, so a class named here would never be
// emitted. What it writes is aria-pressed and [hidden], both of which room.templ
// styles.

import type { DrawMode, DrawOptions } from "./draw.ts";
import type { Tools } from "./tools.ts";
import { DEFAULT_WIDTH } from "./draw.ts";

export interface DrawTool {
	options(): DrawOptions;
	stop(): void;
}

// The defaults, and they are the markup's too: room.templ renders the first
// button of the group pressed. A pen is what somebody who picks up the tool
// meant to pick up.
const DEFAULT_MODE: DrawMode = "pen";

export function mountDrawTool(mount: HTMLElement, tools: Tools | null, color: string): DrawTool {
	const options: DrawOptions = { mode: DEFAULT_MODE, color, width: DEFAULT_WIDTH };

	const found = mount.querySelector("[data-draw-options]");
	if (!(found instanceof HTMLElement)) {
		// A closed room renders no pill, and the tool that would read this has
		// no button to be chosen either. The defaults answer for a table where
		// nothing can ask.
		return { options: () => options, stop() {} };
	}

	const root: HTMLElement = found;

	function paint(): void {
		for (const button of root.querySelectorAll("[data-draw-mode]")) {
			button.setAttribute("aria-pressed", String(button.getAttribute("data-draw-mode") === options.mode));
		}
	}

	function onClick(e: Event): void {
		if (!(e.target instanceof Element)) {
			return;
		}

		const button = e.target.closest("[data-draw-mode]");
		const value = button?.getAttribute("data-draw-mode");
		if (!value) {
			return;
		}

		// THE VALUE IS TAKEN AS WRITTEN, which is fog-tool.ts's rule: the closed
		// set is validated in Go, which is where a closed set is validated, and
		// a value this page did not render cannot get here without a devtools
		// console that could have sent the command directly.
		options.mode = value as DrawMode;
		paint();
	}

	function follow(): void {
		root.hidden = !(tools?.drawing() ?? false);
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
