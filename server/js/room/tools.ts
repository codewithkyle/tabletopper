import { typing } from "./keys.ts";
export type Mode = "select" | "pan" | "measure" | "fog" | "draw" | "ping";
export interface Tools {
	mode(): Mode;
	chosen(): Mode;
	onChange(fn: () => void): void;
	stop(): void;
}
const PAN_KEY = "Space";
const GRAB = "grab";
const MODES: readonly (readonly [string, Mode])[] = [
	["[data-room-tool-pans]", "pan"],
	["[data-room-tool-measures]", "measure"],
	["[data-room-tool-fogs]", "fog"],
	["[data-room-tool-draws]", "draw"],
	["[data-room-tool-pings]", "ping"],
];
export function showing<T>(chosen: T, pans: T | null, held: boolean): T {
	return held && pans !== null ? pans : chosen;
}
export function mountTools(mount: HTMLElement): Tools | null {
	const found = mount.querySelector("[data-room-tools]");
	if (!(found instanceof HTMLElement)) {
		return null;
	}
	const root: HTMLElement = found;
	const buttons = Array.from(root.querySelectorAll("[data-room-tool]"));
	const pans = root.querySelector("[data-room-tool-pans]");
	const modes = new Map<Element, Mode>();
	for (const [selector, mode] of MODES) {
		const button = root.querySelector(selector);
		if (button) {
			modes.set(button, mode);
		}
	}
	const keys = new Map<string, Element>();
	for (const button of buttons) {
		const key = button.getAttribute("data-room-tool-key");
		if (key) {
			keys.set(key.toLowerCase(), button);
		}
	}
	const canvas = mount.querySelector("[data-tabletop-canvas]");
	let chosen = buttons.find((b) => b.getAttribute("aria-pressed") === "true") ?? buttons[0] ?? null;
	let held = false;
	const changed: (() => void)[] = [];
	function lit(): Element | null {
		return showing(chosen, pans, held);
	}
	function modeOf(button: Element | null): Mode {
		return button ? modes.get(button) ?? "select" : "select";
	}
	function mode(): Mode {
		return modeOf(lit());
	}
	function paint(): void {
		const current = lit();
		for (const button of buttons) {
			button.setAttribute("aria-pressed", String(button === current));
		}
		if (canvas instanceof HTMLElement) {
			canvas.style.cursor = mode() === "pan" ? GRAB : "";
		}
		for (const fn of changed) {
			fn();
		}
	}
	function onClick(e: Event): void {
		if (!(e.target instanceof Element)) {
			return;
		}
		const button = e.target.closest("[data-room-tool]");
		if (!button) {
			return;
		}
		chosen = button;
		paint();
	}
	function onKeyDown(e: KeyboardEvent): void {
		if (typing(e.target)) {
			return;
		}
		if (e.code === PAN_KEY) {
			e.preventDefault();
			if (!held) {
				held = true;
				paint();
			}
			return;
		}
		if (e.ctrlKey || e.metaKey || e.altKey || e.repeat) {
			return;
		}
		const button = keys.get(e.key.toLowerCase());
		if (!button) {
			return;
		}
		chosen = button;
		paint();
	}
	function onKeyUp(e: KeyboardEvent): void {
		if (e.code !== PAN_KEY || !held) {
			return;
		}
		held = false;
		paint();
	}
	function onBlur(): void {
		if (!held) {
			return;
		}
		held = false;
		paint();
	}
	root.addEventListener("click", onClick);
	document.addEventListener("keydown", onKeyDown);
	document.addEventListener("keyup", onKeyUp);
	window.addEventListener("blur", onBlur);
	paint();
	return {
		mode,
		chosen: () => modeOf(chosen),
		onChange(fn) {
			changed.push(fn);
		},
		stop() {
			root.removeEventListener("click", onClick);
			document.removeEventListener("keydown", onKeyDown);
			document.removeEventListener("keyup", onKeyUp);
			window.removeEventListener("blur", onBlur);
		},
	};
}
