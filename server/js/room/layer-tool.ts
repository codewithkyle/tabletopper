// The floors menu in the tool pill: one click to change the floor the players
// are shown. Why it exists and why it is the ACTIVE layer it changes are in
// server/templ/pages/room-layer-tool.go, beside the markup.
//
// IT IS FILLED FROM THE STORE AND REBUILT EVERY TIME IT OPENS, which is what
// the right-click menu's floor list does and for the same reasons: a GM adds,
// renames and deletes floors while the table is live, the list is a few names
// long, and nothing anybody can be holding open goes stale.
//
// IT IS PLACED BY HAND RATHER THAN BY BEING A CHILD OF THE BUTTON. The pill is
// positioned and z-indexed, so it is a stacking context, and a menu inside it
// could never rise above a window sitting under that corner. This one is a
// sibling of the table and is raised above every window as it opens, exactly as
// the right-click menu is.
//
// NO CLASS NAME IS WRITTEN IN THIS FILE. server/js is not a Tailwind source, so
// a class named here would never be emitted. Everything below sets text,
// toggles [hidden], writes attributes, and clones a <template> whose styling
// belongs to room-layer-tool.templ.

import type { State } from "./protocol.ts";
import { nextZ } from "./window.ts";

// htmx is a global from base.templ. The one call is what gives this the app's
// ordinary error handling: a refusal comes back as an HX-Trigger and opens the
// alert modal, exactly as it would from a button with hx-post on it.
//
// IT CANNOT BE A BUTTON WITH hx-post, which is the same wall layer-bar.ts hits:
// htmx captures the path when it processes an element, and the floor is IN the
// path, so an attribute rewritten per row is an attribute that is ignored.
declare const htmx: {
	ajax(verb: string, path: string, context: { source: Element }): void;
};

export interface LayerTool {
	close(): void;
	stop(): void;
}

// EDGE is how close to the table's edge the menu may be put, and GAP is how far
// it stands off the button it belongs to. Both in CSS pixels.
const EDGE = 8;
const GAP = 8;

export function mountLayerTool(mount: HTMLElement, state: State): LayerTool | null {
	const opener = mount.querySelector("[data-layer-tool]");
	const found = mount.querySelector("[data-layer-menu]");
	const template = mount.querySelector("[data-layer-menu-template]");

	// A PLAYER'S PAGE RENDERS NONE OF THE THREE. Activating a floor is the GM's
	// alone, so there is nothing here to mount rather than a control to hide.
	if (
		!(opener instanceof HTMLElement) ||
		!(found instanceof HTMLElement) ||
		!(template instanceof HTMLTemplateElement)
	) {
		return null;
	}

	// The re-declarations carry the narrowing above into the closures below.
	const button: HTMLElement = opener;
	const root: HTMLElement = found;
	const shell: HTMLTemplateElement = template;

	// The heading is kept across a rebuild and the rows are replaced, which is
	// why it is the one element in the menu with a data attribute of its own.
	const heading = root.querySelector("[data-layer-menu-heading]");
	const roomID = mount.dataset.room ?? "";

	function open(): void {
		fill();

		root.hidden = false;

		// ABOVE EVERY WINDOW, ASKED FOR RATHER THAN ASSUMED. See window.ts:
		// windows climb as they are raised, so a number written into the markup
		// is only above them for the first few minutes of a session.
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

			// WHICH FLOOR THEY ARE ON NOW. Without it the list is five names
			// with nothing to tell them apart, and the fastest way to find out
			// which one is live is the window this exists to avoid opening.
			if (active instanceof HTMLElement) {
				active.hidden = layer.id !== state.table.activeLayer;
			}

			root.append(row);
		}
	}

	// place anchors the menu to the button and stands it off to the left, which
	// is the side the pill is not against.
	//
	// IT IS MEASURED AFTER IT IS FILLED AND SHOWN, because a hidden element has
	// no size to read. That is one forced layout per opening, which is a gesture
	// that happens a few times a minute and never inside the frame loop.
	function place(): void {
		const anchor = button.getBoundingClientRect();
		const box = mount.getBoundingClientRect();

		const x = anchor.left - box.left - root.offsetWidth - GAP;
		const y = anchor.top - box.top;

		root.style.transform = `translate(${Math.round(
			clamp(x, mount.clientWidth - root.offsetWidth - EDGE),
		)}px, ${Math.round(clamp(y, mount.clientHeight - root.offsetHeight - EDGE))}px)`;
	}

	// choose asks for a floor to be made active. The one already active is not
	// asked for: the route would answer with an event that changed nothing, and
	// every client would redraw for it.
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

	// A pointer down anywhere else closes it, which covers panning, opening a
	// window, and the right button putting up a menu of its own. The button is
	// exempt because the click that follows is what toggles it, and a close here
	// would be undone by that click a moment later.
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

	// A resized window has moved the button the menu was measured against.
	// Closing is honest and re-placing is not: the pill may have moved out from
	// under it entirely.
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

// clamp keeps the menu inside the table at the near edge as well as the far
// one, for a window too small to hold it at the place it was asked for.
function clamp(value: number, high: number): number {
	return Math.max(EDGE, Math.min(value, Math.max(EDGE, high)));
}
