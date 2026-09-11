import type { Renderer } from "./render/renderer.ts";
import type { State } from "./protocol.ts";
declare const htmx: {
	ajax(verb: string, path: string, context: { source: Element }): void;
};
export interface LayerBar {
	refresh(): void;
}
export function mountLayerBar(mount: HTMLElement, state: State, renderer: Renderer): LayerBar | null {
	const found = document.querySelector("[data-layer-bar]");
	if (!(found instanceof HTMLElement)) {
		return null;
	}
	const chooser = found.querySelector("[data-layer-select]");
	if (!(chooser instanceof HTMLSelectElement)) {
		return null;
	}
	const bar: HTMLElement = found;
	const field: HTMLSelectElement = chooser;
	const viewing = bar.querySelector("[data-layer-viewing]");
	const activate = bar.querySelector("[data-layer-activate]");
	const roomID = mount.dataset.room ?? "";
	let signature = "";
	field.addEventListener("change", () => {
		renderer.view.choose(field.value);
		renderer.invalidate();
		refresh();
	});
	activate?.addEventListener("click", () => {
		const layer = field.value;
		if (layer === "" || typeof htmx === "undefined") {
			return;
		}
		htmx.ajax("POST", `/rooms/${roomID}/layers/${layer}/activate`, { source: activate });
	});
	function refresh(): void {
		const layers = state.table.layers;
		bar.hidden = layers.length < 2;
		if (bar.hidden) {
			return;
		}
		const next = layers.map((l) => `${l.id} ${l.name}`).join("");
		if (next !== signature) {
			signature = next;
			field.replaceChildren(
				...layers.map((l) => {
					const choice = document.createElement("option");
					choice.value = l.id;
					choice.textContent = l.name;
					return choice;
				}),
			);
		}
		const viewed = renderer.view.viewed()?.id ?? "";
		if (field.value !== viewed) {
			field.value = viewed;
		}
		const following = renderer.view.following();
		if (viewing instanceof HTMLElement) {
			viewing.hidden = following;
		}
		if (activate instanceof HTMLElement) {
			activate.hidden = following;
		}
	}
	refresh();
	return { refresh };
}
