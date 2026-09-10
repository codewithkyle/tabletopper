// The Fog menu's two items, kept pointed at the floor this GM is LOOKING at.
//
// WHICH FLOOR THAT IS EXISTS ONLY HERE. The viewed layer is a local override
// that is never sent -- mountLayerBar's header spells out why -- so a
// server-rendered menu item cannot know it, and `Fill fog` on the floor the
// players happen to be standing on is the wrong floor exactly when a GM is
// prepping the next scene.
//
// IT IS hx-vals AND NOT THE PATH, and that is the whole reason this module
// exists rather than a line in the template. htmx captures an element's path
// when it PROCESSES the element, so a path rewritten afterwards is ignored --
// the wall layer-bar.ts hit and answered with htmx.ajax. hx-vals is read when
// the request is built instead, which makes it the one place on a button a
// changing value can live while the button still carries hx-confirm. Neither
// item could give up the confirm: each throws away every shape on the floor.
//
// AN EMPTY VALUE IS STILL A WORKING ITEM. The template ships hx-vals of "{}"
// and the route falls back to the active floor when no layer arrives, so a page
// whose bundle never ran -- no WebGL2, a script that threw -- has two menu items
// that do the ordinary thing rather than two that silently do nothing.
//
// NO CLASS NAME IS WRITTEN IN THIS FILE, for the reason layer-bar.ts writes
// none: server/js is not a Tailwind source.

export interface FogMenu {
	refresh(): void;
}

export function mountFogMenu(viewed: () => string): FogMenu | null {
	const items = Array.from(document.querySelectorAll("[data-room-layered]"));
	if (items.length === 0) {
		return null;
	}

	// last is what was written, so a settle that changed nothing writes nothing.
	// This runs on every frame that settles a floor, and setAttribute on an
	// element htmx is watching is not free.
	let last = "";

	function refresh(): void {
		const layer = viewed();
		if (layer === last) {
			return;
		}
		last = layer;

		const vals = layer === "" ? "{}" : JSON.stringify({ layer });
		for (const item of items) {
			item.setAttribute("hx-vals", vals);
		}
	}

	refresh();

	return { refresh };
}
