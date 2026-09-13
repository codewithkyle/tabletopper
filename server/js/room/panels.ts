import {
	ROOM_CHARACTER,
	ROOM_INFO,
	ROOM_INITIATIVE,
	ROOM_MUSIC,
	ROOM_PAWN,
	ROOM_PLAYERS,
	ROOM_RESYNC,
	ROOM_ROLLS,
	ROOM_TABLETOP,
	WINDOW_CLOSE,
	WINDOW_RETITLE,
} from "../../public/js/events.js";
import type { Change, Frame, InitiativeEntry, Pawn } from "./protocol.ts";
import { PAWN_WINDOW } from "./pawn-window.ts";
import { SHEET_WINDOW } from "./sheet-window.ts";
import { openWindows } from "./window.ts";
let tracked = new Set<string>();
let seated = new Map<string, string>();
let mine = "";
const panelEvents: Partial<Record<Change["type"], string>> = {
	"players.upserted": ROOM_PLAYERS,
	"players.removed": ROOM_PLAYERS,
	"initiative.updated": ROOM_INITIATIVE,
	"rolls.upserted": ROOM_ROLLS,
	"rolls.removed": ROOM_ROLLS,
	"music.updated": ROOM_MUSIC,
	"room.updated": ROOM_INFO,
	"table.updated": ROOM_TABLETOP,
	"layers.updated": ROOM_TABLETOP,
};
const everything = [ROOM_PLAYERS, ROOM_INITIATIVE, ROOM_INFO, ROOM_TABLETOP, ROOM_ROLLS, ROOM_MUSIC];
export function announce(frame: Frame): void {
	if (frame.type === "snapshot") {
		track(frame.state.initiative.entries);
		seat(frame);
		for (const name of everything) {
			window.dispatchEvent(new CustomEvent(name));
		}
		window.dispatchEvent(new CustomEvent(ROOM_RESYNC));
		if (mine !== "") {
			window.dispatchEvent(new CustomEvent(ROOM_CHARACTER, { detail: { id: mine } }));
		}
		reconcilePawnWindows(frame.state.pawns);
		return;
	}
	if (frame.type === "rolled") {
		window.dispatchEvent(new CustomEvent(ROOM_ROLLS));
		return;
	}
	if (frame.type !== "changes") {
		return;
	}
	const panels = new Set<string>();
	for (const change of frame.events) {
		switch (change.type) {
			case "pawns.upserted":
				for (const pawn of change.pawns) {
					pawnChanged(pawn.id, panels);
					window.dispatchEvent(
						new CustomEvent(WINDOW_RETITLE, {
							detail: { id: PAWN_WINDOW + pawn.id, title: pawn.name },
						}),
					);
					if (pawn.characterId !== null) {
						seated.set(pawn.id, pawn.characterId);
					} else {
						seated.delete(pawn.id);
					}
					if (pawn.characterId !== null && pawn.characterId === mine) {
						characterChanged(mine);
						window.dispatchEvent(
							new CustomEvent(WINDOW_RETITLE, {
								detail: { id: SHEET_WINDOW, title: pawn.name },
							}),
						);
					}
				}
				continue;
			case "pawns.removed":
				for (const id of change.ids) {
					pawnChanged(id, panels);
					window.dispatchEvent(
						new CustomEvent(WINDOW_CLOSE, { detail: { id: PAWN_WINDOW + id } }),
					);
					if (seated.get(id) === mine && mine !== "") {
						characterChanged(mine);
					}
					seated.delete(id);
				}
				continue;
			case "initiative.updated":
				track(change.initiative.entries);
				break;
		}
		const name = panelEvents[change.type];
		if (name) {
			panels.add(name);
		}
	}
	for (const name of panels) {
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
function pawnChanged(id: string, panels: Set<string>): void {
	window.dispatchEvent(new CustomEvent(ROOM_PAWN, { detail: { id } }));
	if (tracked.has(id)) {
		panels.add(ROOM_INITIATIVE);
	}
}
function characterChanged(id: string): void {
	window.dispatchEvent(new CustomEvent(ROOM_CHARACTER, { detail: { id } }));
}
function seat(frame: Frame & { type: "snapshot" }): void {
	seated = new Map();
	for (const pawn of frame.state.pawns ?? []) {
		if (pawn.characterId !== null) {
			seated.set(pawn.id, pawn.characterId);
		}
	}
	const me = (frame.state.players ?? []).find((player) => player.id === frame.you.id);
	mine = me?.characterId ?? "";
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
