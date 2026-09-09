// What the socket raises on the DOM, and -- just as much -- what it does not.
//
// TWO RULES ARE PINNED HERE AND BOTH ARE ABOUT COST.
//
// THE FIRST IS THAT A PAWN EVENT CARRIES ITS ID. Ten pawn windows are open and
// one goblin takes damage; every window hears room:pawn and the filter in its
// own hx-trigger compares the id, so nine decline and one refetches. Without
// the id in the detail there is no filter to write, and one goblin taking
// damage is ten GETs, every round, for the whole fight.
//
// THE SECOND IS THAT THE THREE HOT PATHS RAISE NOTHING AT ALL. pawn.moved,
// pawn.dragging and stroke.extended fire up to twenty times a second. Binding a
// refetch to one of them is precisely what "the socket carries JSON and the DOM
// refetches" was designed to avoid, and it is the kind of line somebody adds in
// good faith while making a panel live. The assertion is written down where it
// would be broken.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { Event, InitiativeEntry, Pawn, State } from "./protocol.ts";

// panels.ts dispatches on the global window, which Node does not have. An
// EventTarget is the whole of the interface it uses, so it is the whole of what
// is stood up here -- and the import has to follow the assignment, which is
// what makes it dynamic.
const target = new EventTarget();
(globalThis as unknown as { window: EventTarget }).window = target;

const { announce } = await import("./panels.ts");

// snapshot is the one event that carries a whole state, and it is the one place
// the tracker's membership is refreshed from -- so the state cannot be a stub
// here the way it can be for every other event. What panels.ts reads out of it
// is the initiative entries and nothing else.
function snapshot(entries: InitiativeEntry[]): Event {
	return {
		type: "snapshot",
		seq: 1,
		state: { initiative: { entries, active: null, round: 0 } } as State,
		you: { id: "01ME", role: "gm" },
		version: "test",
	};
}

// entry is one line of the tracker as the set-building code reads it.
function entry(id: string, ...pawns: string[]): InitiativeEntry {
	return { id, pawnIds: pawns, name: id, initiative: 0 };
}

// raised collects the name and detail of everything one call dispatches.
function raised(event: Event): { name: string; detail: unknown }[] {
	const seen: { name: string; detail: unknown }[] = [];
	const names = [
		"room:players",
		"room:initiative",
		"room:info",
		"room:tabletop",
		"room:pawn",
		"window:close",
		"window:retitle",
	];

	const listeners = names.map((name) => {
		const listener = (e: globalThis.Event) => {
			seen.push({ name, detail: (e as CustomEvent).detail });
		};
		target.addEventListener(name, listener);

		return () => target.removeEventListener(name, listener);
	});

	announce(event);
	for (const off of listeners) {
		off();
	}

	return seen;
}

function pawn(id: string, name = "Goblin"): Pawn {
	return {
		id,
		kind: "monster",
		layerId: "01LAYER",
		name,
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
	};
}

test("pawn.updated raises room:pawn carrying the pawn's own id", () => {
	const seen = raised({ type: "pawn.updated", seq: 4, pawn: pawn("01GOBLIN", "Goblin") });

	const changed = seen.filter((e) => e.name === "room:pawn");
	assert.equal(changed.length, 1);
	assert.deepEqual(changed[0]?.detail, { id: "01GOBLIN" });
});

// A rename reaches the panel's body by refetch and would never reach the
// window's title bar, which is set from the trigger that opened it.
test("pawn.updated also retitles that pawn's window", () => {
	const seen = raised({ type: "pawn.updated", seq: 5, pawn: pawn("01GOBLIN", "Goblin chief") });

	const retitled = seen.filter((e) => e.name === "window:retitle");
	assert.equal(retitled.length, 1);
	assert.deepEqual(retitled[0]?.detail, { id: "pawn:01GOBLIN", title: "Goblin chief" });
});

test("pawn.spawned raises room:pawn as well, for a pawn that came back into view", () => {
	const seen = raised({ type: "pawn.spawned", seq: 6, pawn: pawn("01GUARD", "Guard") });

	assert.deepEqual(
		seen.filter((e) => e.name === "room:pawn").map((e) => e.detail),
		[{ id: "01GUARD" }],
	);
});

// The fragment 404s from here on and the page's noSwap config covers 4xx, so
// without this the window sits there showing a dead goblin's hit points
// forever. The close has to come from outside the fragment, because the
// fragment is what stopped existing.
test("pawn.removed raises both room:pawn and window:close", () => {
	const seen = raised({ type: "pawn.removed", seq: 7, id: "01GOBLIN" });

	assert.deepEqual(seen.map((e) => e.name).sort(), ["room:pawn", "window:close"]);
	assert.deepEqual(
		seen.find((e) => e.name === "window:close")?.detail,
		{ id: "pawn:01GOBLIN" },
	);
});

// THE HOT-PATH RULE. These three are the only messages in the protocol that
// fire at the rate of a hand moving, and none of them may reach the DOM.
test("the three hot paths raise nothing at all", () => {
	const positions = [{ id: "01GOBLIN", x: 64, y: 64 }];

	assert.deepEqual(raised({ type: "pawn.moved", seq: 8, pawns: positions }), []);
	assert.deepEqual(raised({ type: "pawn.dragging", seq: 8, pawns: positions }), []);
	assert.deepEqual(raised({ type: "stroke.extended", seq: 9, id: "01STROKE", points: [0, 0] }), []);
});

// A snapshot changes everything at once, which is the first join and every
// resync -- exactly when a panel drawn from a stale fetch would be wrong.
test("a snapshot raises every panel event and no pawn event", () => {
	const seen = raised(snapshot([]));

	assert.deepEqual(seen.map((e) => e.name).sort(), [
		"room:info",
		"room:initiative",
		"room:players",
		"room:tabletop",
	]);
});

// THE STRIP WEARS THE CREATURE'S WOUNDS AND DAMAGE ARRIVES AS pawn.updated. A
// line of the turn order is a portrait with blood on it, and the blood is read
// off hit points that change through the pawn family -- which raises room:pawn
// with an id and nothing else. Without the set below, the strip would be stale
// until the turn advanced.
test("a pawn the tracker names refetches the strip as well as its own window", () => {
	raised(snapshot([entry("01ENTRY", "01GOBLIN", "01OGRE")]));

	const seen = raised({ type: "pawn.updated", seq: 4, pawn: pawn("01GOBLIN") });

	assert.deepEqual(seen.map((e) => e.name).sort(), [
		"room:initiative",
		"room:pawn",
		"window:retitle",
	]);
});

test("a pawn the tracker does not name refetches only its own window", () => {
	raised(snapshot([entry("01ENTRY", "01GOBLIN")]));

	const seen = raised({ type: "pawn.updated", seq: 4, pawn: pawn("01WAGON") });

	assert.deepEqual(seen.map((e) => e.name).sort(), ["room:pawn", "window:retitle"]);
});

// AND THE SET IS REBUILT RATHER THAN ADDED TO. A goblin taken out of the order
// should stop costing a refetch every time it is hit, which is the half of this
// that a growing set would get wrong.
test("initiative.updated refreshes which pawns the tracker names", () => {
	raised(snapshot([entry("01ENTRY", "01GOBLIN")]));

	raised({
		type: "initiative.updated",
		seq: 5,
		initiative: { entries: [entry("01ENTRY", "01OGRE")], active: null, round: 1 },
	});

	assert.deepEqual(
		raised({ type: "pawn.updated", seq: 6, pawn: pawn("01GOBLIN") }).map((e) => e.name).sort(),
		["room:pawn", "window:retitle"],
	);
	assert.deepEqual(
		raised({ type: "pawn.updated", seq: 7, pawn: pawn("01OGRE") }).map((e) => e.name).sort(),
		["room:initiative", "room:pawn", "window:retitle"],
	);
});

// A REMOVAL IS A CHANGE TO THE STRIP TOO, because the line naming it is about to
// go and the fragment behind it is what says so.
test("a removed pawn the tracker names refetches the strip", () => {
	raised(snapshot([entry("01ENTRY", "01GOBLIN")]));

	const seen = raised({ type: "pawn.removed", seq: 8, id: "01GOBLIN" });

	assert.deepEqual(seen.map((e) => e.name).sort(), [
		"room:initiative",
		"room:pawn",
		"window:close",
	]);
});
