import assert from "node:assert/strict";
import { test } from "node:test";
import type { FrameContext } from "../frame-context.ts";
import type { Pawn } from "../../protocol.ts";
import type { Resources } from "../resources.ts";
import type { SpriteCache } from "../sprites.ts";
import { decalsStage } from "./decals.ts";
import { empty } from "../../store.ts";
import { fakeDOM, recordingGL } from "./testing.ts";
import { newFrame } from "../frame-context.ts";
import { newOverlay } from "../../model/overlay.ts";
import { revisions } from "../../model/revisions.ts";
const GROUND = "01LAYERGROUND";
function pawn(hp: number): Pawn {
	return {
		id: "01PAWN", kind: "monster", layerId: GROUND, name: "Goblin", image: "",
		x: 400, y: 400, z: 1, size: "medium", width: 0, height: 0, rotation: 0,
		visible: true, hp, maxHp: 20, hpBand: null, ac: 13, conditions: [],
		ownerId: null, monsterId: null, characterId: null,
	};
}
function stageResources(): Resources {
	return {
		sprites: {
			sprite: () => ({ layer: 0, w: 256, h: 256 }),
			texture: () => ({}),
		} as unknown as SpriteCache,
		invalidate: () => {},
	} as unknown as Resources;
}
function bleedingTable(): { gl: ReturnType<typeof recordingGL>; frame: FrameContext; stage: ReturnType<typeof decalsStage> } {
	const gl = recordingGL();
	const resources = stageResources();
	const stage = decalsStage(gl.gl, resources);
	const frame = newFrame(
		gl.gl, { x: 0, y: 0, zoom: 1 }, { width: 100, height: 100 },
		"gm", "01USER", empty(), revisions(), newOverlay(), resources,
	);
	frame.viewedID = GROUND;
	frame.cell = 64;
	frame.state.pawns = [pawn(20)];
	stage.build?.(frame);
	frame.state.pawns = [pawn(6)];
	stage.build?.(frame);
	return { gl, frame, stage };
}
function drawn(gl: ReturnType<typeof recordingGL>, frame: FrameContext, stage: ReturnType<typeof decalsStage>): string[] {
	stage.build?.(frame);
	gl.reset();
	stage.draw(frame);
	return gl.draws();
}
test("a hit leaves blood on the floor it was taken on", () => {
	const dom = fakeDOM();
	try {
		const { gl, frame, stage } = bleedingTable();
		assert.deepEqual(drawn(gl, frame, stage), ["drawArraysInstanced"]);
	} finally {
		dom.restore();
	}
});
test("clearing the tabletop washes the blood off every viewer's floor", () => {
	const dom = fakeDOM();
	try {
		const { gl, frame, stage } = bleedingTable();
		stage.event?.({ type: "table.cleared", seq: 4 });
		frame.state.pawns = [];
		assert.deepEqual(drawn(gl, frame, stage), [], "the floor still carries blood the GM cleared");
	} finally {
		dom.restore();
	}
});
