// The spawn dialog's own behaviour, and the bridge from it to the canvas.
//
// THE DIALOG IS SERVER-RENDERED AND THIS IS THE ONE THING IT CANNOT BE. Its
// search, its kind switch and its Close are ordinary htmx; what needs a script
// is turning the card that was clicked into an armed spec and handing that to
// the renderer.
//
// IT USED TO ASK THE GM TWO MORE QUESTIONS AND BOTH ARE GONE. A token was
// creature-or-object with a size or a pair of cell counts beside it; a token is
// an object, and how big it is, is the size of the picture, which the card now
// carries as two data attributes and the server reads again for itself. What is
// left of the controls is the Players-see-it switch.
//
// ARMING IS A DIRECT CALL AND NO LONGER A WINDOW EVENT. It used to cross
// bundles as `room:arm`, because the menu bar in public/js/room.js armed the
// canvas with a player's own character and could not import this module. That
// item is gone -- putting something on the table is the GM's act, and the GM's
// only way in is this dialog, which is in the same bundle as the canvas -- so
// the event had one raiser fewer than it had listeners and was removed rather
// than left as a hook nothing pulls.
//
// NO CLASS NAME IS WRITTEN HERE. server/js is not a Tailwind source, and after
// the two questions went there is nothing left in this file that touches
// presentation at all.

import type { Armed } from "./pawns.ts";
import type { Size } from "./protocol.ts";

// ARMED_SIZE is the creature size a ghost is drawn at before the server has
// said what the thing actually is. A monster's real size is a column in the
// manual, which this dialog's cards do not carry and the spawn does not send --
// the hub reads it when it resolves -- so the pointer carries a one-cell disc
// until the pawn lands.
const ARMED_SIZE: Size = "medium";

export interface Arming {
	stop(): void;
}

// mountDialogs wires the document once. The listener is delegated, because the
// spawn dialog arrives in a modal swap and its cards arrive in a second swap
// inside that one.
export function mountDialogs(arm: (armed: Armed | null) => void): Arming {
	// A card arms and closes.
	function onClick(e: MouseEvent): void {
		if (!(e.target instanceof Element)) {
			return;
		}

		const card = e.target.closest("[data-spawn-pick]");
		if (!(card instanceof HTMLElement)) {
			return;
		}

		const dialog = card.closest("#room-spawn");
		if (!(dialog instanceof HTMLElement)) {
			return;
		}

		arm(read(dialog, card));
		window.dispatchEvent(new CustomEvent("modal:close"));
	}

	document.addEventListener("click", onClick);

	return {
		stop() {
			document.removeEventListener("click", onClick);
		},
	};
}

// read builds the armed spec out of the card and the one switch above it.
//
// A MONSTER IS A CREATURE AND A TOKEN IS AN OBJECT, which is the whole of the
// kind decision now. A creature is something with a stat line -- a monster from
// the manual or a player's character -- and a token is a picture of a thing:
// a wagon, a door, a crate.
function read(dialog: HTMLElement, card: HTMLElement): Armed {
	const monster = (card.dataset.spawnSource ?? "monster") === "monster";

	return {
		kind: monster ? "monster" : "object",
		id: card.dataset.spawnId ?? "",
		name: card.dataset.spawnName ?? "",
		image: card.querySelector("img")?.getAttribute("src") ?? "",
		visible: checked(dialog, "[data-spawn-shown]"),
		size: ARMED_SIZE,
		width: pixels(card.dataset.spawnWidth),
		height: pixels(card.dataset.spawnHeight),
	};
}

function checked(dialog: HTMLElement, selector: string): boolean {
	const found = dialog.querySelector(selector);

	return found instanceof HTMLInputElement ? found.checked : true;
}

// pixels reads one of the card's size attributes. Zero is "the library row does
// not say", which the canvas draws as one cell -- the same answer the hub gives
// the pawn itself.
function pixels(value: string | undefined): number {
	const parsed = Number.parseInt(value ?? "", 10);

	return Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
}
