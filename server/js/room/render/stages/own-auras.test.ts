import assert from "node:assert/strict";
import { test } from "node:test";
import type { AuraPass } from "../aura-pass.ts";
import type { FogShape, Initiative, Layer, Pawn } from "../../protocol.ts";
import type { FrameContext } from "../frame-context.ts";
import { aurasStage } from "./auras.ts";
import { fillAuras } from "./auras.ts";
import { fogStage } from "./fog.ts";
import { lifted } from "./lifted.ts";
import { ownAurasStage } from "./own-auras.ts";
import { ownPawnsStage } from "./own-pawns.ts";
import { stagesFor } from "./order.ts";
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
function turn(ids: string[]): Initiative {
	return { entries: [{ id: "01LINE", pawnIds: ids, name: "Party", initiative: 20 }], active: "01LINE", round: 1 };
}
function frameFor(pawns: Pawn[], active: string[]): FrameContext {
	return {
		state: { pawns, fog, initiative: turn(active), table: { grid: { cellSize: 64 } } },
		viewed,
		viewedID: GROUND,
		role: "player",
		user: PLAYER,
		cell: 64,
	} as unknown as FrameContext;
}
function counter(): { pass: AuraPass; count: number } {
	const seen = { pass: null as unknown as AuraPass, count: 0 };
	seen.pass = { add: () => { seen.count++; } } as unknown as AuraPass;
	return seen;
}
function split(pawns: Pawn[], active: string[]): { below: number; above: number; glowing: boolean } {
	const frame = frameFor(pawns, active);
	const low = counter();
	const high = counter();
	const a = fillAuras(low.pass, frame, (p) => !lifted(frame, p));
	const b = fillAuras(high.pass, frame, (p) => lifted(frame, p));
	return { below: low.count, above: high.count, glowing: a || b };
}
test("the turn aura on a player's own pawn in the dark comes up over the fog", () => {
	const { below, above } = split([pawn({ id: "mine", ownerId: PLAYER })], ["mine"]);
	assert.equal(below, 0, "the aura was also drawn under the fog");
	assert.equal(above, 1);
});
test("an aura on a pawn standing in the light stays where it was", () => {
	const { below, above } = split([pawn({ id: "mine", x: 64, y: 64, ownerId: PLAYER })], ["mine"]);
	assert.equal(below, 1);
	assert.equal(above, 0);
});
test("nobody else's aura is lifted out of the dark", () => {
	const { below, above } = split([pawn({ id: "theirs", ownerId: "01OTHER" })], ["theirs"]);
	assert.equal(above, 0, "another player's aura was lifted over the fog");
	assert.equal(below, 1, "the aura left the pass it belongs to");
});
test("a pawn that is not on the count glows nowhere", () => {
	const { below, above, glowing } = split([pawn({ id: "mine", ownerId: PLAYER })], ["somebodyElse"]);
	assert.equal(below + above, 0);
	assert.equal(glowing, false);
});
test("a lifted aura still asks for the next frame, so it keeps turning", () => {
	assert.equal(split([pawn({ id: "mine", ownerId: PLAYER })], ["mine"]).glowing, true);
});
test("an aura is drawn once, never in both passes", () => {
	const pawns = [
		pawn({ id: "mineDark", ownerId: PLAYER }),
		pawn({ id: "mineLight", x: 64, y: 64, ownerId: PLAYER }),
	];
	const { below, above } = split(pawns, ["mineDark", "mineLight"]);
	assert.equal(below + above, 2);
	assert.equal(above, 1);
});
test("the lifted aura sits under its lifted pawn, as the grounded one does", () => {
	const player = stagesFor("player");
	assert.ok(player.indexOf(ownAurasStage) > player.indexOf(fogStage));
	assert.ok(player.indexOf(ownAurasStage) < player.indexOf(ownPawnsStage), "the aura was drawn over its own token");
	assert.ok(player.indexOf(aurasStage) < player.indexOf(fogStage));
	assert.equal(stagesFor("gm").includes(ownAurasStage), false);
});
