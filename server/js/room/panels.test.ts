import assert from "node:assert/strict";
import { test } from "node:test";
import type { Event, InitiativeEntry, Pawn, State } from "./protocol.ts";
const target = new EventTarget();
(globalThis as unknown as { window: EventTarget }).window = target;
const { announce } = await import("./panels.ts");
function snapshot(entries: InitiativeEntry[]): Event {
	return {
		type: "snapshot",
		seq: 1,
		state: { initiative: { entries, active: null, round: 0 } } as State,
		you: { id: "01ME", role: "gm" },
		version: "test",
	};
}
function entry(id: string, ...pawns: string[]): InitiativeEntry {
	return { id, pawnIds: pawns, name: id, initiative: 0 };
}
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
test("pawn.removed raises both room:pawn and window:close", () => {
	const seen = raised({ type: "pawn.removed", seq: 7, id: "01GOBLIN" });
	assert.deepEqual(seen.map((e) => e.name).sort(), ["room:pawn", "window:close"]);
	assert.deepEqual(
		seen.find((e) => e.name === "window:close")?.detail,
		{ id: "pawn:01GOBLIN" },
	);
});
test("the three hot paths raise nothing at all", () => {
	const positions = [{ id: "01GOBLIN", x: 64, y: 64 }];
	assert.deepEqual(raised({ type: "pawn.moved", seq: 8, pawns: positions }), []);
	assert.deepEqual(raised({ type: "pawn.dragging", seq: 8, pawns: positions }), []);
	assert.deepEqual(raised({ type: "stroke.extended", seq: 9, id: "01STROKE", points: [0, 0] }), []);
});
test("a snapshot raises every panel event and no pawn event", () => {
	const seen = raised(snapshot([]));
	assert.deepEqual(seen.map((e) => e.name).sort(), [
		"room:info",
		"room:initiative",
		"room:players",
		"room:tabletop",
	]);
});
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
test("a removed pawn the tracker names refetches the strip", () => {
	raised(snapshot([entry("01ENTRY", "01GOBLIN")]));
	const seen = raised({ type: "pawn.removed", seq: 8, id: "01GOBLIN" });
	assert.deepEqual(seen.map((e) => e.name).sort(), [
		"room:initiative",
		"room:pawn",
		"window:close",
	]);
});
