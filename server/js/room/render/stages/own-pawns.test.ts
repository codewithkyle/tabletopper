import assert from "node:assert/strict";
import { test } from "node:test";
import type { Drawn } from "../../model/overlay.ts";
import type { FogShape, Layer, Pawn } from "../../protocol.ts";
import { aboveFog, concealed } from "../../model/polygon.ts";
import { stagesFor } from "./order.ts";
import { fogStage } from "./fog.ts";
import { ownPawnsStage } from "./own-pawns.ts";
import { pawnsStage } from "./pawns.ts";
import { visiblePawns } from "./pawns.ts";
const GROUND = "01LAYERGROUND";
const PLAYER = "01PLAYER";
const viewed: Layer = { id: GROUND, name: "Ground", map: null, fogEnabled: true, fogPrefill: true, partyStart: null };
const fog: FogShape[] = [{ id: "01CLEARED", layerId: GROUND, kind: "rect", mode: "reveal", points: [0, 0, 128, 128] }];
function pawn(over: Partial<Pawn> = {}): Pawn {
	return {
		id: "01PAWN", kind: "player", layerId: GROUND, name: "Ilya", image: "",
		x: 400, y: 400, z: 1, size: "medium", width: 0, height: 0, rotation: 0,
		visible: true, hp: 7, maxHp: 7, hpBand: null, ac: 15, conditions: [],
		ownerId: null, monsterId: null, characterId: null, ...over,
	};
}
function split(pawns: Pawn[]): { below: string[]; above: string[] } {
	const hidden = (p: Pawn): boolean =>
		concealed(p, fog, viewed, "player", PLAYER) || aboveFog(p, fog, viewed, "player", PLAYER);
	const elsewhere = (p: Pawn): boolean => !aboveFog(p, fog, viewed, "player", PLAYER);
	const below: Drawn[] = [];
	const above: Drawn[] = [];
	visiblePawns(pawns, GROUND, below, hidden);
	visiblePawns(pawns, GROUND, above, elsewhere);
	return { below: below.map((d) => d.id), above: above.map((d) => d.id) };
}
test("a player's own pawn in the dark is drawn once, over the fog", () => {
	const mine = pawn({ id: "mine", ownerId: PLAYER });
	const { below, above } = split([mine]);
	assert.deepEqual(below, [], "the token was also drawn under the fog that hides it");
	assert.deepEqual(above, ["mine"]);
});
test("a player's own pawn in the light stays with everybody else", () => {
	const mine = pawn({ id: "mine", x: 64, y: 64, ownerId: PLAYER });
	const { below, above } = split([mine]);
	assert.deepEqual(below, ["mine"]);
	assert.deepEqual(above, [], "a token in plain sight was lifted over the fog");
});
test("somebody else's pawn in the dark is drawn nowhere at all", () => {
	const theirs = pawn({ id: "theirs", ownerId: "01OTHER" });
	const { below, above } = split([theirs]);
	assert.deepEqual(below, []);
	assert.deepEqual(above, []);
});
test("every pawn lands in exactly one of the two passes", () => {
	const pawns = [
		pawn({ id: "mineDark", ownerId: PLAYER }),
		pawn({ id: "mineLight", x: 64, y: 64, ownerId: PLAYER }),
		pawn({ id: "theirsLight", x: 64, y: 64, ownerId: "01OTHER" }),
	];
	const { below, above } = split(pawns);
	assert.deepEqual([...below, ...above].sort(), ["mineDark", "mineLight", "theirsLight"]);
	assert.equal(below.filter((id) => above.includes(id)).length, 0, "a pawn was drawn twice");
});
test("the extra pass exists only for players, and only after the fog", () => {
	const player = stagesFor("player");
	assert.ok(player.indexOf(ownPawnsStage) > player.indexOf(fogStage));
	assert.ok(player.indexOf(pawnsStage) < player.indexOf(fogStage));
	assert.equal(stagesFor("gm").includes(ownPawnsStage), false, "the GM got a pass it has no use for");
});
