import type { Grid, HPBand, Pawn, Stroke } from "../protocol.ts";
import type { Rgb } from "./types.ts";
export interface Drawn {
	id: string;
	kind: Pawn["kind"];
	name: string;
	image: string;
	x: number;
	y: number;
	z: number;
	size: Pawn["size"];
	width: number;
	height: number;
	rotation: number;
	hidden: boolean;
	health: HPBand | null;
}
export type Ghostable = Pick<
	Drawn,
	"id" | "kind" | "name" | "image" | "x" | "y" | "z" | "size" | "width" | "height" | "rotation"
>;
export interface Outline {
	x: number;
	y: number;
	halfW: number;
	halfH: number;
	color: Rgb;
	alpha: number;
	thickness: number;
	rect: boolean;
	rotation: number;
}
export interface Segment {
	x0: number;
	y0: number;
	x1: number;
	y1: number;
	color: Rgb;
	alpha: number;
	width: number;
}
export interface Cell {
	x: number;
	y: number;
	size: number;
	type: Grid["type"];
	color: Rgb;
	alpha: number;
}
export interface Label {
	text: string;
	x: number;
	y: number;
	color: [number, number, number];
	alpha: number;
}
export interface Handle {
	lx: number;
	ly: number;
	turns: boolean;
	x: number;
	y: number;
	rotation: number;
}
export interface Overlay {
	readonly ghosts: Drawn[];
	readonly outlines: Outline[];
	readonly segments: Segment[];
	readonly cells: Cell[];
	readonly labels: Label[];
	readonly handles: Handle[];
	inHand: Stroke | null;
	reset(): void;
}
export function newOverlay(): Overlay {
	const ghosts: Drawn[] = [];
	const outlines: Outline[] = [];
	const segments: Segment[] = [];
	const cells: Cell[] = [];
	const labels: Label[] = [];
	const handles: Handle[] = [];
	const overlay: Overlay = {
		ghosts,
		outlines,
		segments,
		cells,
		labels,
		handles,
		inHand: null,
		reset() {
			ghosts.length = 0;
			outlines.length = 0;
			segments.length = 0;
			cells.length = 0;
			labels.length = 0;
			handles.length = 0;
			overlay.inHand = null;
		},
	};
	return overlay;
}
export interface Pool<T> {
	take(): T;
	reset(): void;
}
export function pool<T>(blank: () => T): Pool<T> {
	const slots: T[] = [];
	let used = 0;
	return {
		take() {
			const slot = slots[used] ?? (slots[used] = blank());
			used++;
			return slot;
		},
		reset() {
			used = 0;
		},
	};
}
export function blankDrawn(): Drawn {
	return {
		id: "", kind: "monster", name: "", image: "",
		x: 0, y: 0, z: 0, size: "medium",
		width: 0, height: 0, rotation: 0, hidden: false, health: null,
	};
}
export function ghostOf(from: Ghostable, x: number, y: number, into: Drawn): Drawn {
	into.id = from.id;
	into.kind = from.kind;
	into.name = from.name;
	into.image = from.image;
	into.x = x;
	into.y = y;
	into.z = from.z;
	into.size = from.size;
	into.width = from.width;
	into.height = from.height;
	into.rotation = from.rotation;
	into.hidden = false;
	into.health = null;
	return into;
}
export function blankOutline(): Outline {
	return {
		x: 0, y: 0, halfW: 0, halfH: 0,
		color: [1, 1, 1], alpha: 1, thickness: 1, rect: false, rotation: 0,
	};
}
export function blankSegment(): Segment {
	return { x0: 0, y0: 0, x1: 0, y1: 0, color: [1, 1, 1], alpha: 1, width: 1 };
}
export function blankCell(): Cell {
	return { x: 0, y: 0, size: 0, type: "square", color: [1, 1, 1], alpha: 1 };
}
export function blankLabel(): Label {
	return { text: "", x: 0, y: 0, color: [1, 1, 1], alpha: 1 };
}
