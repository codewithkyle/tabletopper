import assert from "node:assert/strict";
import { test } from "node:test";
import type { FogShape, Layer, Pawn } from "../protocol.ts";
import { aboveFog, concealed, covered } from "./polygon.ts";
const GROUND = "01LAYERGROUND";
const PLAYER = "01PLAYER";
function layer(over: Partial<Layer> = {}): Layer {
	return { id: GROUND, name: "Ground", map: null, fogEnabled: true, fogPrefill: true, ...over };
}
function shape(): FogShape {
	return { id: "01CLEARED", layerId: GROUND, kind: "rect", mode: "reveal", points: [0, 0, 128, 128] };
}
function pawn(over: Partial<Pawn> = {}): Pick<Pawn, "x" | "y" | "ownerId"> {
	return { x: 400, y: 400, ownerId: null, ...over };
}
const FOG = [shape()];
test("covered asks only where the pawn stands, not whose it is", () => {
	assert.equal(covered(pawn(), FOG, layer()), true);
	assert.equal(covered(pawn({ x: 64, y: 64 }), FOG, layer()), false);
	assert.equal(covered(pawn(), FOG, layer({ fogEnabled: false })), false);
	assert.equal(covered(pawn(), FOG, null), false);
});
test("a pawn in the dark is concealed from a player who does not own it", () => {
	assert.equal(concealed(pawn(), FOG, layer(), "player", PLAYER), true);
});
test("a player's own pawn is never concealed, however dark the room", () => {
	assert.equal(concealed(pawn({ ownerId: PLAYER }), FOG, layer(), "player", PLAYER), false);
});
test("the GM is concealed from nothing", () => {
	assert.equal(concealed(pawn(), FOG, layer(), "gm", "01GM"), false);
});
test("a player's own pawn in the dark is the one thing drawn over the fog", () => {
	assert.equal(aboveFog(pawn({ ownerId: PLAYER }), FOG, layer(), "player", PLAYER), true);
});
test("a pawn standing in the light needs no lifting over the fog", () => {
	assert.equal(aboveFog(pawn({ x: 64, y: 64, ownerId: PLAYER }), FOG, layer(), "player", PLAYER), false);
});
test("somebody else's pawn is never lifted over the fog", () => {
	assert.equal(aboveFog(pawn({ ownerId: "01OTHER" }), FOG, layer(), "player", PLAYER), false);
	assert.equal(aboveFog(pawn(), FOG, layer(), "player", PLAYER), false);
});
test("the GM lifts nothing, because the GM's fog is see-through already", () => {
	assert.equal(aboveFog(pawn({ ownerId: "01GM" }), FOG, layer(), "gm", "01GM"), false);
});
test("concealed and aboveFog never both claim the same pawn", () => {
	const cases = [pawn(), pawn({ ownerId: PLAYER }), pawn({ x: 64, y: 64 }), pawn({ x: 64, y: 64, ownerId: PLAYER })];
	for (const p of cases) {
		const hidden = concealed(p, FOG, layer(), "player", PLAYER);
		const lifted = aboveFog(p, FOG, layer(), "player", PLAYER);
		assert.ok(!(hidden && lifted), `${JSON.stringify(p)} was both hidden and lifted`);
	}
});
