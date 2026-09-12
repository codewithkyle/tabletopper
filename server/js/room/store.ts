import type {
	Event,
	FogShape,
	Pawn,
	Player,
	State,
	Stroke,
} from "./protocol.ts";
type Identified = Player | Pawn | FogShape | Stroke;
export function reduce(state: State, event: Event): void {
	switch (event.type) {
		case "snapshot":
			Object.assign(state, clone(event.state));
			break;
		case "room.updated":
			state.room = clone(event.room);
			break;
		case "table.updated":
			state.table = clone(event.table);
			break;
		case "initiative.updated":
			state.initiative = clone(event.initiative);
			break;
		case "player.joined":
		case "player.updated":
			upsert(state.players, clone(event.player));
			break;
		case "player.left":
			state.players = without(state.players, event.id);
			break;
		case "pawn.spawned":
		case "pawn.updated":
			upsert(state.pawns, clone(event.pawn));
			break;
		case "pawn.removed":
			state.pawns = without(state.pawns, event.id);
			break;
		case "fog.added":
			upsert(state.fog, clone(event.shape));
			break;
		case "fog.removed":
			state.fog = without(state.fog, event.id);
			break;
		case "stroke.began":
			upsert(state.strokes, clone(event.stroke));
			break;
		case "stroke.ended": {
			const stroke = byID(state.strokes, event.id);
			if (stroke) {
				stroke.done = true;
			}
			break;
		}
		case "stroke.erased":
			state.strokes = state.strokes.filter((stroke) => !event.ids.includes(stroke.id));
			break;
		case "pawn.moved":
			for (const at of event.pawns) {
				const pawn = byID(state.pawns, at.id);
				if (pawn) {
					pawn.x = at.x;
					pawn.y = at.y;
				}
			}
			break;
		case "stroke.extended": {
			const stroke = byID(state.strokes, event.id);
			if (stroke) {
				stroke.points.push(...event.points);
			}
			break;
		}
		case "error":
		case "pinged":
		case "pawn.dragging":
		case "player.kicked":
		case "room.closed":
			return;
		default: {
			const unreduced: never = event;
			throw new Error(`room: no reduction for ${(unreduced as Event).type}`);
		}
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
function upsert<T extends Identified>(into: T[], value: T): void {
	const at = into.findIndex((existing) => existing.id === value.id);
	if (at === -1) {
		into.push(value);
		return;
	}
	into[at] = value;
}
function without<T extends Identified>(from: T[], id: string): T[] {
	return from.filter((value) => value.id !== id);
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
