import {
	ROOM_INFO,
	ROOM_INITIATIVE,
	ROOM_PAWN,
	ROOM_PLAYERS,
	ROOM_TABLETOP,
	WINDOW_CLOSE,
	WINDOW_RETITLE,
} from "../../public/js/events.js";
import type { Event, InitiativeEntry, Pawn } from "./protocol.ts";
import { PAWN_WINDOW } from "./pawn-window.ts";
import { openWindows } from "./window.ts";
let tracked = new Set<string>();
const panelEvents: Partial<Record<Event["type"], string>> = {
	"player.joined": ROOM_PLAYERS,
	"player.updated": ROOM_PLAYERS,
	"player.left": ROOM_PLAYERS,
	"initiative.updated": ROOM_INITIATIVE,
	"room.updated": ROOM_INFO,
	"table.updated": ROOM_TABLETOP,
};
const everything = [ROOM_PLAYERS, ROOM_INITIATIVE, ROOM_INFO, ROOM_TABLETOP];
export function announce(event: Event): void {
	if (event.type === "snapshot") {
		track(event.state.initiative.entries);
		for (const name of everything) {
			window.dispatchEvent(new CustomEvent(name));
		}
		reconcilePawnWindows(event.state.pawns);
		return;
	}
	if (event.type === "initiative.updated") {
		track(event.initiative.entries);
	}
	switch (event.type) {
		case "pawn.spawned":
		case "pawn.updated":
			pawnChanged(event.pawn.id);
			window.dispatchEvent(
				new CustomEvent(WINDOW_RETITLE, {
					detail: { id: PAWN_WINDOW + event.pawn.id, title: event.pawn.name },
				}),
			);
			return;
		case "pawn.removed":
			pawnChanged(event.id);
			window.dispatchEvent(
				new CustomEvent(WINDOW_CLOSE, { detail: { id: PAWN_WINDOW + event.id } }),
			);
			return;
	}
	const name = panelEvents[event.type];
	if (name) {
		window.dispatchEvent(new CustomEvent(name));
	}
}
function reconcilePawnWindows(pawns: readonly Pawn[]): void {
	let present: Set<string> | null = null;
	for (const id of openWindows()) {
		if (!id.startsWith(PAWN_WINDOW)) {
			continue;
		}
		present ??= new Set(pawns.map((p) => p.id));
		const pawn = id.slice(PAWN_WINDOW.length);
		if (present.has(pawn)) {
			window.dispatchEvent(new CustomEvent(ROOM_PAWN, { detail: { id: pawn } }));
		} else {
			window.dispatchEvent(new CustomEvent(WINDOW_CLOSE, { detail: { id } }));
		}
	}
}
function pawnChanged(id: string): void {
	window.dispatchEvent(new CustomEvent(ROOM_PAWN, { detail: { id } }));
	if (tracked.has(id)) {
		window.dispatchEvent(new CustomEvent(ROOM_INITIATIVE));
	}
}
function track(entries: readonly InitiativeEntry[]): void {
	const ids = new Set<string>();
	for (const entry of entries) {
		for (const id of entry.pawnIds) {
			ids.add(id);
		}
	}
	tracked = ids;
}
