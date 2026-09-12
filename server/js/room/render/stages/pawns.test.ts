import assert from "node:assert/strict";
import { test } from "node:test";
import type { Drawn } from "../../model/overlay.ts";
import type { Pawn } from "../../protocol.ts";
import { visiblePawns } from "./pawns.ts";
const GROUND = "01LAYERGROUND";
const CELLAR = "01LAYERCELLAR";
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
test("only the viewed floor's pawns are drawn", () => {
	const pawns = [
		pawn({ id: "a", layerId: GROUND }),
		pawn({ id: "b", layerId: CELLAR }),
		pawn({ id: "c", layerId: GROUND }),
	];
	const out: Drawn[] = [];
	visiblePawns(pawns, GROUND, out);
	assert.deepEqual(out.map((p) => p.id), ["a", "c"]);
	visiblePawns(pawns, CELLAR, out);
	assert.deepEqual(out.map((p) => p.id), ["b"]);
	visiblePawns(pawns, "01LAYERATTIC", out);
	assert.deepEqual(out, []);
});
test("rebuilding writes into the same objects", () => {
	const out: Drawn[] = [];
	visiblePawns([pawn({ id: "a", x: 10 })], GROUND, out);
	const first = out[0];
	visiblePawns([pawn({ id: "a", x: 99 })], GROUND, out);
	assert.equal(out[0], first, "a rebuild allocated a fresh object");
	assert.equal(out[0].x, 99);
});
test("a pawn players cannot see is marked hidden", () => {
	const out: Drawn[] = [];
	visiblePawns([pawn({ visible: false })], GROUND, out);
	assert.equal(out[0].hidden, true);
});
test("health is whichever the viewer was told, and the number wins", () => {
	const out: Drawn[] = [];
	visiblePawns([pawn({ hp: 0 })], GROUND, out);
	assert.equal(out[0].health, "dead");
	visiblePawns([pawn({ hp: 1, maxHp: 7 })], GROUND, out);
	assert.equal(out[0].health, "veryBloody");
	visiblePawns([pawn({ hp: null, maxHp: null, hpBand: "dead" })], GROUND, out);
	assert.equal(out[0].health, "dead", "a player was not shown the band they were sent");
	visiblePawns([pawn({ hp: null, maxHp: null, hpBand: "nearDeath" })], GROUND, out);
	assert.equal(out[0].health, "nearDeath");
	visiblePawns([pawn({ hp: null, maxHp: null, hpBand: null })], GROUND, out);
	assert.equal(out[0].health, null, "health was invented out of nothing at all");
	visiblePawns([pawn({ hp: 4, maxHp: 7, hpBand: "dead" })], GROUND, out);
	assert.equal(out[0].health, "bruised");
});
