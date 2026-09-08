// The client's copy of the room, and the reducer that keeps it in step with the
// server's.
//
// THIS IS A PORT OF internal/room/reduce.go, LINE FOR LINE, AND IT HAS TO STAY
// ONE. The Go reducer is the specification; reduce.test.ts replays the golden
// fixtures that the Go tests generate, so a change on that side fails this side
// until the port follows. That is the whole reason a reducer exists in Go at
// all -- the server never reduces its own events.
//
// THE THREE SIZING RULES ARE VISIBLE AS THREE SHAPES BELOW. Singletons assign,
// collection items upsert or delete by id, and the three hot paths mutate named
// fields of an entity that is already here.

import type {
	Event,
	FogShape,
	Pawn,
	Player,
	State,
	Stroke,
} from "./protocol.ts";

// Identified is every entity the reducer looks up, which is every entity that
// lives in a collection.
type Identified = Player | Pawn | FogShape | Stroke;

// reduce applies one event to one state, in place.
//
// IN PLACE, AND NOT A NEW OBJECT, because the renderer holds a reference to
// this and reads it once a frame. A reducer that returned a fresh state would
// allocate a copy of the room on every pawn that moved, which is the second
// performance rule in the overview -- no allocation in the hot path -- broken
// by the one function every hot path goes through.
//
// IT DOES NOT TOUCH seq. Tracking the sequence is the socket's job: it is what
// notices a gap and asks for a resync, and it is not part of the state a
// snapshot restores.
export function reduce(state: State, event: Event): void {
	switch (event.type) {
		// The snapshot is not a reduction, it is a replacement. Everything the
		// client held is discarded, which is exactly what a resync is for.
		case "snapshot":
			Object.assign(state, clone(event.state));
			break;

		// Singletons: assign the whole object.
		case "room.updated":
			state.room = clone(event.room);
			break;
		case "table.updated":
			state.table = clone(event.table);
			break;
		case "initiative.updated":
			state.initiative = clone(event.initiative);
			break;

		// Collection items: upsert or delete by id. Every one of these carries
		// the entity whole, so the upsert is one assignment and there is no
		// question of which fields the event meant to change.
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

		// Collections cleared by layer, which is a delete that names a
		// predicate rather than a list of ids.
		case "fog.cleared":
			state.fog = state.fog.filter((shape) => shape.layerId !== event.layer);
			break;
		case "stroke.cleared":
			state.strokes = state.strokes.filter((stroke) => stroke.layerId !== event.layer);
			break;
		case "stroke.erased":
			state.strokes = state.strokes.filter((stroke) => !event.ids.includes(stroke.id));
			break;

		// The hot paths. An id that is not here is ignored rather than an
		// error: a player's copy of a move can legitimately name pawns they
		// cannot see, because the filtering happens per audience and a race can
		// put an event and a removal in either order.
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

		// THE TRANSIENT EVENTS, LISTED RATHER THAN FILTERED, and the reason is
		// the line after them. A ping fades, a drag ghost is drawn and
		// forgotten, an error opens a dialog and a kick ends the session: none
		// of them is state. Naming them here rather than testing
		// TRANSIENT_EVENTS is what lets the default branch below be a `never`,
		// so an event added to the protocol and forgotten here fails the type
		// check instead of falling through at a table.
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

// normalize is the canonical form the server keeps its state in, so that a
// reduced state and a freshly projected one compare equal. Fog and initiative
// are deliberately left alone: in both of those the order IS the meaning -- a
// hide drawn over a reveal covers it, and the turn order is the order the GM
// dragged the lines into.
//
// SORTING BY THE ENCODED ULID IS SORTING BY ITS BYTES. Crockford base32 keeps
// the alphabet in ascending order, so comparing the twenty-six characters gives
// the same answer as comparing the sixteen bytes, which is what Go compares.
export function normalize(state: State): void {
	state.players.sort(byIdentifier);
	state.pawns.sort(byIdentifier);
	state.strokes.sort(byIdentifier);
}

// empty is what the client holds before its first snapshot: a room with nothing
// in it, so every reader can be written against a state that is always there.
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
			monsterHp: "band",
			playersCanDraw: true,
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

// clone keeps the state and the event that produced it from sharing an array. A
// stroke extended twice would otherwise append to the event's own points.
function clone<T>(value: T): T {
	return structuredClone(value);
}
