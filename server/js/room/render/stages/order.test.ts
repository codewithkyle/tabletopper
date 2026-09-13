import assert from "node:assert/strict";
import { test } from "node:test";
import { aurasStage } from "./auras.ts";
import { decalsStage } from "./decals.ts";
import { floorMarksStage } from "./floor-marks.ts";
import { fogStage } from "./fog.ts";
import { ghostsStage } from "./ghosts.ts";
import { gridStage } from "./grid.ts";
import { handlesStage } from "./handles.ts";
import { overMarksStage } from "./over-marks.ts";
import { partyStartStage } from "./party-start.ts";
import { ownPawnsStage } from "./own-pawns.ts";
import { ownAurasStage } from "./own-auras.ts";
import { ownRingsStage } from "./own-rings.ts";
import { pawnsStage } from "./pawns.ts";
import { pingsStage } from "./pings.ts";
import { ringsStage } from "./rings.ts";
import { nameOf, stagesFor } from "./order.ts";
import { numbersStage } from "./numbers.ts";
import { strokesStage } from "./strokes.ts";
import { terrainStage } from "./terrain.ts";
import { tilesStage } from "./tiles.ts";
const COMMON = [
	tilesStage, terrainStage, gridStage, decalsStage, strokesStage,
	floorMarksStage, aurasStage, pawnsStage, ringsStage, ghostsStage,
	handlesStage, pingsStage, overMarksStage,
];
test("a GM sees fog over the floor and under everything that stands on it", () => {
	assert.deepEqual(stagesFor("gm"), [
		tilesStage, terrainStage, gridStage, decalsStage, strokesStage,
		fogStage, numbersStage, partyStartStage,
		floorMarksStage, aurasStage, pawnsStage, ringsStage, ghostsStage,
		handlesStage, pingsStage,
		overMarksStage,
	]);
});
test("a player sees fog over the pawns it hides", () => {
	assert.deepEqual(stagesFor("player"), [
		tilesStage, terrainStage, gridStage, decalsStage, strokesStage,
		floorMarksStage, aurasStage, pawnsStage, ringsStage, ghostsStage,
		handlesStage, pingsStage,
		fogStage, ownAurasStage, ownPawnsStage, ownRingsStage,
		overMarksStage,
	]);
});
test("fog is one stage, drawn once, wherever the role puts it", () => {
	for (const role of ["gm", "player"] as const) {
		const order = stagesFor(role);
		assert.equal(order.filter((stage) => stage === fogStage).length, 1, role);
		assert.equal(order.length, role === "player" ? 17 : 16, role);
	}
});
test("every other stage keeps its place whichever side of the table you are on", () => {
	for (const role of ["gm", "player"] as const) {
		const lifted = [fogStage, numbersStage, partyStartStage, ownAurasStage, ownPawnsStage, ownRingsStage];
		const order = stagesFor(role).filter((stage) => !lifted.includes(stage));
		assert.deepEqual(order, COMMON, role);
	}
});
test("terrain lies on the map and under everything drawn on it", () => {
	for (const role of ["gm", "player"] as const) {
		const order = stagesFor(role);
		assert.ok(order.indexOf(terrainStage) > order.indexOf(tilesStage), role + ": terrain is under the map");
		assert.ok(order.indexOf(terrainStage) < order.indexOf(gridStage), role + ": terrain is over the grid lines");
		assert.ok(order.indexOf(terrainStage) < order.indexOf(strokesStage), role + ": terrain is over the drawing");
		assert.ok(order.indexOf(terrainStage) < order.indexOf(fogStage), role + ": terrain is over the fog");
	}
});
test("marks bracket the pawns: cells under, labels over", () => {
	const order = stagesFor("gm");
	assert.ok(order.indexOf(floorMarksStage) < order.indexOf(pawnsStage));
	assert.ok(order.indexOf(overMarksStage) > order.indexOf(pawnsStage));
	assert.ok(order.indexOf(ghostsStage) > order.indexOf(pawnsStage), "a ghost is drawn over the pawn it came from");
});
test("every stage in the order has a name, because a timing readout of blanks says nothing", () => {
	const seen = new Set<string>();
	for (const role of ["gm", "player"] as const) {
		for (const stage of stagesFor(role)) {
			const name = nameOf(stage);
			assert.notEqual(name, "stage", `a stage in the ${role} order is unnamed`);
			seen.add(name);
		}
	}
	assert.equal(seen.size, 19, "two stages share a name, so their timings would be indistinguishable");
});
test("only the GM is handed cell numbers, and they sit over the fog that dims the map", () => {
	assert.ok(!stagesFor("player").includes(numbersStage), "a player was handed the GM's reference numbers");
	const order = stagesFor("gm");
	assert.ok(order.indexOf(numbersStage) > order.indexOf(fogStage), "the fog dims the numbers under it");
	assert.ok(order.indexOf(numbersStage) < order.indexOf(pawnsStage), "a number is drawn over the pawn standing on it");
});
test("only the GM is shown where the party starts", () => {
	assert.ok(!stagesFor("player").includes(partyStartStage), "a player was shown the GM's party start");
	const order = stagesFor("gm");
	assert.ok(order.indexOf(partyStartStage) > order.indexOf(fogStage), "the fog dims the mark under it");
	assert.ok(order.indexOf(partyStartStage) < order.indexOf(pawnsStage), "the mark is drawn over a pawn standing on it");
});
