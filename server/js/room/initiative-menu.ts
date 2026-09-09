// The menu the right button puts up on a line of the turn order, and the double
// click beside it.
//
// A LINE'S GESTURES ARE THE TABLE'S GESTURES. A click gives that creature the
// turn, a double click opens its pawn window, the right button puts up a small
// menu, and a drag reorders. The last three are what a pawn ON THE CANVAS
// already answers to, so a line answering the same way is one thing to learn
// rather than two.
//
// IT IS pawn-menu.ts's SHAPE AND NOT pawn-menu.ts. That one is built around one
// pawn, its floors and its window; a line of the tracker is a GROUP of nine
// pawns or a lair action with none, and neither of those has a floor to be moved
// to. What is shared is the mechanism: clone nothing, write into one menu, place
// it inside the table's bounds, and press an htmx button rather than building a
// request.
//
// THE TWO ITEMS ARE Open pawn panel AND Remove from tracker, and the first is
// hidden on a line that is not one creature. Removing is NOT confirmed: it is
// undone by pressing Sync tracker, which is one menu away. Clear tracker keeps
// its confirmation, because Clear is the whole fight.
//
// THE REMOVAL PRESSES THE LINE'S OWN HIDDEN BUTTON, which is where the DELETE
// and its URL live. That is the same reason pawn-menu.ts presses buttons instead
// of building requests -- htmx reads the verb attribute when it PROCESSES an
// element, so a URL written onto a shared button after the fact is a URL htmx
// never sees.
//
// NO CLASS NAME IS WRITTEN IN THIS FILE, for the reason server/js never carries
// one: it is not a Tailwind source. What it writes is text, [hidden] and a
// transform, and every class is in room-initiative.templ.

import type { Named } from "./pawn-window.ts";
import type { Point } from "./render/camera.ts";
import { nextZ } from "./window.ts";

export interface EntryMenuDeps {
	// details opens a pawn's window, and it is the SAME callback the canvas's
	// double click uses. Two ways in that built the window separately would be
	// two windows for one goblin.
	details: (pawn: Named) => void;
}

export interface EntryMenu {
	close(): void;
	stop(): void;
}

// EDGE is how close to the table's edge the menu may be put before it is turned
// back the other way, in CSS pixels.
const EDGE = 8;

export function mountEntryMenu(mount: HTMLElement, deps: EntryMenuDeps): EntryMenu | null {
	const found = mount.querySelector("[data-entry-menu]");
	if (!(found instanceof HTMLElement)) {
		return null;
	}

	// The re-declaration is what carries the narrowing above into the closures
	// below: TypeScript does not keep a control-flow narrowing across a
	// function boundary, and every one of these is one.
	const root: HTMLElement = found;

	const heading = root.querySelector("[data-entry-menu-name]");
	const details = root.querySelector("[data-entry-menu-details]");

	// target is the line the menu is currently about, and null is closed.
	let target: HTMLElement | null = null;

	// origin is where the right click was, kept because the menu is measured
	// after it is shown -- a hidden element has no size to read.
	const origin: Point = { x: 0, y: 0 };

	// named is the creature a line is about, or null for a line with no pawn.
	// The name is read out of the rendered markup rather than out of an
	// attribute, which is how pawn-menu.ts reads it too.
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

	function place(): void {
		const width = root.offsetWidth;
		const height = root.offsetHeight;

		const right = origin.x + width + EDGE > mount.clientWidth;
		const below = origin.y + height + EDGE > mount.clientHeight;

		const x = clamp(right ? origin.x - width : origin.x, mount.clientWidth - width - EDGE);
		const y = clamp(below ? origin.y - height : origin.y, mount.clientHeight - height - EDGE);

		root.style.transform = `translate(${Math.round(x)}px, ${Math.round(y)}px)`;
	}

	function onContextMenu(event: MouseEvent): void {
		if (!(event.target instanceof Element)) {
			return;
		}

		const entry = event.target.closest("[data-entry]");
		if (!(entry instanceof HTMLElement)) {
			return;
		}

		event.preventDefault();

		const box = mount.getBoundingClientRect();
		open(entry, { x: event.clientX - box.left, y: event.clientY - box.top });
	}

	// A DOUBLE CLICK OPENS THE PAWN WINDOW, on a line that is one creature. A
	// group is nine windows and a lair action is none, so both do nothing --
	// which is the same answer the canvas gives for a double click on bare
	// floor.
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

	// A POINTER DOWN ANYWHERE ELSE CLOSES IT, which covers nearly everything a
	// hand can do next. The right click that opens the menu is safe from this
	// in both of the orders platforms use -- Linux and Windows raise contextmenu
	// on the release and macOS on the press -- because the pointerdown that
	// precedes it finds a menu that is already closed or one being replaced.
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

	// A SWAP TAKES THE LINE THE MENU IS ABOUT OUT OF THE DOCUMENT. The strip
	// refetches on every hit and every turn, so a menu left open would be
	// pointing at an element that is no longer there and its Remove would press
	// a button in a detached tree.
	function onSettle(event: Event): void {
		if (event.target instanceof Element && event.target.hasAttribute("data-turns")) {
			close();
		}
	}

	mount.addEventListener("contextmenu", onContextMenu);
	mount.addEventListener("dblclick", onDoubleClick);
	root.addEventListener("click", onClick);
	document.addEventListener("pointerdown", onPointerDown);
	document.addEventListener("wheel", onWheel);
	document.addEventListener("keydown", onKeyDown);
	document.addEventListener("htmx:afterSettle", onSettle);

	return {
		close,

		stop() {
			close();
			mount.removeEventListener("contextmenu", onContextMenu);
			mount.removeEventListener("dblclick", onDoubleClick);
			root.removeEventListener("click", onClick);
			document.removeEventListener("pointerdown", onPointerDown);
			document.removeEventListener("wheel", onWheel);
			document.removeEventListener("keydown", onKeyDown);
			document.removeEventListener("htmx:afterSettle", onSettle);
		},
	};
}

// clamp keeps the menu inside the table at the near edge as well as the far
// one, which is what a right click in the top-left corner needs after the flip
// above has already moved it off the other way.
function clamp(value: number, high: number): number {
	return Math.max(EDGE, Math.min(value, Math.max(EDGE, high)));
}
