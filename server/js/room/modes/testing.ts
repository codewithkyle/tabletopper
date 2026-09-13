import type { DrawMode } from "./draw.ts";
import type { FogMode, FogShape, Grid, Pawn, ShapeKind, State, Stroke } from "../protocol.ts";
import type { Mode } from "../tools.ts";
import type { Overlay } from "../model/overlay.ts";
import { empty } from "../store.ts";
import { newOverlay } from "../model/overlay.ts";
const keydown: ((e: { key: string }) => void)[] = [];
(globalThis as unknown as { document: unknown }).document = {
	addEventListener(type: string, fn: (e: { key: string }) => void) {
		if (type === "keydown") {
			keydown.push(fn);
		}
	},
	removeEventListener() {},
};
(globalThis as unknown as { window: unknown }).window = { devicePixelRatio: 1 };
let skew = 0;
const realNow = performance.now.bind(performance);
performance.now = () => realNow() + skew;
export function wait(ms: number): void {
	skew += ms;
}
const { createTable } = await import("./table.ts");
export const GROUND = "01LAYERGROUND";
export const CELLAR = "01LAYERCELLAR";
export const GM = "01GM";
export const PLAYER = "01PLAYER";
export const NONE = { shift: false, alt: false };
export const SHIFT = { shift: true, alt: false };
export const ALT = { shift: false, alt: true };
export const at = (x: number, y: number) => ({ x, y });
export function grid(over: Partial<Grid> = {}): Grid {
	return {
		lines: "solid",
		cellSize: 64,
		offsetX: 0,
		offsetY: 0,
		color: "#000000FF",
		snap: "cells",
		feetPerCell: 5,
		diagonals: "equal",
		...over,
	};
}
export function pawn(over: Partial<Pawn> = {}): Pawn {
	return {
		id: "01PAWN",
		kind: "monster",
		layerId: GROUND,
		name: "Goblin",
		image: "",
		x: 0,
		y: 0,
		z: 1,
		size: "medium",
		width: 0,
		height: 0,
		rotation: 0,
		visible: true,
		hp: 7,
		maxHp: 7,
		hpBand: null,
		ac: 15,
		conditions: [],
		ownerId: null,
		monsterId: null,
		characterId: null,
		...over,
	};
}
export function press(key: string, target: unknown = null, mods: Record<string, boolean> = {}): void {
	for (const fn of keydown) {
		(fn as (e: unknown) => void)({ key, target, ...mods });
	}
}
export function cleared(): FogShape {
	return {
		id: "01CLEARED", layerId: GROUND, kind: "rect", mode: "reveal",
		points: [0, 0, 128, 128],
	};
}
export function drawn(over: Partial<Stroke> = {}): Stroke {
	return {
		id: "01LINE", by: GM, layerId: GROUND, kind: "free",
		color: "#FF0000", width: 4, points: [0, 0, 100, 0], done: true, ...over,
	};
}
export function table(
	pawns: Pawn[],
	over: Partial<{
		role: "gm" | "player"; user: string; grid: Grid;
		scale: number; mode: Mode;
		shape: ShapeKind; fogMode: FogMode;
		fogEnabled: boolean; fogPrefill: boolean; fog: FogShape[];
		drawMode: DrawMode; strokes: Stroke[];
	}> = {},
) {
	const state: State = empty();
	state.pawns = pawns;
	state.table.grid = over.grid ?? grid();
	state.table.activeLayer = GROUND;
	state.table.layers = [{
		id: GROUND, name: "Ground floor", map: null,
		fogEnabled: over.fogEnabled ?? false,
		fogPrefill: over.fogPrefill ?? true,
		partyStart: null,
	}];
	state.fog = over.fog ?? [];
	state.strokes = over.strokes ?? [];
	const sent: Record<string, unknown>[] = [];
	const opened: string[] = [];
	const menus: string[] = [];
	let removals = 0;
	let chosen: Mode = over.mode ?? "select";
	let held = false;
	const drawOptions = { mode: over.drawMode ?? "pen", color: "#FF0000", width: 4 };
	const fogOptions = { shape: over.shape ?? "rect", mode: over.fogMode ?? "reveal" };
	const controller = createTable({
		state,
		role: over.role ?? "gm",
		user: over.user ?? GM,
		viewed: () => GROUND,
		send: (command) => {
			sent.push(command as unknown as Record<string, unknown>);
		},
		invalidate: () => {},
		scale: () => over.scale ?? 1,
		details: (p) => {
			opened.push(p.id);
		},
		menu: (p) => {
			menus.push(p.id);
		},
		remove: () => {
			removals += 1;
		},
		mode: () => (held ? "pan" : chosen),
		chosen: () => chosen,
		fogOptions: () => fogOptions,
		drawOptions: () => drawOptions,
	});
	const overlay = newOverlay();
	function out(): Overlay {
		overlay.reset();
		controller.tool.contribute(overlay);
		return overlay;
	}
	return {
		controller,
		sent,
		state,
		opened,
		menus,
		removals: () => removals,
		choose: (next: Mode) => {
			chosen = next;
		},
		hold: (on: boolean) => {
			held = on;
		},
		chooseDraw: (mode: DrawMode) => {
			drawOptions.mode = mode;
		},
		chooseBrush: (color: string, width: number) => {
			drawOptions.color = color;
			drawOptions.width = width;
		},
		chooseFog: (shape: ShapeKind, mode: FogMode) => {
			fogOptions.shape = shape;
			fogOptions.mode = mode;
		},
		out,
		outlines: () => out().outlines,
		ghosts: () => out().ghosts,
		segments: () => out().segments,
		labels: () => out().labels,
		cells: () => out().cells,
		handles: () => out().handles,
		inHand: () => out().inHand,
	};
}
