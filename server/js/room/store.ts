import type {
	Cell,
	Change,
	Event,
	FogShape,
	Layer,
	Pawn,
	Player,
	Roll,
	State,
	Stroke,
} from "./protocol.ts";
type Identified = Player | Pawn | Layer | FogShape | Stroke | Roll;
type Celled = { layerId: string; q: number; r: number };
type Reducers = {
	[K in Change["type"]]: (state: State, change: Extract<Change, { type: K }>) => void;
};
const reducers: Reducers = {
	"room.updated": (state, change) => {
		state.room = clone(change.room);
	},
	"table.updated": (state, change) => {
		Object.assign(state.table, clone(change.table));
	},
	"layers.updated": (state, change) => {
		state.table.layers = clone(change.layers);
	},
	"initiative.updated": (state, change) => {
		state.initiative = clone(change.initiative);
	},
	"players.upserted": (state, change) => upsert(state.players, change.players),
	"players.removed": (state, change) => remove(state.players, change.ids),
	"pawns.upserted": (state, change) => upsert(state.pawns, change.pawns),
	"pawns.removed": (state, change) => remove(state.pawns, change.ids),
	"pawns.moved": (state, change) => {
		for (const at of change.pawns) {
			const pawn = byID(state.pawns, at.id);
			if (pawn) {
				pawn.x = at.x;
				pawn.y = at.y;
			}
		}
	},
	"fog.upserted": (state, change) => upsert(state.fog, change.shapes),
	"fog.removed": (state, change) => remove(state.fog, change.ids),
	"music.updated": (state, change) => {
		state.music = clone(change.music);
	},
	"rolls.upserted": (state, change) => upsert(state.rolls, change.rolls),
	"rolls.removed": (state, change) => remove(state.rolls, change.ids),
	"strokes.upserted": (state, change) => upsert(state.strokes, change.strokes),
	"strokes.removed": (state, change) => remove(state.strokes, change.ids),
	"strokes.extended": (state, change) => {
		const stroke = byID(state.strokes, change.id);
		if (stroke) {
			stroke.points.push(...change.points);
		}
	},
	"tiles.stamped": (state, change) => upsertCells(state.tiles, change.tiles),
	"tiles.erased": (state, change) => removeCells(state.tiles, change.layer, change.cells),
	"notes.upserted": (state, change) => upsertCells(state.notes, change.notes),
	"notes.removed": (state, change) => removeCells(state.notes, change.layer, change.cells),
	"palette.updated": (state, change) => {
		state.table.palette = clone(change.palette);
	},
	"strokes.ended": (state, change) => {
		const stroke = byID(state.strokes, change.id);
		if (stroke) {
			stroke.done = true;
		}
	},
};
export function reduce(state: State, event: Event): void {
	switch (event.type) {
		case "snapshot":
			Object.assign(state, clone(event.state));
			break;
		case "error":
		case "pinged":
		case "rolled":
		case "pawn.dragging":
		case "player.kicked":
		case "room.closed":
			return;
		default:
			(reducers[event.type] as (state: State, change: Change) => void)(state, event);
			break;
	}
	normalize(state);
}
export function normalize(state: State): void {
	state.players.sort(byIdentifier);
	state.pawns.sort(byIdentifier);
	state.strokes.sort(byIdentifier);
	state.rolls.sort(byIdentifier);
	state.tiles.sort(byCell);
	state.notes.sort(byCell);
}
export function empty(): State {
	return {
		schema: 0,
		seq: 0,
		room: { id: "", name: "", locked: false },
		table: {
			layers: [],
			palette: [],
			activeLayer: "",
			grid: {
				type: "square",
				lines: "solid",
				cellSize: 64,
				offsetX: 0,
				offsetY: 0,
				color: "#000000FF",
				snap: "cells",
				feetPerCell: 5,
				units: "feet",
				diagonals: "equal",
				numbered: false,
			},
			pawnLabels: "default",
			playersCanDraw: true,
			playersCanStamp: false,
			fogPrefill: false,
			initiativeGrouping: "grouped",
		},
		players: [],
		pawns: [],
		initiative: { entries: [], active: null, round: 0 },
		fog: [],
		strokes: [],
		tiles: [],
		notes: [],
		rolls: [],
		music: { trackId: null, name: "", playing: false, loop: false, at: 0, since: 0 },
	};
}
function upsert<T extends Identified>(into: T[], values: readonly T[]): void {
	for (const value of values) {
		const at = into.findIndex((existing) => existing.id === value.id);
		if (at === -1) {
			into.push(clone(value));
			continue;
		}
		into[at] = clone(value);
	}
}
function remove<T extends Identified>(from: T[], ids: readonly string[]): void {
	for (let at = from.length - 1; at >= 0; at--) {
		if (ids.includes(from[at].id)) {
			from.splice(at, 1);
		}
	}
}
function upsertCells<T extends Celled>(into: T[], values: readonly T[]): void {
	for (const value of values) {
		const at = into.findIndex((held) => sameCell(held, value));
		if (at === -1) {
			into.push(clone(value));
			continue;
		}
		into[at] = clone(value);
	}
}
function removeCells<T extends Celled>(from: T[], layer: string, cells: readonly Cell[]): void {
	for (let at = from.length - 1; at >= 0; at--) {
		const held = from[at];
		if (held.layerId === layer && cells.some((cell) => cell.q === held.q && cell.r === held.r)) {
			from.splice(at, 1);
		}
	}
}
function sameCell(a: Celled, b: Celled): boolean {
	return a.layerId === b.layerId && a.q === b.q && a.r === b.r;
}
function byID<T extends Identified>(from: T[], id: string): T | undefined {
	return from.find((value) => value.id === id);
}
function byCell(a: Celled, b: Celled): number {
	if (a.layerId !== b.layerId) {
		return a.layerId < b.layerId ? -1 : 1;
	}
	if (a.q !== b.q) {
		return a.q - b.q;
	}
	return a.r - b.r;
}
function byIdentifier(a: Identified, b: Identified): number {
	return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
}
function clone<T>(value: T): T {
	return structuredClone(value);
}
