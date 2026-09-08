// The floor control at the right end of the menu bar: which layer this browser
// is looking at, and whether that is the one the players see.
//
// IT IS THE GM'S AND ONLY THE GM'S. A player's canvas always draws the active
// layer -- it is the only one whose pawns reach them at all, because the
// projection in internal/room removes the rest before the event is encoded --
// so their half of the bar is a server-rendered name and this is never mounted.
//
// IT IS FILLED FROM THE STORE RATHER THAN FETCHED, which is the opposite of
// every other live panel in this room. The reason is that half of what it shows
// is not on the server: the viewed layer is a local choice that is never sent,
// so a fragment could report which floor is ACTIVE and could not report which
// floor this GM is looking at, which is the entire point of the control.
//
// NO CLASS NAME IS WRITTEN IN THIS FILE. server/js is not a Tailwind source, so
// a class named here would never be emitted -- everything below sets text,
// toggles [hidden], and builds options whose styling belongs to the field.

import type { Renderer } from "./render/renderer.ts";
import type { State } from "./protocol.ts";

// htmx is a global from base.templ. The one call is what gives the Show button
// the app's ordinary error handling: a refusal comes back as an HX-Trigger and
// opens the alert modal, exactly as it would from a button with hx-post on it.
//
// IT CANNOT BE A BUTTON WITH hx-post, which is why this is here at all. htmx
// captures the path when it processes an element, so an attribute rewritten
// afterwards is ignored -- and the layer in this path changes every time the GM
// picks another floor.
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

	// The re-declarations are what carry the narrowing above into refresh:
	// TypeScript does not keep a control-flow narrowing across a function
	// boundary, and refresh is one.
	const bar: HTMLElement = found;
	const field: HTMLSelectElement = chooser;

	const viewing = bar.querySelector("[data-layer-viewing]");
	const activate = bar.querySelector("[data-layer-activate]");

	const roomID = mount.dataset.room ?? "";

	// signature is what the choices were built from. Rebuilding them on every
	// event would reset the field while somebody has it open, and would fire no
	// change event to tell anybody -- so the list is rebuilt only when the
	// floors themselves have changed.
	let signature = "";

	field.addEventListener("change", () => {
		renderer.view.choose(field.value);

		// THE FRAME HAS TO BE ASKED FOR. Choosing a floor changes what the
		// canvas would draw and nothing else on the page moves, so an idle
		// loop stays idle -- and the GM picks a floor and watches the old one
		// go on sitting there. The renderer cannot notice this for itself: the
		// override lives in a plain variable, not in the store it re-reads.
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

		// ONE FLOOR IS NO CHOICE, and nearly every room has one floor. A
		// permanent control reading "Ground floor" beside a room with one
		// ground floor is naming the only thing there is.
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

		// The badge and the button are one state and not two: they appear
		// together, when the floor being looked at is not the floor the players
		// are on, and the button is the way back to agreeing.
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
