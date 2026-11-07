import type { Named } from "./pawn-window.ts";
import type { Point } from "./model/types.ts";
import { nextZ } from "./window.ts";
import { onLongPress } from "./long-press.ts";
export interface EntryMenuDeps {
	details: (pawn: Named) => void;
}
export interface EntryMenu {
	close(): void;
	stop(): void;
}
const EDGE = 8;
export function mountEntryMenu(mount: HTMLElement, deps: EntryMenuDeps): EntryMenu | null {
	const found = mount.querySelector("[data-entry-menu]");
	if (!(found instanceof HTMLElement)) {
		return null;
	}
	const root: HTMLElement = found;
	const heading = root.querySelector("[data-entry-menu-name]");
	const details = root.querySelector("[data-entry-menu-details]");
	let target: HTMLElement | null = null;
	const origin: Point = { x: 0, y: 0 };
	function named(entry: HTMLElement): Named | null {
		const id = entry.getAttribute("data-entry-solo");
		if (!id) {
			return null;
		}
		const label = entry.querySelector("[data-entry-name]");
		return { id, name: label?.textContent?.trim() ?? "" };
	}
	function open(entry: HTMLElement, screen: Point): void {
		target = entry;
		const pawn = named(entry);
		if (heading) {
			const label = entry.querySelector("[data-entry-name]");
			heading.textContent = label?.textContent?.trim() ?? "";
		}
		if (details instanceof HTMLElement) {
			details.hidden = pawn === null;
		}
		root.hidden = false;
		root.style.zIndex = String(nextZ());
		origin.x = screen.x;
		origin.y = screen.y;
		place();
	}
	function close(): void {
		if (root.hidden) {
			return;
		}
		target = null;
		root.hidden = true;
	}
	function place(): void {
		const width = root.offsetWidth;
		const height = root.offsetHeight;
		const right = origin.x + width + EDGE > mount.clientWidth;
		const below = origin.y + height + EDGE > mount.clientHeight;
		const x = clamp(right ? origin.x - width : origin.x, mount.clientWidth - width - EDGE);
		const y = clamp(below ? origin.y - height : origin.y, mount.clientHeight - height - EDGE);
		root.style.transform = `translate(${Math.round(x)}px, ${Math.round(y)}px)`;
	}
	function entryAt(target: EventTarget | null): HTMLElement | null {
		if (!(target instanceof Element)) {
			return null;
		}
		const entry = target.closest("[data-entry]");
		return entry instanceof HTMLElement ? entry : null;
	}
	function openAt(entry: HTMLElement, clientX: number, clientY: number): void {
		const box = mount.getBoundingClientRect();
		open(entry, { x: clientX - box.left, y: clientY - box.top });
	}
	function onContextMenu(event: MouseEvent): void {
		const entry = entryAt(event.target);
		if (!entry) {
			return;
		}
		event.preventDefault();
		if (!pressing.touching()) {
			openAt(entry, event.clientX, event.clientY);
		}
	}
	function onDoubleClick(event: MouseEvent): void {
		if (!(event.target instanceof Element)) {
			return;
		}
		const entry = event.target.closest("[data-entry]");
		if (!(entry instanceof HTMLElement)) {
			return;
		}
		const pawn = named(entry);
		if (pawn) {
			deps.details(pawn);
		}
	}
	function onClick(event: Event): void {
		if (!(event.target instanceof Element)) {
			return;
		}
		const entry = target;
		if (event.target.closest("[data-entry-menu-details]")) {
			close();
			const pawn = entry ? named(entry) : null;
			if (pawn) {
				deps.details(pawn);
			}
			return;
		}
		if (event.target.closest("[data-entry-menu-remove]")) {
			close();
			const button = entry?.querySelector("[data-entry-remove]");
			if (button instanceof HTMLElement) {
				button.dispatchEvent(new MouseEvent("click", { bubbles: true }));
			}
		}
	}
	function onPointerDown(event: Event): void {
		if (root.hidden || (event.target instanceof Node && root.contains(event.target))) {
			return;
		}
		close();
	}
	function onWheel(): void {
		close();
	}
	function onKeyDown(event: KeyboardEvent): void {
		if (event.key === "Escape") {
			close();
		}
	}
	function onSwap(event: Event): void {
		if (event.target instanceof Element && event.target.hasAttribute("data-turns")) {
			close();
		}
	}
	const pressing = onLongPress(mount, (target, x, y) => {
		const entry = entryAt(target);
		if (!entry) {
			return false;
		}
		openAt(entry, x, y);
		return true;
	});
	mount.addEventListener("contextmenu", onContextMenu);
	mount.addEventListener("dblclick", onDoubleClick);
	root.addEventListener("click", onClick);
	document.addEventListener("pointerdown", onPointerDown);
	document.addEventListener("wheel", onWheel);
	document.addEventListener("keydown", onKeyDown);
	document.addEventListener("htmx:after:swap", onSwap);
	return {
		close,
		stop() {
			close();
			pressing.stop();
			mount.removeEventListener("contextmenu", onContextMenu);
			mount.removeEventListener("dblclick", onDoubleClick);
			root.removeEventListener("click", onClick);
			document.removeEventListener("pointerdown", onPointerDown);
			document.removeEventListener("wheel", onWheel);
			document.removeEventListener("keydown", onKeyDown);
			document.removeEventListener("htmx:after:swap", onSwap);
		},
	};
}
function clamp(value: number, high: number): number {
	return Math.max(EDGE, Math.min(value, Math.max(EDGE, high)));
}
