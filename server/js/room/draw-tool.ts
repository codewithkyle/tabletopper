// The drawing tool's second pill: what a gesture on the table does, what colour
// it does it in, and how wide.
//
// IT IS THE TOOL'S STATE AND NOT THE ROOM'S, which is fog-tool.ts's decision
// and holds here for the same reason: nothing in it is sent anywhere or stored
// anywhere -- the mode, the colour and the width are read when a gesture starts
// and written into what goes out -- so a viewer who reloads gets the defaults
// back. That is also why it is a pill rather than a window: a window is a
// refetching fragment and would reset the choice every time the table updated
// underneath it.
//
// IT IS HIDDEN WHILE ANOTHER TOOL IS CHOSEN, on tools.onChange.
//
// IT IS EVERY ROLE'S, unlike the fog's. A player's page renders this pill and a
// player may draw unless the GM has said otherwise, which is the core's refusal
// rather than a missing control.
//
// THE COLOUR AND THE WIDTH ARE FOLDED AND OPEN BESIDE THE PILL. Both are panels
// too big to leave on screen over the corner of a map for a setting somebody
// changes every few minutes, and both open to the LEFT because that is where
// the pill's own tooltips already open. Where they open is the TEMPLATE'S to
// say -- an absolutely positioned panel anchored to its button's row -- and not
// this file's: nothing here measures anything or writes a position.
//
// ONE PANEL AT A TIME. Opening one closes the other; so does choosing a mode,
// Escape, a press anywhere outside the pill, and the tool going away. Two open
// panels beside a pill would overlap, because both are anchored to the same
// edge.
//
// NO CLASS NAME IS WRITTEN IN THIS FILE, for the reason fog-tool.ts writes none:
// server/js is not a Tailwind source, so a class named here would never be
// emitted. What it writes is aria-pressed, aria-expanded, [hidden], one inline
// background colour and one text node.

import type { HexColorPicker } from "vanilla-colorful/hex-color-picker.js";
import type { DrawMode, DrawOptions } from "./draw.ts";
import type { Tools } from "./tools.ts";
import { DEFAULT_WIDTH } from "./draw.ts";
import { typing } from "./keys.ts";

export interface DrawTool {
	options(): DrawOptions;
	stop(): void;
}

// The default mode, and it is the markup's too: room.templ renders the first
// button of the group pressed. A pen is what somebody who picks up the tool
// meant to pick up.
const DEFAULT_MODE: DrawMode = "pen";

// The panels, named by what their button carries. A third would be a third
// value here and nothing else.
type Panel = "color" | "width";

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
	const swatch = root.querySelector("[data-draw-swatch]");
	const picker = root.querySelector("[data-draw-picker]");
	const slider = root.querySelector("[data-draw-width]");
	const reading = root.querySelector("[data-draw-width-value]");

	// open is which panel is showing, or null. It is a single value rather than
	// a flag per panel because that is the rule: one at a time.
	let open: Panel | null = null;

	function paintMode(): void {
		for (const button of root.querySelectorAll("[data-draw-mode]")) {
			button.setAttribute("aria-pressed", String(button.getAttribute("data-draw-mode") === options.mode));
		}
	}

	function paintPanels(): void {
		for (const panel of root.querySelectorAll("[data-draw-popout]")) {
			if (panel instanceof HTMLElement) {
				panel.hidden = panel.getAttribute("data-draw-popout") !== open;
			}
		}

		for (const button of root.querySelectorAll("[data-draw-open]")) {
			button.setAttribute("aria-expanded", String(button.getAttribute("data-draw-open") === open));
		}
	}

	// THE CHIP IS PAINTED INLINE AND OVER A WHITE BACKING the template puts
	// there. A style property rather than a class, for the reason in the header;
	// the first paint is this call, because the colour is derived from the
	// viewer's own id and the server has never heard of it.
	function paintColor(): void {
		if (swatch instanceof HTMLElement) {
			swatch.style.backgroundColor = options.color;
		}
	}

	function paintWidth(): void {
		if (reading instanceof HTMLElement) {
			reading.textContent = String(options.width);
		}
	}

	function show(panel: Panel | null): void {
		open = panel;
		paintPanels();
	}

	function onClick(e: Event): void {
		if (!(e.target instanceof Element)) {
			return;
		}

		const mode = e.target.closest("[data-draw-mode]")?.getAttribute("data-draw-mode");
		if (mode) {
			// THE VALUE IS TAKEN AS WRITTEN, which is fog-tool.ts's rule: the
			// closed set is validated in Go, which is where a closed set is
			// validated, and a value this page did not render cannot get here
			// without a devtools console that could have sent the command
			// directly.
			options.mode = mode as DrawMode;
			paintMode();
			show(null);

			return;
		}

		const panel = e.target.closest("[data-draw-open]")?.getAttribute("data-draw-open");
		if (panel === "color" || panel === "width") {
			show(open === panel ? null : panel);
		}
	}

	// A drag on the picker. The value arrives as #rrggbb -- there is no alpha on
	// this element, which is the point of choosing it -- and comes back
	// lowercase, so it is upper-cased to match what hexColor produces for the
	// default. The server accepts either; what differs is the two spellings a
	// person would see in devtools for the same colour.
	function onPicked(e: Event): void {
		if (!(e.target instanceof Element) || !e.target.matches("[data-draw-picker]")) {
			return;
		}

		const detail = (e as CustomEvent<{ value?: unknown }>).detail;
		if (typeof detail?.value !== "string" || detail.value === "") {
			return;
		}

		options.color = detail.value.toUpperCase();
		paintColor();
	}

	// AND THE SLIDER IS READ LIVE RATHER THAN ON CHANGE, so the number beside it
	// tracks the thumb. Nothing is sent and nothing is saved, so there is no
	// reason to wait for the drag to settle the way the grid's colour does.
	function onInput(e: Event): void {
		if (!(e.target instanceof HTMLInputElement) || !e.target.matches("[data-draw-width]")) {
			return;
		}

		const next = Number.parseInt(e.target.value, 10);
		if (!Number.isFinite(next)) {
			return;
		}

		options.width = next;
		paintWidth();
	}

	// ESCAPE CLOSES A PANEL AND IS NOT TAKEN FROM THE PAGE. pawns.ts hears the
	// same key and puts away whatever is in hand; nothing is in hand while
	// somebody is picking a colour, so both running is both doing nothing to
	// each other.
	function onKeyDown(e: KeyboardEvent): void {
		if (e.key === "Escape" && open !== null && !typing(e.target)) {
			show(null);
		}
	}

	// A PRESS ANYWHERE ELSE CLOSES THEM, which is what the canvas needs and is
	// not written in terms of the canvas: a press on the map, on the menu bar or
	// on a window all mean the same thing, which is that this pill is no longer
	// what the hand is doing.
	//
	// IT IS pointerdown AND NOT click, because the table acts on a press: a
	// click listener would close the panel after the stroke it did not want had
	// already begun.
	function onPointerDown(e: Event): void {
		if (open === null) {
			return;
		}
		if (e.target instanceof Node && root.contains(e.target)) {
			return;
		}

		show(null);
	}

	function follow(): void {
		const drawing = tools?.drawing() ?? false;
		root.hidden = !drawing;

		// A PILL THAT HAS GONE AWAY TAKES ITS PANELS WITH IT. Without this, a
		// panel left open when the tool changed would come back open the next
		// time somebody chose Draw, over a map they were looking at.
		if (!drawing && open !== null) {
			show(null);
		}
	}

	root.addEventListener("click", onClick);
	root.addEventListener("color-changed", onPicked);
	root.addEventListener("input", onInput);
	document.addEventListener("keydown", onKeyDown);
	document.addEventListener("pointerdown", onPointerDown);
	tools?.onChange(follow);

	// The picker opens on the viewer's own colour. The property and not the
	// attribute, which is what color.ts writes and is safe for the same reason:
	// main.ts imports the element's module at the top of the bundle, so the
	// custom element is defined and this one has upgraded long before anything
	// here runs. Assigning it never fires color-changed -- only a pointer move
	// does -- so there is no loop to break.
	if (picker) {
		(picker as HexColorPicker).color = options.color;
	}

	// THE WIDTH IS READ OUT OF THE MARKUP RATHER THAN WRITTEN INTO IT, which is
	// the one place this file differs from fog-tool.ts and the reason is that it
	// can: a mode is a button that is either pressed or not, and a width is a
	// number the template already renders. Reading it means the slider and the
	// pen cannot open on two different values -- there is only one value, and it
	// is room.go's. DEFAULT_WIDTH is what answers for a page with no pill at
	// all.
	if (slider instanceof HTMLInputElement) {
		const rendered = Number.parseInt(slider.value, 10);
		if (Number.isFinite(rendered)) {
			options.width = rendered;
		}
	}

	paintMode();
	paintColor();
	paintWidth();
	paintPanels();
	follow();

	return {
		options: () => options,

		stop() {
			root.removeEventListener("click", onClick);
			root.removeEventListener("color-changed", onPicked);
			root.removeEventListener("input", onInput);
			document.removeEventListener("keydown", onKeyDown);
			document.removeEventListener("pointerdown", onPointerDown);
		},
	};
}
