import type { State } from "./protocol.ts";
import { nextZ } from "./window.ts";
declare const htmx: {
	ajax(verb: string, path: string, context: { source: Element }): void;
};
export interface LayerTool {
	close(): void;
	stop(): void;
}
const EDGE = 8;
const GAP = 8;
export function mountLayerTool(mount: HTMLElement, state: State): LayerTool | null {
	const opener = mount.querySelector("[data-layer-tool]");
	const found = mount.querySelector("[data-layer-menu]");
	const template = mount.querySelector("[data-layer-menu-template]");
	if (
		!(opener instanceof HTMLElement) ||
		!(found instanceof HTMLElement) ||
		!(template instanceof HTMLTemplateElement)
	) {
		return null;
	}
	const button: HTMLElement = opener;
	const root: HTMLElement = found;
	const shell: HTMLTemplateElement = template;
	const heading = root.querySelector("[data-layer-menu-heading]");
	const roomID = mount.dataset.room ?? "";
	function open(): void {
		fill();
		root.hidden = false;
		root.style.zIndex = String(nextZ());
		button.setAttribute("aria-expanded", "true");
		place();
	}
	function close(): void {
		if (root.hidden) {
			return;
		}
		root.hidden = true;
		button.setAttribute("aria-expanded", "false");
	}
	function fill(): void {
		if (heading) {
			root.replaceChildren(heading);
		} else {
			root.replaceChildren();
		}
		for (const layer of state.table.layers) {
			const row = shell.content.cloneNode(true) as DocumentFragment;
			const choice = row.querySelector("[data-layer-menu-choice]");
			if (!(choice instanceof HTMLElement)) {
				continue;
			}
			const name = row.querySelector("[data-layer-menu-name]");
			const active = row.querySelector("[data-layer-menu-active]");
			choice.dataset.layerMenuChoice = layer.id;
			if (name) {
				name.textContent = layer.name;
			}
			if (active instanceof HTMLElement) {
				active.hidden = layer.id !== state.table.activeLayer;
			}
			root.append(row);
		}
	}
	function place(): void {
		const anchor = button.getBoundingClientRect();
		const box = mount.getBoundingClientRect();
		const x = anchor.left - box.left - root.offsetWidth - GAP;
		const y = anchor.top - box.top;
		root.style.transform = `translate(${Math.round(
			clamp(x, mount.clientWidth - root.offsetWidth - EDGE),
		)}px, ${Math.round(clamp(y, mount.clientHeight - root.offsetHeight - EDGE))}px)`;
	}
	function choose(layer: string): void {
		close();
		if (layer === "" || layer === state.table.activeLayer || typeof htmx === "undefined") {
			return;
		}
		htmx.ajax("POST", `/rooms/${roomID}/layers/${layer}/activate`, { source: button });
	}
	function onClick(e: Event): void {
		if (!(e.target instanceof Element)) {
			return;
		}
		if (e.target.closest("[data-layer-tool]")) {
			if (root.hidden) {
				open();
			} else {
				close();
			}
			return;
		}
		const row = e.target.closest("[data-layer-menu-choice]");
		if (row instanceof HTMLElement) {
			choose(row.dataset.layerMenuChoice ?? "");
		}
	}
	function onPointerDown(e: Event): void {
		if (root.hidden || !(e.target instanceof Node)) {
			return;
		}
		if (root.contains(e.target) || button.contains(e.target)) {
			return;
		}
		close();
	}
	function onKeyDown(e: KeyboardEvent): void {
		if (e.key === "Escape") {
			close();
		}
	}
	function onResize(): void {
		close();
	}
	document.addEventListener("click", onClick);
	document.addEventListener("pointerdown", onPointerDown);
	document.addEventListener("keydown", onKeyDown);
	window.addEventListener("resize", onResize);
	return {
		close,
		stop() {
			close();
			document.removeEventListener("click", onClick);
			document.removeEventListener("pointerdown", onPointerDown);
			document.removeEventListener("keydown", onKeyDown);
			window.removeEventListener("resize", onResize);
		},
	};
}
function clamp(value: number, high: number): number {
	return Math.max(EDGE, Math.min(value, Math.max(EDGE, high)));
}
