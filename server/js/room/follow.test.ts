import assert from "node:assert/strict";
import { test } from "node:test";
import type { Event, InitiativeEntry, Pawn, State } from "./protocol.ts";
import type { Rect } from "./model/types.ts";
import { actingBounds, mountFollow } from "./follow.ts";
import { empty } from "./store.ts";
const GROUND = "01GROUND";
const CELLAR = "01CELLAR";
function pawn(over: Partial<Pawn> = {}): Pawn {
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
function line(id: string, pawnIds: string[]): InitiativeEntry {
	return { id, pawnIds, name: "Goblins", initiative: 12 };
}
function fight(): State {
	const state = empty();
	state.table.activeLayer = GROUND;
	state.pawns = [
		pawn({ id: "01HERO", kind: "player", x: 1000, y: 1000 }),
		pawn({ id: "01GOB1", x: 100, y: 100 }),
		pawn({ id: "01GOB2", x: 300, y: 100 }),
		pawn({ id: "01GOB3", x: 100, y: 500 }),
	];
	state.initiative = {
		entries: [line("01HEROLINE", ["01HERO"]), line("01MOBLINE", ["01GOB1", "01GOB2", "01GOB3"])],
		active: "01HEROLINE",
		round: 1,
	};
	return state;
}
function watched(state: State, viewed = GROUND): {
	boxes: Rect[];
	send(event: Event): void;
	following(on: boolean): void;
} {
	const boxes: Rect[] = [];
	const follow = mountFollow(state, {
		viewed: () => viewed,
		focus: (rect) => {
			boxes.push(rect);
		},
	});
	return {
		boxes,
		send(event) {
			follow.event(event);
		},
		following(on) {
			follow.following(on);
		},
	};
}
function updated(): Event {
	return { type: "initiative.updated", seq: 1, at: 0, initiative: { entries: [], active: null, round: 0 } } as unknown as Event;
}
function snapshot(): Event {
	return { type: "snapshot" } as unknown as Event;
}
test("the turn moving points the camera at whoever is up", () => {
	const state = fight();
	const seen = watched(state);
	seen.send(snapshot());
	state.initiative.active = "01MOBLINE";
	seen.send(updated());
	assert.equal(seen.boxes.length, 1);
	assert.deepEqual(seen.boxes[0], { x1: 68, y1: 68, x2: 332, y2: 532 });
});
test("a grouped line is boxed by every pawn in it", () => {
	const state = fight();
	const seen = watched(state);
	seen.send(snapshot());
	state.initiative.active = "01MOBLINE";
	seen.send(updated());
	const box = seen.boxes[0];
	for (const p of state.pawns.slice(1)) {
		assert.ok(box.x1 <= p.x && p.x <= box.x2, `${p.id} is off the box across`);
		assert.ok(box.y1 <= p.y && p.y <= box.y2, `${p.id} is off the box down`);
	}
});
test("a snapshot records the turn without moving the camera", () => {
	const state = fight();
	const seen = watched(state);
	state.initiative.active = "01MOBLINE";
	seen.send(snapshot());
	assert.equal(seen.boxes.length, 0, "the reconnect moved the camera");
	state.initiative.active = "01HEROLINE";
	seen.send(updated());
	assert.equal(seen.boxes.length, 1);
});
test("a tracker edited mid-turn moves nothing", () => {
	const state = fight();
	const seen = watched(state);
	seen.send(snapshot());
	state.initiative.entries.push(line("01LAIR", []));
	seen.send(updated());
	state.initiative.entries.reverse();
	seen.send(updated());
	assert.equal(seen.boxes.length, 0);
});
test("a line with no pawns moves nothing", () => {
	const state = fight();
	const seen = watched(state);
	seen.send(snapshot());
	state.initiative.entries.push(line("01LAIR", []));
	state.initiative.active = "01LAIR";
	seen.send(updated());
	assert.equal(seen.boxes.length, 0);
});
test("a turn on a floor nobody is looking at moves nothing", () => {
	const state = fight();
	const seen = watched(state, CELLAR);
	seen.send(snapshot());
	state.initiative.active = "01MOBLINE";
	seen.send(updated());
	assert.equal(seen.boxes.length, 0);
});
test("combat ending moves nothing", () => {
	const state = fight();
	const seen = watched(state);
	seen.send(snapshot());
	state.initiative = { entries: [], active: null, round: 0 };
	seen.send(updated());
	assert.equal(seen.boxes.length, 0);
});
test("nothing but the tracker is listened to", () => {
	const state = fight();
	const seen = watched(state);
	seen.send(snapshot());
	state.initiative.active = "01MOBLINE";
	seen.send({ type: "pawn.moved" } as unknown as Event);
	seen.send({ type: "table.updated" } as unknown as Event);
	assert.equal(seen.boxes.length, 0);
});
test("the box is the size the table draws, not the size of the grid", () => {
	const state = empty();
	state.table.activeLayer = GROUND;
	state.pawns = [pawn({ id: "01CART", kind: "object", x: 500, y: 500, width: 400, height: 120 })];
	state.initiative = { entries: [line("01CARTLINE", ["01CART"])], active: null, round: 1 };
	state.initiative.active = "01CARTLINE";
	assert.deepEqual(actingBounds(state, GROUND), { x1: 300, y1: 440, x2: 700, y2: 560 });
});
test("a tracker with nobody acting has no bounds", () => {
	const state = fight();
	state.initiative.active = null;
	assert.equal(actingBounds(state, GROUND), null);
});
test("an active id with no line behind it has no bounds", () => {
	const state = fight();
	state.initiative.active = "01GONE";
	assert.equal(actingBounds(state, GROUND), null);
});
test("a hidden pawn is still framed", () => {
	const state = fight();
	state.pawns[1].visible = false;
	state.initiative.active = "01MOBLINE";
	const box = actingBounds(state, GROUND);
	assert.ok(box && box.x1 === 68, "the hidden ambusher was left out of the box");
});
test("a viewer who turned it off is not taken anywhere", () => {
	const state = fight();
	const seen = watched(state);
	seen.following(false);
	state.initiative.active = "01MOBLINE";
	seen.send(updated());
	assert.equal(seen.boxes.length, 0);
});
test("turning it on mid-turn does not chase the go already in progress", () => {
	const state = fight();
	const seen = watched(state);
	seen.following(false);
	state.initiative.active = "01MOBLINE";
	seen.send(updated());
	seen.following(true);
	seen.send(updated());
	assert.equal(seen.boxes.length, 0);
});
test("the turn after it comes back is followed", () => {
	const state = fight();
	const seen = watched(state);
	seen.following(false);
	state.initiative.active = "01MOBLINE";
	seen.send(updated());
	seen.following(true);
	state.initiative.active = "01HEROLINE";
	seen.send(updated());
	assert.equal(seen.boxes.length, 1);
	assert.ok(seen.boxes[0].x1 === 968, "the camera did not frame the hero whose turn it now is");
});
