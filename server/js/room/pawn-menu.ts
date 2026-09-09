// The menu a right click on a pawn puts up.
//
// THE RIGHT BUTTON USED TO OPEN THE PAWN'S WINDOW AND NOW ASKS A QUESTION
// INSTEAD. Opening the window is the first item on this list, the double click
// is the gesture people reach for on their own, and the two things a GM could
// previously only do through a keyboard key or through a control that appears
// for a multiple selection -- move a pawn to another floor, take it off the
// table -- are the other two items.
//
// IT IS ONE MENU AND NOT ONE PER PAWN, for the reason the overlay is one label:
// the pawn it is about is written into it when it opens. Nothing here is
// rendered per pawn and nothing is left behind when it closes.
//
// THE TWO MUTATIONS ARE htmx BUTTONS THAT ALREADY EXIST ON THE PAGE, and this
// only sets hx-vals on them and presses them. That is not a shortcut: removing
// pawns is confirmed, the confirmation is the app's confirm modal, and
// hx-confirm is read off the element making the request -- so building the
// DELETE here would skip the dialog, and window.confirm is banned. The floor
// move goes the same way for consistency and to keep one route rather than two.
//
// NO CLASS NAME IS WRITTEN IN THIS FILE. server/js is not a Tailwind source, so
// a class named here would never be emitted. Everything below sets text,
// toggles [hidden], writes attributes, and clones a <template> whose styling
// belongs to room-pawn-menu.templ.

import type { Named } from "./pawn-window.ts";
import type { Point } from "./render/camera.ts";
import { nextZ } from "./window.ts";

// Floor is one of the room's layers as this list needs it.
export interface Floor {
	id: string;
	name: string;
}

// Target is the pawn a menu is about: its identity, its heading, and the floor
// it is standing on so the list can say which of them that is.
export interface Target extends Named {
	layerId: string;
}

export interface PawnMenuDeps {
	// layers is the room's floors, read when the menu opens rather than held,
	// because a GM adds and renames them while the table is live.
	layers: () => Floor[];

	// details opens the pawn's window, and it is the SAME callback the double
	// click uses. Two ways in that built the window separately would be two
	// windows for one goblin.
	details: (pawn: Named) => void;
}

export interface PawnMenu {
	open(pawn: Target, screen: Point): void;
	close(): void;
	stop(): void;
}

// EDGE is how close to the table's edge the menu may be put before it is turned
// back the other way, in CSS pixels.
const EDGE = 8;

export function mountPawnMenu(mount: HTMLElement, deps: PawnMenuDeps): PawnMenu | null {
	const found = mount.querySelector("[data-pawn-menu]");
	if (!(found instanceof HTMLElement)) {
		return null;
	}

	// The re-declaration is what carries the narrowing above into the closures
	// below: TypeScript does not keep a control-flow narrowing across a
	// function boundary, and every one of these is one.
	const root: HTMLElement = found;

	const heading = root.querySelector("[data-pawn-menu-name]");
	const floors = root.querySelector("[data-pawn-menu-floors]");
	const list = root.querySelector("[data-pawn-menu-layers]");
	const removeButton = root.querySelector("[data-pawn-menu-remove]");

	// THE TWO htmx BUTTONS AND THE ROW TEMPLATE ARE THE GM'S OR NOTHING. A
	// player's page renders none of them, so these are nulls rather than hidden
	// elements and every use below is a no-op without a role test of its own.
	//
	// The move button is looked up on the MOUNT rather than on the menu, for
	// the reason the Delete key's button is: it is not part of the panel, it is
	// never seen, and it exists only because a request needs an element.
	const mover = mount.querySelector("[data-pawn-menu-move]");
	const template = mount.querySelector("[data-pawn-menu-template]");
	const shell = template instanceof HTMLTemplateElement ? template : null;

	// target is the pawn the menu is currently about, and null is closed.
	let target: Target | null = null;

	// origin is where the right click was, kept because the menu is placed more
	// than once: opening the floor submenu changes its height, and a menu that
	// was turned back at the bottom edge has to be turned back again by more.
	const origin: Point = { x: 0, y: 0 };

	function open(pawn: Target, screen: Point): void {
		target = pawn;

		if (heading) {
			heading.textContent = pawn.name;
		}

		// hx-vals AND hx-confirm ARE BOTH SET HERE AND BOTH READ AT REQUEST
		// TIME, which is what lets one button carry a different pawn each time
		// it is pressed. The confirmation names the pawn because this removes
		// the one under the pointer and not the selection, and those are
		// routinely different things.
		removeButton?.setAttribute("hx-vals", vals(pawn.id));
		removeButton?.setAttribute(
			"hx-confirm",
			`Remove ${pawn.name} from the table. This cannot be undone.`,
		);

		fillFloors(pawn);

		// The submenu is folded back every time, so a menu opened on the next
		// pawn reads the way the first one did.
		if (floors instanceof HTMLDetailsElement) {
			floors.open = false;
		}

		root.hidden = false;

		// ABOVE EVERY WINDOW, ASKED FOR RATHER THAN ASSUMED. See window.ts:
		// windows climb as they are raised, so a number written into the markup
		// is only above them for the first few minutes of a session.
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

	// place puts the menu at the pointer, and turns it back on itself at the
	// two edges it would otherwise hang off.
	//
	// IT IS MEASURED AFTER IT IS SHOWN, because a hidden element has no size to
	// read. That is one forced layout per right click, which is a gesture that
	// happens a few times a minute and never inside the frame loop.
	function place(): void {
		const width = root.offsetWidth;
		const height = root.offsetHeight;

		const right = origin.x + width + EDGE > mount.clientWidth;
		const below = origin.y + height + EDGE > mount.clientHeight;

		const x = clamp(right ? origin.x - width : origin.x, mount.clientWidth - width - EDGE);
		const y = clamp(below ? origin.y - height : origin.y, mount.clientHeight - height - EDGE);

		root.style.transform = `translate(${Math.round(x)}px, ${Math.round(y)}px)`;
	}

	// THE FLOOR LIST IS THE ONE THING THAT RESIZES THE MENU AFTER IT IS OPEN.
	// Without this, a right click near the bottom of the table opens a menu that
	// fits and then unfolds a list of floors below the edge of the window.
	function onToggle(): void {
		if (!root.hidden) {
			place();
		}
	}

	// fillFloors rebuilds the submenu. It is rebuilt per open rather than
	// cached, because the list is a few items long and a GM adds, renames and
	// deletes floors while the table is live -- and unlike the overlay's select
	// there is nothing here anybody can have open while it happens.
	function fillFloors(pawn: Target): void {
		if (!(list instanceof HTMLElement) || !shell) {
			return;
		}

		list.replaceChildren();

		for (const layer of deps.layers()) {
			const row = shell.content.cloneNode(true) as DocumentFragment;

			const button = row.querySelector("[data-pawn-menu-layer]");
			if (!(button instanceof HTMLElement)) {
				continue;
			}

			const name = row.querySelector("[data-pawn-menu-layer-name]");
			const here = row.querySelector("[data-pawn-menu-here]");

			button.dataset.pawnMenuLayer = layer.id;
			if (name) {
				name.textContent = layer.name;
			}

			// WHICH FLOOR IT IS ALREADY ON. Without this the list is five names
			// with nothing to tell them apart, and the GM has to open the pawn's
			// window to find out where it is before deciding where to send it.
			if (here instanceof HTMLElement) {
				here.hidden = layer.id !== pawn.layerId;
			}

			list.append(row);
		}
	}

	// moveTo presses the hidden button with the two values written onto it.
	function moveTo(layer: string): void {
		const pawn = target;
		close();

		if (!pawn || layer === "" || layer === pawn.layerId || !(mover instanceof HTMLElement)) {
			return;
		}

		mover.setAttribute("hx-vals", JSON.stringify({ ids: pawn.id, layer }));
		mover.dispatchEvent(new MouseEvent("click", { bubbles: true }));
	}

	// vals is the id in the shape the route reads: one comma-separated field,
	// because that is what the overlay's whole selection posts and there is no
	// second route for a list of one.
	function vals(id: string): string {
		return JSON.stringify({ ids: id });
	}

	function onClick(e: Event): void {
		if (!(e.target instanceof Element)) {
			return;
		}

		if (e.target.closest("[data-pawn-menu-details]")) {
			const pawn = target;
			close();
			if (pawn) {
				deps.details(pawn);
			}

			return;
		}

		const floor = e.target.closest("[data-pawn-menu-layer]");
		if (floor instanceof HTMLElement) {
			moveTo(floor.dataset.pawnMenuLayer ?? "");

			return;
		}

		// THE REMOVAL CLOSES THE MENU AND DOES NOT SEND ANYTHING. htmx is
		// listening on the button itself and has already built its request by
		// the time this bubbles up; what happens next is the confirm modal,
		// which is inert to everything outside it -- so the menu cannot be
		// reopened onto another pawn while an answer is pending.
		if (e.target.closest("[data-pawn-menu-remove]")) {
			close();
		}
	}

	// A POINTER DOWN ANYWHERE ELSE CLOSES IT, which covers nearly everything a
	// hand can do next: panning, zooming by drag, opening a window, pressing a
	// menu in the bar. The right click that opens the menu is safe from this in
	// both of the orders platforms use -- Linux and Windows raise contextmenu
	// on the release and macOS on the press, and either way the pointerdown
	// that precedes it finds a menu that is already closed or one being
	// replaced.
	function onPointerDown(e: Event): void {
		if (root.hidden || (e.target instanceof Node && root.contains(e.target))) {
			return;
		}

		close();
	}

	// And the wheel, which is the one way to move the camera without pressing
	// anything: a menu left behind would be pointing at a pawn that had walked
	// out from under it.
	function onWheel(): void {
		close();
	}

	function onKeyDown(e: KeyboardEvent): void {
		if (e.key === "Escape") {
			close();
		}
	}

	root.addEventListener("click", onClick);
	floors?.addEventListener("toggle", onToggle);
	document.addEventListener("pointerdown", onPointerDown);
	document.addEventListener("wheel", onWheel);
	document.addEventListener("keydown", onKeyDown);

	return {
		open,
		close,

		stop() {
			close();
			root.removeEventListener("click", onClick);
			floors?.removeEventListener("toggle", onToggle);
			document.removeEventListener("pointerdown", onPointerDown);
			document.removeEventListener("wheel", onWheel);
			document.removeEventListener("keydown", onKeyDown);
		},
	};
}

// clamp keeps the menu inside the table at the near edge as well as the far
// one, which is what a right click in the top-left corner needs after the flip
// above has already moved it off the other way.
function clamp(value: number, high: number): number {
	return Math.max(EDGE, Math.min(value, Math.max(EDGE, high)));
}
