import type {
	Change,
	Event,
	FogShape,
	Layer,
	Pawn,
	Player,
	State,
	Stroke,
} from "./protocol.ts";
type Identified = Player | Pawn | Layer | FogShape | Stroke;
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
	"strokes.upserted": (state, change) => upsert(state.strokes, change.strokes),
	"strokes.removed": (state, change) => remove(state.strokes, change.ids),
	"strokes.extended": (state, change) => {
		const stroke = byID(state.strokes, change.id);
		if (stroke) {
			stroke.points.push(...change.points);
		}
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
}
export function empty(): State {
	return {
		schema: 0,
		seq: 0,
		room: { id: "", name: "", locked: false },
		table: {
			layers: [],
			activeLayer: "",
			grid: {
				lines: "solid",
				cellSize: 64,
				offsetX: 0,
				offsetY: 0,
				color: "#000000FF",
				snap: "cells",
				feetPerCell: 5,
				diagonals: "equal",
			},
			pawnLabels: "default",
			playersCanDraw: true,
			fogPrefill: false,
			initiativeGrouping: "grouped",
		},
		players: [],
		pawns: [],
		initiative: { entries: [], active: null, round: 0 },
		fog: [],
		strokes: [],
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
function byID<T extends Identified>(from: T[], id: string): T | undefined {
	return from.find((value) => value.id === id);
}
function byIdentifier(a: Identified, b: Identified): number {
	return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
}
function clone<T>(value: T): T {
	return structuredClone(value);
}
