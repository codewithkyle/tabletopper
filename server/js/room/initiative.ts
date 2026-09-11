import Sortable from "sortablejs";
import { ROOM_INITIATIVE } from "../../public/js/events.js";
import { typing } from "./keys.ts";
import type { State } from "./protocol.ts";
export interface Turns {
	changed(): void;
	stop(): void;
}
const WARNING = 60;
const DANGER = 120;
const GRACE = 250;
const GHOST = "initiative-ghost";
const CHOSEN = "initiative-chosen";
export function turnTone(seconds: number): string {
	if (seconds >= DANGER) {
		return "danger";
	}
	if (seconds >= WARNING) {
		return "warning";
	}
	return "plain";
}
export function clockText(seconds: number): string {
	const whole = Math.max(0, Math.floor(seconds));
	const minutes = Math.floor(whole / 60);
	const rest = whole % 60;
	return `${minutes}:${rest < 10 ? "0" : ""}${rest}`;
}
export function mountTurns(mount: HTMLElement, state: State, now = () => performance.now()): Turns {
	let seen: string | null = null;
	let since: number | null = null;
	let ticker: ReturnType<typeof setInterval> | null = null;
	let sortable: Sortable | null = null;
	let dropped: number | null = null;
	function strip(): HTMLElement | null {
		const found = mount.querySelector("[data-turns]");
		return found instanceof HTMLElement ? found : null;
	}
	function halt(): void {
		if (ticker === null) {
			return;
		}
		clearInterval(ticker);
		ticker = null;
	}
	function write(): boolean {
		const found = mount.querySelector("[data-turn-timer]");
		if (!(found instanceof HTMLElement)) {
			halt();
			return false;
		}
		const seconds = since === null ? 0 : (now() - since) / 1000;
		found.textContent = clockText(seconds);
		found.setAttribute("data-turn-tone", turnTone(seconds));
		return true;
	}
	function reveal(): void {
		const root = strip();
		if (root?.hasAttribute("data-dragging")) {
			return;
		}
		const found = mount.querySelector("[data-turn-active]");
		if (found instanceof HTMLElement && typeof found.scrollIntoView === "function") {
			found.scrollIntoView({ inline: "nearest", block: "nearest", behavior: "smooth" });
		}
	}
	function run(): void {
		halt();
		if (write() && state.initiative.active !== null) {
			ticker = setInterval(write, 1000);
		}
	}
	function changed(): void {
		const active = state.initiative.active;
		if (active !== seen) {
			seen = active;
			since = active === null ? null : now();
		}
		run();
		reveal();
	}
	function order(box: HTMLElement, root: HTMLElement): void {
		const button = root.querySelector("[data-turn-order]");
		if (!(button instanceof HTMLElement)) {
			return;
		}
		const ids: string[] = [];
		for (const el of box.querySelectorAll("[data-entry]")) {
			const id = el.getAttribute("data-entry");
			if (id) {
				ids.push(id);
			}
		}
		button.setAttribute("hx-vals", JSON.stringify({ entries: ids.join(",") }));
		button.dispatchEvent(new MouseEvent("click", { bubbles: true }));
	}
	function remount(): void {
		sortable?.destroy();
		sortable = null;
		const root = strip();
		if (!root?.hasAttribute("data-reorder")) {
			return;
		}
		const found = root.querySelector("[data-entries]");
		if (!(found instanceof HTMLElement)) {
			return;
		}
		const box: HTMLElement = found;
		sortable = new Sortable(box, {
			draggable: "[data-entry]",
			animation: 150,
			ghostClass: GHOST,
			chosenClass: CHOSEN,
			onStart() {
				root.setAttribute("data-dragging", "");
			},
			onEnd(event) {
				root.removeAttribute("data-dragging");
				dropped = now();
				if (event.oldIndex === event.newIndex) {
					window.dispatchEvent(new CustomEvent(ROOM_INITIATIVE));
					return;
				}
				order(box, root);
			},
		});
	}
	function onClickCapture(event: Event): void {
		if (dropped === null || now() - dropped > GRACE) {
			return;
		}
		if (!(event.target instanceof Element) || !event.target.closest("[data-entry]")) {
			return;
		}
		event.preventDefault();
		event.stopPropagation();
	}
	function onKeyDown(event: KeyboardEvent): void {
		if (event.ctrlKey || event.metaKey || event.altKey || event.repeat) {
			return;
		}
		if (event.key.toLowerCase() !== "n" || typing(event.target)) {
			return;
		}
		const button = mount.querySelector("[data-turn-next]");
		if (button instanceof HTMLElement) {
			button.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		}
	}
	function onSwap(event: Event): void {
		if (!(event.target instanceof Element) || !event.target.hasAttribute("data-turns")) {
			return;
		}
		remount();
		run();
		reveal();
	}
	mount.addEventListener("click", onClickCapture, true);
	document.addEventListener("keydown", onKeyDown);
	document.addEventListener("htmx:after:swap", onSwap);
	remount();
	return {
		changed,
		stop() {
			halt();
			sortable?.destroy();
			sortable = null;
			mount.removeEventListener("click", onClickCapture, true);
			document.removeEventListener("keydown", onKeyDown);
			document.removeEventListener("htmx:after:swap", onSwap);
		},
	};
}
