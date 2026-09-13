import type { Event, InitiativeEntry, State } from "./protocol.ts";
import type { TurnSound } from "./turn-sound.ts";
export interface OnDeck {
	entry: string;
	name: string;
	after: string;
}
export interface TurnAlert {
	event(event: Event): void;
	settings(alerting: boolean, notifying: boolean): void;
	stop(): void;
}
export interface TurnAlertOptions {
	user: string;
	sound: TurnSound;
	away?: () => boolean;
}
const WATCHED = new Set(["snapshot", "initiative.updated", "pawns.upserted", "pawns.removed"]);
const TITLE = "You're on deck";
const TAG = "tabletopper-on-deck";
export function nameOf(state: State, entry: InitiativeEntry): string {
	for (const id of entry.pawnIds) {
		const pawn = state.pawns.find((p) => p.id === id);
		if (pawn) {
			return pawn.name;
		}
	}
	return entry.name;
}
export function skipped(state: State, entry: InitiativeEntry): boolean {
	let found = false;
	for (const id of entry.pawnIds) {
		const pawn = state.pawns.find((p) => p.id === id);
		if (!pawn) {
			continue;
		}
		found = true;
		if (pawn.kind === "player" || pawn.hpBand !== "dead") {
			return false;
		}
	}
	return found;
}
export function upNext(state: State): InitiativeEntry | null {
	const entries = state.initiative.entries;
	const count = entries.length;
	if (count === 0) {
		return null;
	}
	const active = state.initiative.active;
	const from = active === null ? -1 : entries.findIndex((e) => e.id === active);
	if (from < 0) {
		return entries.find((e) => !skipped(state, e)) ?? entries[0];
	}
	for (let step = 1; step <= count; step++) {
		const entry = entries[(from + step) % count];
		if (!skipped(state, entry)) {
			return entry;
		}
	}
	return entries[(from + 1) % count];
}
export function onDeck(state: State, user: string): OnDeck | null {
	const active = state.initiative.active;
	if (user === "" || active === null) {
		return null;
	}
	const next = upNext(state);
	if (next === null || next.id === active) {
		return null;
	}
	const mine = next.pawnIds.some((id) => state.pawns.find((p) => p.id === id)?.ownerId === user);
	if (!mine) {
		return null;
	}
	const current = state.initiative.entries.find((e) => e.id === active);
	return {
		entry: next.id,
		name: nameOf(state, next),
		after: current === undefined ? "" : nameOf(state, current),
	};
}
export function deckLine(deck: OnDeck): string {
	return deck.after === "" ? deck.name : `${deck.name} — after ${deck.after}`;
}
export function struck(seen: string | null, deck: OnDeck | null, resync: boolean): boolean {
	if (resync || deck === null) {
		return false;
	}
	return deck.entry !== seen;
}
export function mountTurnAlert(mount: HTMLElement, state: State, options: TurnAlertOptions): TurnAlert | null {
	const found = mount.querySelector("[data-on-deck]");
	if (!(found instanceof HTMLElement)) {
		return null;
	}
	const banner = found;
	const line = banner.querySelector("[data-on-deck-line]");
	const close = banner.querySelector("[data-on-deck-close]");
	const away = options.away ?? (() => document.hidden);
	let alerting = true;
	let notifying = false;
	let seen: string | null = null;
	let dismissed: string | null = null;
	let posted: Notification | null = null;
	function drop(): void {
		posted?.close();
		posted = null;
	}
	function paint(deck: OnDeck | null): void {
		if (deck === null || !alerting || dismissed === deck.entry) {
			banner.hidden = true;
			return;
		}
		banner.hidden = false;
		if (line instanceof HTMLElement) {
			line.textContent = deckLine(deck);
		}
	}
	function post(deck: OnDeck): void {
		if (!notifying || !away() || typeof Notification === "undefined") {
			return;
		}
		if (Notification.permission !== "granted") {
			return;
		}
		try {
			drop();
			posted = new Notification(TITLE, { body: deckLine(deck), tag: TAG });
		} catch {
		}
	}
	function onVisible(): void {
		if (!document.hidden) {
			drop();
		}
	}
	function onClose(): void {
		dismissed = seen;
		drop();
		banner.hidden = true;
	}
	function refresh(resync: boolean): void {
		const deck = onDeck(state, options.user);
		const ring = alerting && struck(seen, deck, resync);
		const entry = deck === null ? null : deck.entry;
		if (entry !== seen) {
			seen = entry;
			dismissed = null;
			drop();
		}
		if (ring && deck !== null) {
			options.sound.play();
			post(deck);
		}
		paint(deck);
	}
	close?.addEventListener("click", onClose);
	document.addEventListener("visibilitychange", onVisible);
	return {
		event(e) {
			if (!WATCHED.has(e.type)) {
				return;
			}
			refresh(e.type === "snapshot");
		},
		settings(nextAlerting, nextNotifying) {
			alerting = nextAlerting;
			notifying = nextNotifying;
			if (!alerting) {
				drop();
			}
			paint(onDeck(state, options.user));
		},
		stop() {
			drop();
			close?.removeEventListener("click", onClose);
			document.removeEventListener("visibilitychange", onVisible);
		},
	};
}
