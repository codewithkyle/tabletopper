import assert from "node:assert/strict";
import { test } from "node:test";
import type { FrameContext } from "../frame-context.ts";
import type { Grid, Layer } from "../../protocol.ts";
import type { Resources } from "../resources.ts";
import { empty } from "../../store.ts";
import { fakeDOM, recordingGL } from "./testing.ts";
import { newFrame } from "../frame-context.ts";
import { newOverlay } from "../../model/overlay.ts";
import { partyStartStage } from "./party-start.ts";
import { revisions } from "../../model/revisions.ts";
const GROUND = "01LAYERGROUND";
function grid(over: Partial<Grid> = {}): Grid {
	return {
		type: "square", lines: "solid", cellSize: 64, offsetX: 0, offsetY: 0,
		color: "#000000FF", snap: "cells", feetPerCell: 5, units: "feet", diagonals: "equal", numbered: false,
		...over,
	};
}
function floor(over: Partial<Layer> = {}): Layer {
	return {
		id: GROUND, name: "Ground floor", map: null, gmMap: null,
		fogEnabled: false, fogPrefill: true, partyStart: null, ...over,
	};
}
function startFrame(gl: WebGL2RenderingContext, viewed: Layer): FrameContext {
	const frame = newFrame(
		gl, { x: 0, y: 0, zoom: 1 }, { width: 100, height: 100 },
		"gm", "01USER", empty(), revisions(), newOverlay(),
		{ atlas: null } as unknown as Resources,
	);
	frame.state.table.grid = grid();
	frame.state.table.layers = [viewed];
	frame.viewed = viewed;
	frame.viewedID = viewed.id;
	return frame;
}
function drawnWith(viewed: Layer): string[] {
	const dom = fakeDOM();
	try {
		const gl = recordingGL();
		const stage = partyStartStage(gl.gl, { atlas: null } as unknown as Resources);
		const frame = startFrame(gl.gl, viewed);
		stage.build?.(frame);
		gl.reset();
		stage.draw(frame);
		return gl.draws();
	} finally {
		dom.restore();
	}
}
test("the floor the GM is looking at is marked where the party starts", () => {
	assert.deepEqual(drawnWith(floor({ partyStart: { x: 128, y: 64 } })), ["drawArraysInstanced"]);
});
test("a floor with nowhere for the party to start is not marked", () => {
	assert.deepEqual(drawnWith(floor()), [], "a mark was drawn for a floor that carries none");
});
