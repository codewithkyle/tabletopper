import type { Pawn, Role } from "./protocol.ts";
import type { Rect } from "./model/types.ts";
import { compareStack } from "./model/stack.ts";
import { pawnExtents } from "./model/shape.ts";
export const SELECTION_MAX = 200;
export function mayMove(pawn: Pawn, role: Role, user: string): boolean {
	return role === "gm" || (pawn.ownerId !== null && pawn.ownerId === user);
}
export class Selection {
	private readonly chosen = new Set<string>();
	has(id: string): boolean {
		return this.chosen.has(id);
	}
	get size(): number {
		return this.chosen.size;
	}
	ids(): string[] {
		return [...this.chosen];
	}
	only(): string | null {
		return this.chosen.size === 1 ? (this.chosen.values().next().value ?? null) : null;
	}
	set(ids: readonly string[]): boolean {
		const next = ids.slice(0, SELECTION_MAX);
		if (this.same(next)) {
			return false;
		}
		this.chosen.clear();
		for (const id of next) {
			this.chosen.add(id);
		}
		return true;
	}
	add(ids: readonly string[]): boolean {
		let changed = false;
		for (const id of ids) {
			if (this.chosen.size >= SELECTION_MAX || this.chosen.has(id)) {
				continue;
			}
			this.chosen.add(id);
			changed = true;
		}
		return changed;
	}
	toggle(id: string): boolean {
		if (this.chosen.delete(id)) {
			return true;
		}
		if (this.chosen.size >= SELECTION_MAX) {
			return false;
		}
		this.chosen.add(id);
		return true;
	}
	clear(): boolean {
		if (this.chosen.size === 0) {
			return false;
		}
		this.chosen.clear();
		return true;
	}
	prune(present: ReadonlySet<string>): boolean {
		let changed = false;
		for (const id of this.chosen) {
			if (!present.has(id)) {
				this.chosen.delete(id);
				changed = true;
			}
		}
		return changed;
	}
	private same(ids: readonly string[]): boolean {
		if (ids.length !== this.chosen.size) {
			return false;
		}
		for (const id of ids) {
			if (!this.chosen.has(id)) {
				return false;
			}
		}
		return true;
	}
}
export function marqueeSelect(
	pawns: readonly Pawn[],
	layerID: string,
	rect: Rect,
	role: Role,
	user: string,
	concealed?: (pawn: Pawn) => boolean,
): string[] {
	const x1 = Math.min(rect.x1, rect.x2);
	const x2 = Math.max(rect.x1, rect.x2);
	const y1 = Math.min(rect.y1, rect.y2);
	const y2 = Math.max(rect.y1, rect.y2);
	const found: string[] = [];
	for (const pawn of pawns) {
		if (pawn.layerId !== layerID || !mayMove(pawn, role, user)) {
			continue;
		}
		if (concealed?.(pawn)) {
			continue;
		}
		if (pawn.x < x1 || pawn.x > x2 || pawn.y < y1 || pawn.y > y2) {
			continue;
		}
		found.push(pawn.id);
		if (found.length >= SELECTION_MAX) {
			break;
		}
	}
	return found;
}
export function riders(pawns: readonly Pawn[], anchor: Pawn, cellSize: number): string[] {
	const cell = Math.max(1, cellSize);
	const [halfW, halfH] = pawnExtents(anchor, cellSize);
	if (halfW * 2 < cell * 2 && halfH * 2 < cell * 2) {
		return [];
	}
	const found: string[] = [];
	for (const pawn of pawns) {
		if (pawn.id === anchor.id || pawn.layerId !== anchor.layerId || compareStack(pawn, anchor) <= 0) {
			continue;
		}
		if (Math.abs(pawn.x - anchor.x) > halfW || Math.abs(pawn.y - anchor.y) > halfH) {
			continue;
		}
		found.push(pawn.id);
	}
	return found;
}
export function dragSet(
	pawns: readonly Pawn[],
	anchor: Pawn,
	selection: Selection,
	options: { withRiders: boolean; role: Role; user: string; cellSize: number },
): string[] {
	const ids: string[] = [anchor.id];
	const seen = new Set<string>([anchor.id]);
	const take = (id: string): void => {
		if (seen.has(id) || ids.length >= SELECTION_MAX) {
			return;
		}
		const pawn = pawns.find((p) => p.id === id);
		if (!pawn || !mayMove(pawn, options.role, options.user)) {
			return;
		}
		seen.add(id);
		ids.push(id);
	};
	if (selection.has(anchor.id)) {
		for (const id of selection.ids()) {
			take(id);
		}
	}
	if (options.withRiders) {
		for (const id of riders(pawns, anchor, options.cellSize)) {
			take(id);
		}
	}
	return ids;
}
