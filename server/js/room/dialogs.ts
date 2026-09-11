import { MODAL_CLOSE } from "../../public/js/events.js";
import type { Armed } from "./pawns.ts";
import type { PawnKind, Size } from "./protocol.ts";

const ARMED_SIZE: Size = "medium";
const SIZES: readonly Size[] = ["tiny", "small", "medium", "large", "huge", "gargantuan"];
const KINDS: Record<string, PawnKind> = {
	monster: "monster",
	npc: "npc",
	token: "object",
};

export interface Arming {
	stop(): void;
}

export function mountDialogs(arm: (armed: Armed | null) => void): Arming {
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

function typed(dialog: HTMLElement, card: HTMLElement): string {
	const found = dialog.querySelector("[data-npc-name]");
	const name = found instanceof HTMLInputElement ? found.value.trim() : "";

	return name || (card.dataset.spawnName ?? "");
}

function checked(dialog: HTMLElement, selector: string): boolean {
	const found = dialog.querySelector(selector);

	return found instanceof HTMLInputElement ? found.checked : true;
}

function number(dialog: HTMLElement, selector: string): number {
	const found = dialog.querySelector(selector);
	if (!(found instanceof HTMLInputElement)) {
		return 0;
	}

	const parsed = Number.parseInt(found.value, 10);

	return Number.isFinite(parsed) ? parsed : 0;
}

function size(dialog: HTMLElement): Size {
	const found = dialog.querySelector("[data-npc-size]");
	if (!(found instanceof HTMLSelectElement)) {
		return ARMED_SIZE;
	}

	const chosen = SIZES.find((s) => s === found.value);

	return chosen ?? ARMED_SIZE;
}

function invalidField(dialog: HTMLElement): HTMLInputElement | HTMLSelectElement | null {
	for (const field of dialog.querySelectorAll("input, select")) {
		const control = field instanceof HTMLInputElement || field instanceof HTMLSelectElement;
		if (control && !field.checkValidity()) {
			return field;
		}
	}

	return null;
}

function pixels(value: string | undefined): number {
	const parsed = Number.parseInt(value ?? "", 10);

	return Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
}
