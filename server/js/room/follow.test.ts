// The camera that follows the turn, and every test here is about the same two
// questions: was that actually a new turn, and is there anything on this screen
// to point at.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { Event, InitiativeEntry, Pawn, State } from "./protocol.ts";
import type { Rect } from "./render/camera.ts";
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

// A fight with a hero on one count and three goblins grouped on another. The
// cell is 64, so a medium creature is a 64 pixel box centred on its position.
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

// watched mounts a follow over a state and records every box it asked for.
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

// The events this module reads carry nothing it uses -- it reads the store the
// reducer has already written -- so the fixtures are the type and the shape and
// nothing else.
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
	// All three goblins, each a 64 pixel box: 100-32 to 300+32 across, and
	// 100-32 to 500+32 down.
	assert.deepEqual(seen.boxes[0], { x1: 68, y1: 68, x2: 332, y2: 532 });
});

// GROUPED COMBAT IS THE REASON THE BOX EXISTS. Nine goblins act on one count,
// so the turn belongs to all of them and a camera framed on the first would
// leave eight creatures whose turn it is off the screen.
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

// A SNAPSHOT IS NOT A TURN CHANGE. It arrives on the first join and on every
// reconnect, carrying the fight as it stands now -- so a tab that was asleep
// through three rounds would otherwise come back and yank the camera on the
// strength of a turn nobody here watched begin.
test("a snapshot records the turn without moving the camera", () => {
	const state = fight();
	const seen = watched(state);

	state.initiative.active = "01MOBLINE";
	seen.send(snapshot());

	assert.equal(seen.boxes.length, 0, "the reconnect moved the camera");

	// And it recorded, so the next turn is followed from the right place.
	state.initiative.active = "01HEROLINE";
	seen.send(updated());

	assert.equal(seen.boxes.length, 1);
});

// initiative.updated is raised by every change to the tracker. A line added, a
// line removed and a drag that reorders the strip are all that event, and none
// of them handed anybody the table.
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

// "Lair action" and "the volcano erupts" are lines in the order with no
// creature on the table, and there is nowhere to point a camera for one.
test("a line with no pawns moves nothing", () => {
	const state = fight();
	const seen = watched(state);

	seen.send(snapshot());

	state.initiative.entries.push(line("01LAIR", []));
	state.initiative.active = "01LAIR";
	seen.send(updated());

	assert.equal(seen.boxes.length, 0);
});

// The GM looking at the cellar is preparing the next room. Dragging them back
// to the ground floor because a goblin's turn came up would be a worse theft
// than the one this whole feature is trying to prevent.
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

// Everything else off the socket goes past. A pawn moving on somebody's turn is
// not a reason to re-frame it, or a GM tidying the board would drag every
// viewer's camera along behind the token.
test("nothing but the tracker is listened to", () => {
	const state = fight();
	const seen = watched(state);

	seen.send(snapshot());

	state.initiative.active = "01MOBLINE";
	seen.send({ type: "pawn.moved" } as unknown as Event);
	seen.send({ type: "table.updated" } as unknown as Event);

	assert.equal(seen.boxes.length, 0);
});

// AN OBJECT IS ITS PICTURE AND A CREATURE IS ITS CELLS, which is pawnExtents'
// rule and not this module's. What matters here is that the box asked for is
// the one the table actually draws, so a wagon on a count is framed as a wagon
// rather than as a 64 pixel square.
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

// An active id that names no line is a tracker mid-edit, not a crash.
test("an active id with no line behind it has no bounds", () => {
	const state = fight();

	state.initiative.active = "01GONE";

	assert.equal(actingBounds(state, GROUND), null);
});

// A HIDDEN PAWN IS IN THE BOX, and only the GM ever has one -- a player's
// events never carry them. An ambush that is acting is exactly what the GM
// needs to be looking at.
test("a hidden pawn is still framed", () => {
	const state = fight();

	state.pawns[1].visible = false;
	state.initiative.active = "01MOBLINE";

	const box = actingBounds(state, GROUND);

	assert.ok(box && box.x1 === 68, "the hidden ambusher was left out of the box");
});

// THE ACCOUNT SETTING. It is a switch on a module that is always mounted, and
// the next three tests are the reason it is not a module that is only mounted
// when the answer is yes.
test("a viewer who turned it off is not taken anywhere", () => {
	const state = fight();
	const seen = watched(state);

	seen.following(false);

	state.initiative.active = "01MOBLINE";
	seen.send(updated());

	assert.equal(seen.boxes.length, 0);
});

// THE ACTING LINE IS TRACKED WHILE IT IS OFF, which is the whole of it. Turning
// the setting on in the middle of somebody's go should do nothing until the
// NEXT turn -- a module mounted at that moment would find nothing recorded,
// read the turn in progress as news, and yank the camera onto a creature whose
// go began five minutes ago.
test("turning it on mid-turn does not chase the go already in progress", () => {
	const state = fight();
	const seen = watched(state);

	seen.following(false);

	state.initiative.active = "01MOBLINE";
	seen.send(updated());

	seen.following(true);

	// A condition added, a line renamed, a drag that reordered the strip: every
	// one of those is an initiative.updated and none of them moved the turn.
	seen.send(updated());

	assert.equal(seen.boxes.length, 0);
});

// And once it is on, the next real turn is followed like any other.
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
