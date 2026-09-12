import type { Armed } from "./place.ts";
import type { DrawOptions } from "./draw.ts";
import type { Event, Pawn, Role, State } from "../protocol.ts";
import type { FogOptions } from "./fog.ts";
import type { Mode } from "../tools.ts";
import type { Outgoing } from "../socket.ts";
import type { Point, Rect } from "../model/types.ts";
import type { Selection } from "../selection.ts";
import type { Tool } from "../render/input.ts";
import { createDraw } from "./draw.ts";
import { createFog } from "./fog.ts";
import { createMeasure } from "./measure.ts";
import { createPan } from "./pan.ts";
import { createPing } from "./ping.ts";
import { createPlace } from "./place.ts";
import { createSelect } from "./select.ts";
import { newBoard } from "./board.ts";
import { newToolSwitch } from "./switch.ts";
export interface TableDeps {
	state: State;
	role: Role;
	user: string;
	viewed: () => string;
	send: (command: Outgoing) => void;
	invalidate: () => void;
	scale: () => number;
	details: (pawn: Pawn) => void;
	menu: (pawn: Pawn, screen: Point) => void;
	remove: () => void;
	mode: () => Mode;
	chosen: () => Mode;
	fogOptions: () => FogOptions;
	drawOptions: () => DrawOptions;
}
export interface Table {
	tool: Tool;
	selection: Selection;
	focus(): Pawn | null;
	bounds(): Rect | null;
	preview(event: Event): void;
	floorChanged(): void;
	arm(armed: Armed | null): void;
	isArmed(): boolean;
	onChange(fn: () => void): void;
	stop(): void;
}
export function createTable(deps: TableDeps): Table {
	let changed: (() => void) | null = null;
	const announce = (): void => {
		changed?.();
		deps.invalidate();
	};
	const board = newBoard({
		state: deps.state,
		role: deps.role,
		user: deps.user,
		viewed: deps.viewed,
		send: deps.send,
		announce,
	});
	const select = createSelect({
		board,
		invalidate: deps.invalidate,
		scale: deps.scale,
		details: deps.details,
		menu: deps.menu,
	});
	const place = createPlace({
		viewed: deps.viewed,
		grid: board.grid,
		send: deps.send,
		announce,
	});
	const tool = newToolSwitch({
		mode: deps.mode,
		chosen: deps.chosen,
		remove: deps.remove,
		select,
		place,
		pan: createPan(),
		measure: createMeasure({
			grid: board.grid,
			scale: deps.scale,
			invalidate: deps.invalidate,
		}),
		ping: createPing({ viewed: deps.viewed, send: deps.send }),
		fog: createFog({
			state: deps.state,
			viewed: deps.viewed,
			grid: board.grid,
			send: deps.send,
			invalidate: deps.invalidate,
			options: deps.fogOptions,
		}),
		draw: createDraw({
			state: deps.state,
			role: deps.role,
			user: deps.user,
			viewed: deps.viewed,
			grid: board.grid,
			send: deps.send,
			invalidate: deps.invalidate,
			scale: deps.scale,
			options: deps.drawOptions,
		}),
	});
	return {
		tool,
		selection: select.selection,
		focus: select.focus,
		bounds: select.bounds,
		preview: select.preview,
		floorChanged: select.floorChanged,
		arm: place.arm,
		isArmed: place.isArmed,
		onChange(fn) {
			changed = fn;
		},
		stop: tool.stop,
	};
}
