import assert from "node:assert/strict";
import { test } from "node:test";
import type { InitiativeEntry, Pawn, State } from "./protocol.ts";
import { deckLine, nameOf, onDeck, skipped, struck, upNext } from "./turn-alert.ts";
import { empty } from "./store.ts";
const ME = "01ME";
const GROUND = "01GROUND";
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
		hpBand: "healthy",
		ac: 15,
		conditions: [],
		ownerId: null,
		monsterId: null,
		characterId: null,
		...over,
	};
}
function line(id: string, pawnIds: string[], name = "Goblins"): InitiativeEntry {
	return { id, pawnIds, name, initiative: 12 };
}
function fight(active: string | null = "01WOLFLINE"): State {
	const state = empty();
	state.table.activeLayer = GROUND;
	state.pawns = [
		pawn({ id: "01THALIA", kind: "player", name: "Thalia", ownerId: ME }),
		pawn({ id: "01WOLF", name: "Dire Wolf" }),
		pawn({ id: "01GOB", name: "Goblin" }),
	];
	state.initiative = {
		entries: [
			line("01WOLFLINE", ["01WOLF"]),
			line("01THALIALINE", ["01THALIA"]),
			line("01GOBLINE", ["01GOB"]),
		],
		active,
		round: 1,
	};
	return state;
}
test("the next turn is the line after the active one", () => {
	assert.equal(upNext(fight())?.id, "01THALIALINE");
});
test("the next turn wraps round the bottom of the order", () => {
	assert.equal(upNext(fight("01GOBLINE"))?.id, "01WOLFLINE");
});
test("an empty tracker has no next turn", () => {
	assert.equal(upNext(empty()), null);
});
test("a dead monster is passed over, the way the server passes it over", () => {
	const state = fight();
	state.pawns[1] = pawn({ id: "01WOLF", name: "Dire Wolf", hpBand: "dead" });
	assert.ok(skipped(state, state.initiative.entries[0]));
	assert.equal(upNext(fight("01THALIALINE"))?.id, "01GOBLINE");
	state.initiative.active = "01GOBLINE";
	assert.equal(upNext(state)?.id, "01THALIALINE");
});
test("a dead player still gets their turn", () => {
	const state = fight();
	state.pawns[0] = pawn({ id: "01THALIA", kind: "player", name: "Thalia", ownerId: ME, hpBand: "dead" });
	assert.equal(skipped(state, state.initiative.entries[1]), false);
	assert.equal(upNext(state)?.id, "01THALIALINE");
});
test("a line with nobody behind it is a name, and a name takes its turn", () => {
	const state = fight();
	state.initiative.entries[1] = line("01THALIALINE", [], "Lair action");
	assert.equal(skipped(state, state.initiative.entries[1]), false);
});
test("a group is only passed over once every one of them is down", () => {
	const state = fight();
	state.pawns.push(pawn({ id: "01GOB2", name: "Goblin", hpBand: "dead" }));
	state.initiative.entries[2] = line("01GOBLINE", ["01GOB", "01GOB2"]);
	assert.equal(skipped(state, state.initiative.entries[2]), false);
	state.pawns[2] = pawn({ id: "01GOB", name: "Goblin", hpBand: "dead" });
	assert.ok(skipped(state, state.initiative.entries[2]));
});
test("being next up is being on deck, and it says who you follow", () => {
	assert.deepEqual(onDeck(fight(), ME), {
		entry: "01THALIALINE",
		name: "Thalia",
		after: "Dire Wolf",
	});
});
test("somebody else's turn coming up is not your alert", () => {
	assert.equal(onDeck(fight("01THALIALINE"), ME), null);
});
test("nothing is on deck until a turn is running", () => {
	assert.equal(onDeck(fight(null), ME), null);
});
test("nobody is on deck to a viewer with no seat", () => {
	assert.equal(onDeck(fight(), ""), null);
});
test("the only creature in the order is never on deck to itself", () => {
	const state = fight("01THALIALINE");
	state.initiative.entries = [line("01THALIALINE", ["01THALIA"], "Thalia")];
	assert.equal(onDeck(state, ME), null);
});
test("a dead monster between you and the turn does not hold the alert back", () => {
	const state = fight();
	state.initiative.entries = [
		line("01WOLFLINE", ["01WOLF"]),
		line("01GOBLINE", ["01GOB"]),
		line("01THALIALINE", ["01THALIA"]),
	];
	state.pawns[2] = pawn({ id: "01GOB", name: "Goblin", hpBand: "dead" });
	assert.equal(onDeck(state, ME)?.entry, "01THALIALINE");
});
test("the line names the pawn rather than whatever the entry was called", () => {
	const state = fight();
	state.initiative.entries[1] = line("01THALIALINE", ["01THALIA"], "Player 2");
	const deck = onDeck(state, ME);
	assert.ok(deck !== null);
	assert.equal(deckLine(deck), "Thalia — after Dire Wolf");
});
test("a line with nobody behind it is called what the tracker calls it", () => {
	const state = fight();
	assert.equal(nameOf(state, line("01LAIR", [], "Lair action")), "Lair action");
	assert.equal(nameOf(state, line("01GONE", ["01NOBODY"], "Ghost")), "Ghost");
});
test("the chime sounds once, when the entry on deck becomes yours", () => {
	const deck = { entry: "01THALIALINE", name: "Thalia", after: "Dire Wolf" };
	assert.ok(struck(null, deck, false));
	assert.equal(struck(deck.entry, deck, false), false);
	assert.equal(struck(null, null, false), false);
});
test("a resync never sounds the chime, however the order looks", () => {
	const deck = { entry: "01THALIALINE", name: "Thalia", after: "Dire Wolf" };
	assert.equal(struck(null, deck, true), false);
	assert.equal(struck("01GOBLINE", deck, true), false);
});
