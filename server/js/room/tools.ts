



































































import { typing } from "./keys.ts";

export interface Tools {
	
	
	
	panning(): boolean;

	
	
	
	
	measuring(): boolean;

	
	
	
	
	fogging(): boolean;

	
	
	
	drawing(): boolean;

	
	
	
	
	pinging(): boolean;

	
	
	
	
	
	
	
	
	
	
	onChange(fn: () => void): void;

	stop(): void;
}




const PAN_KEY = "Space";





const GRAB = "grab";





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
	const measures = root.querySelector("[data-room-tool-measures]");
	const fogs = root.querySelector("[data-room-tool-fogs]");
	const draws = root.querySelector("[data-room-tool-draws]");
	const pings = root.querySelector("[data-room-tool-pings]");

	
	
	
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

	function panning(): boolean {
		return pans !== null && lit() === pans;
	}

	function measuring(): boolean {
		return measures !== null && chosen === measures;
	}

	function fogging(): boolean {
		return fogs !== null && chosen === fogs;
	}

	function drawing(): boolean {
		return draws !== null && chosen === draws;
	}

	function pinging(): boolean {
		return pings !== null && chosen === pings;
	}

	function paint(): void {
		const current = lit();

		for (const button of buttons) {
			button.setAttribute("aria-pressed", String(button === current));
		}

		if (canvas instanceof HTMLElement) {
			canvas.style.cursor = panning() ? GRAB : "";
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
		panning,
		measuring,
		fogging,
		drawing,
		pinging,

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
