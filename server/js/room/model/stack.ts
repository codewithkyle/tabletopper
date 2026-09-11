import type { Initiative, Pawn } from "../protocol.ts";
export interface Stacked {
	id: string;
	kind: Pawn["kind"];
	z: number;
}
export function compareStack(a: Stacked, b: Stacked): number {
	const kinds = stackRank(a.kind) - stackRank(b.kind);
	if (kinds !== 0) {
		return kinds;
	}
	if (a.z !== b.z) {
		return a.z - b.z;
	}
	return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
}
function stackRank(kind: Pawn["kind"]): number {
	return kind === "object" ? 0 : 1;
}
const NOBODY: readonly string[] = Object.freeze([]);
export function actingPawnIds(initiative: Initiative): readonly string[] {
	if (initiative.active === null) {
		return NOBODY;
	}
	const entry = initiative.entries.find((line) => line.id === initiative.active);
	return entry ? entry.pawnIds : NOBODY;
}
