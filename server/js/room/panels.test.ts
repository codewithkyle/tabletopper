import assert from "node:assert/strict";
import { test } from "node:test";
import type { Change, Frame, InitiativeEntry, Pawn, State } from "./protocol.ts";
const target = new EventTarget();
(globalThis as unknown as { window: EventTarget }).window = target;
const { announce } = await import("./panels.ts");
function snapshot(entries: InitiativeEntry[]): Frame {
	return {
		type: "snapshot",
		seq: 1,
		state: { initiative: { entries, active: null, round: 0 } } as State,
		you: { id: "01ME", role: "gm" },
		version: "test",
	};
}
function changes(...events: Change[]): Frame {
	return { type: "changes", seq: 4, events };
}
function entry(id: string, ...pawns: string[]): InitiativeEntry {
	return { id, pawnIds: pawns, name: id, initiative: 0 };
}
function raised(frame: Frame): { name: string; detail: unknown }[] {
	const seen: { name: string; detail: unknown }[] = [];
	const names = [
		"room:players",
		"room:initiative",
		"room:info",
		"room:tabletop",
		"room:pawn",
		"room:rolls",
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
	announce(frame);
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
test("an upserted pawn raises room:pawn carrying the pawn's own id", () => {
	const seen = raised(changes({ type: "pawns.upserted", pawns: [pawn("01GOBLIN", "Goblin")] }));
	const changed = seen.filter((e) => e.name === "room:pawn");
	assert.equal(changed.length, 1);
	assert.deepEqual(changed[0]?.detail, { id: "01GOBLIN" });
});
test("an upserted pawn also retitles that pawn's window", () => {
	const seen = raised(changes({ type: "pawns.upserted", pawns: [pawn("01GOBLIN", "Goblin chief")] }));
	const retitled = seen.filter((e) => e.name === "window:retitle");
	assert.equal(retitled.length, 1);
	assert.deepEqual(retitled[0]?.detail, { id: "pawn:01GOBLIN", title: "Goblin chief" });
});
test("a removed pawn raises both room:pawn and window:close", () => {
	const seen = raised(changes({ type: "pawns.removed", ids: ["01GOBLIN"] }));
	assert.deepEqual(seen.map((e) => e.name).sort(), ["room:pawn", "window:close"]);
	assert.deepEqual(
		seen.find((e) => e.name === "window:close")?.detail,
		{ id: "pawn:01GOBLIN" },
	);
});
test("one frame carrying three pawns raises room:pawn for each of them", () => {
	const seen = raised(changes({
		type: "pawns.upserted",
		pawns: [pawn("01A"), pawn("01B"), pawn("01C")],
	}));
	assert.deepEqual(
		seen.filter((e) => e.name === "room:pawn").map((e) => e.detail),
		[{ id: "01A" }, { id: "01B" }, { id: "01C" }],
	);
});
test("a frame refetches each panel once however many events ask for it", () => {
	const seen = raised(changes(
		{ type: "players.upserted", players: [] },
		{ type: "players.upserted", players: [] },
		{ type: "players.removed", ids: ["01GONE"] },
		{ type: "layers.updated", layers: [] },
		{ type: "table.updated", table: {} as never },
	));
	assert.deepEqual(seen.map((e) => e.name).sort(), ["room:players", "room:tabletop"]);
});
test("three pawns the tracker names refetch the strip once, not three times", () => {
	raised(snapshot([entry("01ENTRY", "01A", "01B", "01C")]));
	const seen = raised(changes({
		type: "pawns.upserted",
		pawns: [pawn("01A"), pawn("01B"), pawn("01C")],
	}));
	assert.equal(seen.filter((e) => e.name === "room:pawn").length, 3);
	assert.equal(seen.filter((e) => e.name === "room:initiative").length, 1);
});
test("the three hot paths raise nothing at all", () => {
	const positions = [{ id: "01GOBLIN", x: 64, y: 64 }];
	assert.deepEqual(raised(changes({ type: "pawns.moved", pawns: positions })), []);
	assert.deepEqual(raised({ type: "pawn.dragging", seq: 8, pawns: positions }), []);
	assert.deepEqual(
		raised(changes({ type: "strokes.extended", id: "01STROKE", points: [0, 0] })),
		[],
	);
});
test("a snapshot raises every panel event and no pawn event", () => {
	const seen = raised(snapshot([]));
	assert.deepEqual(seen.map((e) => e.name).sort(), [
		"room:info",
		"room:initiative",
		"room:players",
		"room:rolls",
		"room:tabletop",
	]);
});
test("a pawn the tracker names refetches the strip as well as its own window", () => {
	raised(snapshot([entry("01ENTRY", "01GOBLIN", "01OGRE")]));
	const seen = raised(changes({ type: "pawns.upserted", pawns: [pawn("01GOBLIN")] }));
	assert.deepEqual(seen.map((e) => e.name).sort(), [
		"room:initiative",
		"room:pawn",
		"window:retitle",
	]);
});
test("a pawn the tracker does not name refetches only its own window", () => {
	raised(snapshot([entry("01ENTRY", "01GOBLIN")]));
	const seen = raised(changes({ type: "pawns.upserted", pawns: [pawn("01WAGON")] }));
	assert.deepEqual(seen.map((e) => e.name).sort(), ["room:pawn", "window:retitle"]);
});
test("initiative.updated refreshes which pawns the tracker names", () => {
	raised(snapshot([entry("01ENTRY", "01GOBLIN")]));
	raised(changes({
		type: "initiative.updated",
		initiative: { entries: [entry("01ENTRY", "01OGRE")], active: null, round: 1 },
	}));
	assert.deepEqual(
		raised(changes({ type: "pawns.upserted", pawns: [pawn("01GOBLIN")] })).map((e) => e.name).sort(),
		["room:pawn", "window:retitle"],
	);
	assert.deepEqual(
		raised(changes({ type: "pawns.upserted", pawns: [pawn("01OGRE")] })).map((e) => e.name).sort(),
		["room:initiative", "room:pawn", "window:retitle"],
	);
});
test("a removed pawn the tracker names refetches the strip", () => {
	raised(snapshot([entry("01ENTRY", "01GOBLIN")]));
	const seen = raised(changes({ type: "pawns.removed", ids: ["01GOBLIN"] }));
	assert.deepEqual(seen.map((e) => e.name).sort(), [
		"room:initiative",
		"room:pawn",
		"window:close",
	]);
});

test("a shared roll repaints the dice tray", () => {
	const seen = raised(changes({ type: "rolls.upserted", rolls: [] } as unknown as Change));
	assert.deepEqual(seen.map((e) => e.name), ["room:rolls"]);
});
test("a secret roll repaints the tray of the one person told about it", () => {
	const seen = raised({ type: "rolled", seq: 9, roll: {} } as unknown as Frame);
	assert.deepEqual(seen.map((e) => e.name), ["room:rolls"]);
});
