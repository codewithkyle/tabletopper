import type { HexColorPicker } from "vanilla-colorful/hex-color-picker.js";
import type { DrawMode, DrawOptions } from "./draw.ts";
import type { Tools } from "./tools.ts";
import { DEFAULT_WIDTH } from "./draw.ts";
import { typing } from "./keys.ts";

export interface DrawTool {
	options(): DrawOptions;
	stop(): void;
}

const DEFAULT_MODE: DrawMode = "pen";
type Panel = "color" | "width";

export function mountDrawTool(mount: HTMLElement, tools: Tools | null, color: string): DrawTool {
	const options: DrawOptions = { mode: DEFAULT_MODE, color, width: DEFAULT_WIDTH };

	const found = mount.querySelector("[data-draw-options]");
	if (!(found instanceof HTMLElement)) {
		return { options: () => options, stop() {} };
	}

	const root: HTMLElement = found;
	const swatch = root.querySelector("[data-draw-swatch]");
	const picker = root.querySelector("[data-draw-picker]");
	const slider = root.querySelector("[data-draw-width]");
	const reading = root.querySelector("[data-draw-width-value]");

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

	function onKeyDown(e: KeyboardEvent): void {
		if (e.key === "Escape" && open !== null && !typing(e.target)) {
			show(null);
		}
	}

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

	if (picker) {
		(picker as HexColorPicker).color = options.color;
	}

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
