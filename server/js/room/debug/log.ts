import type { Dropped } from "../socket.ts";
import type { Frame } from "../protocol.ts";
export interface Entry {
	at: number;
	out: boolean;
	type: string;
	detail: string;
	seq: number;
	bytes: number;
	by: string;
	note: string;
	text: string;
}
export function incoming(text: string, frame: Frame | null, dropped: Dropped | null, at: number): Entry {
	return {
		at,
		out: false,
		type: frame ? frame.type : "unreadable",
		detail: detailOf(frame),
		seq: frame && "seq" in frame ? frame.seq : 0,
		bytes: text.length,
		by: frame && frame.by ? short(frame.by) : "",
		note: dropped ?? "",
		text,
	};
}
export function outgoing(text: string, at: number): Entry {
	const command = parse(text);
	return {
		at,
		out: true,
		type: typeof command?.type === "string" ? command.type : "unreadable",
		detail: typeof command?.cid === "string" ? `cid ${command.cid}` : "",
		seq: 0,
		bytes: text.length,
		by: "",
		note: "",
		text,
	};
}
export function describe(entry: Entry): string {
	const parts = [clock(entry.at), entry.out ? ">" : "<", entry.type];
	if (entry.seq !== 0) {
		parts.push(`#${entry.seq}`);
	}
	if (entry.detail !== "") {
		parts.push(entry.detail);
	}
	parts.push(`${entry.bytes}b`);
	if (entry.by !== "") {
		parts.push(`by ${entry.by}`);
	}
	if (entry.note !== "") {
		parts.push(`[${entry.note}]`);
	}
	return parts.join(" ");
}
export function matches(entry: Entry, term: string): boolean {
	if (term === "") {
		return true;
	}
	const wanted = term.toLowerCase();
	return entry.type.toLowerCase().includes(wanted) ||
		entry.note.toLowerCase().includes(wanted) ||
		entry.text.toLowerCase().includes(wanted);
}
export function pretty(text: string): string {
	const value = parse(text);
	return value === null ? text : JSON.stringify(value, null, 2);
}
export function cidOf(text: string): string {
	const command = parse(text);
	return typeof command?.cid === "string" ? command.cid : "";
}
function detailOf(frame: Frame | null): string {
	if (!frame) {
		return "";
	}
	if (frame.type === "changes") {
		return `(${frame.events.length}) ${frame.events.map((event) => event.type).join(" ")}`;
	}
	if (frame.type === "error") {
		return `${frame.code} cid ${frame.cid}`;
	}
	if (frame.type === "pawn.dragging") {
		return `(${frame.pawns.length})`;
	}
	return "";
}
function parse(text: string): Record<string, unknown> | null {
	try {
		const value: unknown = JSON.parse(text);
		return typeof value === "object" && value !== null ? value as Record<string, unknown> : null;
	} catch {
		return null;
	}
}
function short(id: string): string {
	return id.slice(-6);
}
function clock(at: number): string {
	const whole = Math.floor(at / 1000);
	const seconds = whole % 60;
	const minutes = Math.floor(whole / 60) % 60;
	const millis = Math.floor(at % 1000);
	return `${pad(minutes, 2)}:${pad(seconds, 2)}.${pad(millis, 3)}`;
}
function pad(value: number, width: number): string {
	return String(value).padStart(width, "0");
}
