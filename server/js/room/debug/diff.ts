import type { State } from "../protocol.ts";
const namesShown = 3;
export interface SliceDiff {
	name: string;
	detail: string;
}
interface Identified {
	id: string;
}
export function diffState(mine: State, theirs: State): SliceDiff[] {
	const out: SliceDiff[] = [];
	const { layers: myLayers, ...myTable } = mine.table;
	const { layers: theirLayers, ...theirTable } = theirs.table;
	push(out, plainDiff("schema", mine.schema, theirs.schema));
	push(out, plainDiff("room", mine.room, theirs.room));
	push(out, plainDiff("table", myTable, theirTable));
	push(out, plainDiff("layers", myLayers, theirLayers));
	push(out, plainDiff("initiative", mine.initiative, theirs.initiative));
	push(out, keyedDiff("players", mine.players, theirs.players));
	push(out, keyedDiff("pawns", mine.pawns, theirs.pawns));
	push(out, keyedDiff("fog", mine.fog, theirs.fog));
	push(out, keyedDiff("strokes", mine.strokes, theirs.strokes));
	return out;
}
export function same(a: unknown, b: unknown): boolean {
	return firstDifference(a, b, "") === "";
}
export function firstDifference(a: unknown, b: unknown, path: string): string {
	if (a === b) {
		return "";
	}
	const where = path === "" ? "value" : path;
	if (Array.isArray(a) || Array.isArray(b)) {
		if (!Array.isArray(a) || !Array.isArray(b)) {
			return `${where}: ${kind(a)} here, ${kind(b)} there`;
		}
		if (a.length !== b.length) {
			return `${where}: ${a.length} here, ${b.length} there`;
		}
		for (let i = 0; i < a.length; i++) {
			const found = firstDifference(a[i], b[i], `${where}[${i}]`);
			if (found !== "") {
				return found;
			}
		}
		return "";
	}
	if (isObject(a) && isObject(b)) {
		const keys = new Set([...Object.keys(a), ...Object.keys(b)]);
		for (const key of [...keys].sort()) {
			if (!(key in a) || !(key in b)) {
				return `${where}.${key}: only ${key in a ? "here" : "there"}`;
			}
			const found = firstDifference(a[key], b[key], path === "" ? key : `${path}.${key}`);
			if (found !== "") {
				return found;
			}
		}
		return "";
	}
	return `${where}: ${show(a)} here, ${show(b)} there`;
}
function plainDiff(name: string, mine: unknown, theirs: unknown): SliceDiff | null {
	const found = firstDifference(mine, theirs, "");
	return found === "" ? null : { name, detail: found };
}
function keyedDiff(name: string, mine: readonly Identified[], theirs: readonly Identified[]): SliceDiff | null {
	const byID = new Map(mine.map((one) => [one.id, one]));
	const missing: string[] = [];
	const changed: string[] = [];
	for (const one of theirs) {
		const found = byID.get(one.id);
		if (!found) {
			missing.push(one.id);
			continue;
		}
		byID.delete(one.id);
		if (!same(found, one)) {
			changed.push(one.id);
		}
	}
	const stale = [...byID.keys()];
	if (missing.length === 0 && stale.length === 0 && changed.length === 0) {
		return null;
	}
	const parts: string[] = [`${mine.length} here, ${theirs.length} there`];
	if (missing.length > 0) {
		parts.push(`${missing.length} never arrived (${names(missing)})`);
	}
	if (stale.length > 0) {
		parts.push(`${stale.length} never left (${names(stale)})`);
	}
	if (changed.length > 0) {
		parts.push(`${changed.length} differ (${names(changed)})`);
	}
	return { name, detail: parts.join(", ") };
}
function push(out: SliceDiff[], found: SliceDiff | null): void {
	if (found) {
		out.push(found);
	}
}
function names(ids: readonly string[]): string {
	const shown = ids.slice(0, namesShown).map((id) => id.slice(-6)).join(" ");
	return ids.length > namesShown ? `${shown} ...` : shown;
}
function isObject(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null;
}
function kind(value: unknown): string {
	return Array.isArray(value) ? "a list" : value === null ? "null" : typeof value;
}
function show(value: unknown): string {
	const text = typeof value === "string" ? value : JSON.stringify(value) ?? String(value);
	return text.length > 40 ? `${text.slice(0, 40)}...` : text;
}
