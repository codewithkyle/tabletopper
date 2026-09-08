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

import type { Event, Pawn } from "./protocol.ts";

// panels.ts dispatches on the global window, which Node does not have. An
// EventTarget is the whole of the interface it uses, so it is the whole of what
// is stood up here -- and the import has to follow the assignment, which is
// what makes it dynamic.
const target = new EventTarget();
(globalThis as unknown as { window: EventTarget }).window = target;

const { announce } = await import("./panels.ts");

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

function pawn(id: string, name: string): Pawn {
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
	const seen = raised({
		type: "snapshot",
		seq: 1,
		state: null as never,
		you: { id: "01ME", role: "gm" },
		version: "test",
	});

	assert.deepEqual(seen.map((e) => e.name).sort(), [
		"room:info",
		"room:initiative",
		"room:players",
		"room:tabletop",
	]);
});
