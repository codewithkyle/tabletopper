// The spawn dialog's own behaviour, and the bridge from it to the canvas.
//
// THE DIALOG IS SERVER-RENDERED AND THIS IS THE ONE THING IT CANNOT BE. Its
// search, its kind switch, its Back and its Close are ordinary htmx; what needs
// a script is turning the card that was clicked into an armed spec and handing
// that to the renderer.
//
// IT USED TO ASK THE GM TWO MORE QUESTIONS ABOUT A TOKEN AND BOTH ARE GONE. A
// token was creature-or-object with a size or a pair of cell counts beside it;
// a token is an object, and how big it is, is the size of the picture, which
// the card now carries as two data attributes and the server reads again for
// itself.
//
// WHAT DOES STILL GET ASKED IS THE NPC'S NAME AND STAT LINE, and it is asked
// because a face out of the avatar library has nothing behind it to read either
// from. That is a second fragment rather than a second question on the wall:
// picking a face swaps the dialog for a form, and the Place button on that form
// is the pick this file arms from.
//
// ARMING IS A DIRECT CALL AND NO LONGER A WINDOW EVENT. It used to cross
// bundles as `room:arm`, because the menu bar in public/js/room.js armed the
// canvas with a player's own character and could not import this module. That
// item is gone -- putting something on the table is the GM's act, and the GM's
// only way in is this dialog, which is in the same bundle as the canvas -- so
// the event had one raiser fewer than it had listeners and was removed rather
// than left as a hook nothing pulls.
//
// NO CLASS NAME IS WRITTEN HERE. server/js is not a Tailwind source, and there
// is nothing in this file that touches presentation at all.

import { MODAL_CLOSE } from "../../public/js/events.js";
import type { Armed } from "./pawns.ts";
import type { PawnKind, Size } from "./protocol.ts";

// ARMED_SIZE is the creature size a ghost is drawn at before the server has
// said what the thing actually is. A monster's real size is a column in the
// manual, which this dialog's cards do not carry and the spawn does not send --
// the hub reads it when it resolves -- so the pointer carries a one-cell disc
// until the pawn lands.
//
// AN NPC IS THE EXCEPTION, because its size is not written down anywhere for
// the hub to read either: the form asks, so the ghost can be right from the
// first frame.
const ARMED_SIZE: Size = "medium";

// SIZES is the select's whole vocabulary, checked here rather than cast,
// because what comes out of a <select> is a string as far as the compiler is
// concerned and a browser is free to have sent anything.
const SIZES: readonly Size[] = ["tiny", "small", "medium", "large", "huge", "gargantuan"];

// KINDS maps what a pick card says it is onto what the wire calls it. A monster
// is a creature with a stat line already written down, an NPC is a face with
// one just typed, and a token is an object -- a picture of a thing.
const KINDS: Record<string, PawnKind> = {
	monster: "monster",
	npc: "npc",
	token: "object",
};

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

		// A NUMBER THE BROWSER WOULD REFUSE STOPS HERE, and it stops here rather
		// than at the server because of when the two answers arrive. The server
		// checks this stat line as well and refuses it properly -- but by then
		// the dialog has shut and the GM has clicked on the table, so what they
		// would get is an alert about a field they can no longer see. This is
		// the courtesy half of the pair: it shows them the field.
		const bad = invalidField(dialog);
		if (bad) {
			bad.reportValidity();

			return;
		}

		arm(read(dialog, card));
		window.dispatchEvent(new CustomEvent(MODAL_CLOSE));
	}

	document.addEventListener("click", onClick);

	return {
		stop() {
			document.removeEventListener("click", onClick);
		},
	};
}

// read builds the armed spec out of the card and the controls above it.
//
// A MONSTER AND AN NPC ARE CREATURES AND A TOKEN IS AN OBJECT, which is the
// whole of the kind decision. A creature is something with a stat line -- out
// of the manual, off a sheet, or typed into the form this card sits on -- and
// an object is a picture of a thing: a wagon, a door, a crate.
function read(dialog: HTMLElement, card: HTMLElement): Armed {
	const source = card.dataset.spawnSource ?? "monster";
	const npc = source === "npc";

	return {
		kind: KINDS[source] ?? "monster",
		id: card.dataset.spawnId ?? "",
		name: npc ? typed(dialog, card) : card.dataset.spawnName ?? "",
		image: card.dataset.spawnImage ?? "",
		visible: checked(dialog, "[data-spawn-shown]"),
		size: npc ? size(dialog) : ARMED_SIZE,
		width: pixels(card.dataset.spawnWidth),
		height: pixels(card.dataset.spawnHeight),
		hp: number(dialog, "[data-npc-hp]"),
		maxHp: number(dialog, "[data-npc-maxhp]"),
		ac: number(dialog, "[data-npc-ac]"),
	};
}

// typed is the NPC form's name box.
//
// THE FACE'S OWN NAME IS THE FALLBACK AND NOT THE DEFAULT. The box is required,
// so a browser reaching this with it empty is one that skipped the form; the
// server falls back to the picture's name in that case rather than refusing,
// and this agrees with it.
function typed(dialog: HTMLElement, card: HTMLElement): string {
	const found = dialog.querySelector("[data-npc-name]");
	const name = found instanceof HTMLInputElement ? found.value.trim() : "";

	return name || (card.dataset.spawnName ?? "");
}

function checked(dialog: HTMLElement, selector: string): boolean {
	const found = dialog.querySelector(selector);

	return found instanceof HTMLInputElement ? found.checked : true;
}

// number is one of the NPC form's three fields, or zero when the dialog has no
// such field -- which is every dialog but that one.
function number(dialog: HTMLElement, selector: string): number {
	const found = dialog.querySelector(selector);
	if (!(found instanceof HTMLInputElement)) {
		return 0;
	}

	const parsed = Number.parseInt(found.value, 10);

	return Number.isFinite(parsed) ? parsed : 0;
}

// size is the NPC form's select, refusing anything not in the vocabulary. An
// unknown word is medium, which is what the server settles on as well.
function size(dialog: HTMLElement): Size {
	const found = dialog.querySelector("[data-npc-size]");
	if (!(found instanceof HTMLSelectElement)) {
		return ARMED_SIZE;
	}

	const chosen = SIZES.find((s) => s === found.value);

	return chosen ?? ARMED_SIZE;
}

// invalidField is the first control in the dialog the browser will not accept,
// or nothing when every one of them is fine.
//
// IT ASKS EVERY CONTROL RATHER THAN THE ONES IT KNOWS ABOUT, because validity
// is the browser's own question and the answer for a control carrying no
// constraint is always yes. The search box and the visibility switch are
// therefore free to sit in the same sweep as the four the NPC form adds.
function invalidField(dialog: HTMLElement): HTMLInputElement | HTMLSelectElement | null {
	for (const field of dialog.querySelectorAll("input, select")) {
		const control = field instanceof HTMLInputElement || field instanceof HTMLSelectElement;
		if (control && !field.checkValidity()) {
			return field;
		}
	}

	return null;
}

// pixels reads one of the card's size attributes. Zero is "the library row does
// not say", which the canvas draws as one cell -- the same answer the hub gives
// the pawn itself.
function pixels(value: string | undefined): number {
	const parsed = Number.parseInt(value ?? "", 10);

	return Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
}
