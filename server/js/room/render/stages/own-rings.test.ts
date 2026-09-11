import assert from "node:assert/strict";
import { test } from "node:test";
import type { Condition, FogShape, Layer, Pawn } from "../../protocol.ts";
import type { FrameContext } from "../frame-context.ts";
import type { RingPass } from "../ring-pass.ts";
import { fillConditionRings } from "./rings.ts";
import { fogStage } from "./fog.ts";
import { ownPawnsStage } from "./own-pawns.ts";
import { ownRingsStage } from "./own-rings.ts";
import { ringsStage } from "./rings.ts";
import { stagesFor } from "./order.ts";
import { lifted } from "./lifted.ts";
const GROUND = "01LAYERGROUND";
const PLAYER = "01PLAYER";
const viewed: Layer = { id: GROUND, name: "Ground", map: null, fogEnabled: true, fogPrefill: true };
const fog: FogShape[] = [{ id: "01CLEARED", layerId: GROUND, kind: "rect", mode: "reveal", points: [0, 0, 128, 128] }];
function condition(): Condition {
	return { id: "01COND", name: "Poisoned", color: "green", duration: 0, clear: "end" };
}
function pawn(over: Partial<Pawn> = {}): Pawn {
	return {
		id: "01PAWN", kind: "player", layerId: GROUND, name: "Ilya", image: "",
		x: 400, y: 400, z: 1, size: "medium", width: 0, height: 0, rotation: 0,
		visible: true, hp: 7, maxHp: 7, hpBand: null, ac: 15, conditions: [condition()],
		ownerId: null, monsterId: null, characterId: null, ...over,
	};
}
function frameFor(pawns: Pawn[]): FrameContext {
	return {
		state: { pawns, fog, table: { grid: { cellSize: 64 } } },
		viewed,
		viewedID: GROUND,
		role: "player",
		user: PLAYER,
		cell: 64,
		worldPerDevicePixel: 1,
	} as unknown as FrameContext;
}
function counter(): { pass: RingPass; rings: number } {
	const seen = { pass: null as unknown as RingPass, rings: 0 };
	seen.pass = { add: () => { seen.rings++; } } as unknown as RingPass;
	return seen;
}
function split(pawns: Pawn[]): { below: number; above: number } {
	const frame = frameFor(pawns);
	const low = counter();
	const high = counter();
	fillConditionRings(low.pass, frame, (p) => !lifted(frame, p));
	fillConditionRings(high.pass, frame, (p) => lifted(frame, p));
	return { below: low.rings, above: high.rings };
}
test("a ring on a player's own pawn in the dark rides over the fog with it", () => {
	const { below, above } = split([pawn({ id: "mine", ownerId: PLAYER })]);
	assert.equal(below, 0, "the ring was also drawn under the fog");
	assert.equal(above, 1);
});
test("a ring on a pawn standing in the light stays where it was", () => {
	const { below, above } = split([pawn({ id: "mine", x: 64, y: 64, ownerId: PLAYER })]);
	assert.equal(below, 1);
	assert.equal(above, 0);
});
test("nobody else's rings are lifted, in the dark or out of it", () => {
	const dark = split([pawn({ id: "theirs", ownerId: "01OTHER" })]);
	assert.equal(dark.above, 0, "another player's ring was lifted over the fog");
	const light = split([pawn({ id: "theirs", x: 64, y: 64, ownerId: "01OTHER" })]);
	assert.equal(light.above, 0);
	assert.equal(light.below, 1);
});
test("a ring is drawn once, never in both passes", () => {
	const pawns = [
		pawn({ id: "mineDark", ownerId: PLAYER }),
		pawn({ id: "mineLight", x: 64, y: 64, ownerId: PLAYER }),
		pawn({ id: "theirsLight", x: 64, y: 64, ownerId: "01OTHER" }),
	];
	const { below, above } = split(pawns);
	assert.equal(below + above, 3, "a ring went missing or was drawn twice");
	assert.equal(above, 1);
});
test("every condition on a lifted pawn comes up with it", () => {
	const many = pawn({ id: "mine", ownerId: PLAYER, conditions: [condition(), condition(), condition()] });
	assert.equal(split([many]).above, 3);
});
test("the lifted rings follow the lifted pawn, and only players get either", () => {
	const player = stagesFor("player");
	assert.ok(player.indexOf(ownRingsStage) > player.indexOf(ownPawnsStage));
	assert.ok(player.indexOf(ownPawnsStage) > player.indexOf(fogStage));
	assert.ok(player.indexOf(ringsStage) < player.indexOf(fogStage), "the grounded rings left their place");
	assert.equal(stagesFor("gm").includes(ownRingsStage), false);
});
