// The spawn dialog's own behaviour, and the bridge from it to the canvas.
//
// THE DIALOG IS SERVER-RENDERED AND THIS IS THE THREE THINGS IT CANNOT BE. Its
// search, its Spawn party button and its Close are ordinary htmx; what needs a
// script is reading the controls above the results when a card is clicked,
// showing the object fields instead of the size when the kind changes, and
// handing the answer to the renderer.
//
// ARMING CROSSES BUNDLES AS A WINDOW EVENT, which is the shape the View menu
// already uses: public/js/room.js and this module are in different bundles and
// cannot import each other, so `room:arm` is the contract and it is spelled out
// in both. The player's own "Place my pawn" comes from the menu bar through the
// same event, which is why the canvas does not care which raised it.
//
// NO CLASS NAME IS WRITTEN HERE. server/js is not a Tailwind source; the object
// fields are toggled with [hidden] and the pressed state with aria-pressed,
// both of which are rendered in templ.

import type { Armed } from "./pawns.ts";
import type { Size } from "./protocol.ts";

// ARM_EVENT is the contract. It is also named in public/js/room.js, which
// raises it for the player's own character.
export const ARM_EVENT = "room:arm";

export interface Arming {
	stop(): void;
}

// mountDialogs wires the document once. Every listener is delegated, because
// the spawn dialog arrives in a modal swap and its cards arrive in a second
// swap inside that one.
export function mountDialogs(arm: (armed: Armed | null) => void): Arming {
	function onArm(e: Event): void {
		const detail = (e as CustomEvent<Partial<Armed> | null>).detail;
		if (!detail || !detail.kind) {
			arm(null);

			return;
		}

		arm({
			kind: detail.kind,
			id: detail.id ?? "",
			name: detail.name ?? "",
			image: detail.image ?? "",
			visible: detail.visible ?? true,
			size: detail.size ?? "medium",
			footprintW: detail.footprintW ?? 1,
			footprintH: detail.footprintH ?? 1,
		});
	}

	// A card arms and closes. The kind comes from the card for a monster and
	// from the Creature-or-Object switch for a token, because a token is a
	// picture and what it becomes is the GM's choice rather than the library's.
	function onClick(e: MouseEvent): void {
		if (!(e.target instanceof Element)) {
			return;
		}

		const kindSwitch = e.target.closest("[data-spawn-as]");
		if (kindSwitch instanceof HTMLElement) {
			chooseKind(kindSwitch);

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

	function chooseKind(pressed: HTMLElement): void {
		const dialog = pressed.closest("#room-spawn");
		if (!(dialog instanceof HTMLElement)) {
			return;
		}

		const object = pressed.dataset.spawnAs === "object";

		for (const button of dialog.querySelectorAll("[data-spawn-as]")) {
			button.setAttribute("aria-pressed", String(button === pressed));
		}
		for (const field of dialog.querySelectorAll("[data-spawn-creature]")) {
			(field as HTMLElement).hidden = object;
		}
		for (const field of dialog.querySelectorAll("[data-spawn-object]")) {
			(field as HTMLElement).hidden = !object;
		}
	}

	window.addEventListener(ARM_EVENT, onArm);
	document.addEventListener("click", onClick);

	return {
		stop() {
			window.removeEventListener(ARM_EVENT, onArm);
			document.removeEventListener("click", onClick);
		},
	};
}

// read builds the armed spec out of the card and the controls above it.
function read(dialog: HTMLElement, card: HTMLElement): Armed {
	const source = card.dataset.spawnSource ?? "monster";
	const object = pressedKind(dialog) === "object";

	// A monster is always a creature; a token is whichever the switch says.
	const kind = source === "monster" ? "monster" : object ? "object" : "npc";

	return {
		kind,
		id: card.dataset.spawnId ?? "",
		name: card.dataset.spawnName ?? "",
		image: card.querySelector("img")?.getAttribute("src") ?? "",
		visible: checked(dialog, "[data-spawn-shown]"),
		size: (value(dialog, "[data-spawn-size]") || "medium") as Size,
		footprintW: number(dialog, "[data-spawn-width]"),
		footprintH: number(dialog, "[data-spawn-height]"),
	};
}

function pressedKind(dialog: HTMLElement): string {
	for (const button of dialog.querySelectorAll("[data-spawn-as]")) {
		if (button.getAttribute("aria-pressed") === "true") {
			return (button as HTMLElement).dataset.spawnAs ?? "creature";
		}
	}

	return "creature";
}

function checked(dialog: HTMLElement, selector: string): boolean {
	const found = dialog.querySelector(selector);

	return found instanceof HTMLInputElement ? found.checked : true;
}

function value(dialog: HTMLElement, selector: string): string {
	const found = dialog.querySelector(selector);

	return found instanceof HTMLSelectElement || found instanceof HTMLInputElement ? found.value : "";
}

function number(dialog: HTMLElement, selector: string): number {
	const parsed = Number.parseInt(value(dialog, selector), 10);

	return Number.isFinite(parsed) && parsed > 0 ? parsed : 1;
}
