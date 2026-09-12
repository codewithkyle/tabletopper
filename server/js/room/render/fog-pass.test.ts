import assert from "node:assert/strict";
import { test } from "node:test";
import type { FogShape } from "../protocol.ts";
import { createFogPass } from "./fog-pass.ts";
import { recordingGL } from "./stages/testing.ts";
const GROUND = "01LAYERGROUND";
const map = { width: 512, height: 512 };
function shape(id: string): FogShape {
	return { id, layerId: GROUND, kind: "rect", mode: "hide", points: [0, 0, 64, 64] };
}
function counts(gl: ReturnType<typeof recordingGL>, name: string): number {
	return gl.calls.filter((call) => call.name === name).length;
}
test("a second look at fog that has not moved paints nothing", () => {
	const gl = recordingGL();
	const pass = createFogPass(gl.gl);
	pass.sync([shape("01A")], GROUND, map, false, 64);
	gl.reset();
	pass.sync([shape("01A")], GROUND, map, false, 64);
	assert.equal(counts(gl, "drawArrays"), 0, "unchanged fog was painted again");
	assert.equal(counts(gl, "clear"), 0, "unchanged fog started the mask over");
});
test("one more shape is painted over the mask that is already there", () => {
	const gl = recordingGL();
	const pass = createFogPass(gl.gl);
	pass.sync([shape("01A")], GROUND, map, false, 64);
	gl.reset();
	pass.sync([shape("01A"), shape("01B")], GROUND, map, false, 64);
	assert.equal(counts(gl, "clear"), 0, "one more shape redrew the whole mask");
	assert.equal(counts(gl, "drawArrays"), 1);
});
test("a shape that is gone starts the mask over", () => {
	const gl = recordingGL();
	const pass = createFogPass(gl.gl);
	pass.sync([shape("01A"), shape("01B")], GROUND, map, false, 64);
	gl.reset();
	pass.sync([shape("01B")], GROUND, map, false, 64);
	assert.equal(counts(gl, "clear"), 1, "an erased shape did not start the mask over");
	assert.equal(counts(gl, "drawArrays"), 1);
});
test("a change of prefill starts the mask over", () => {
	const gl = recordingGL();
	const pass = createFogPass(gl.gl);
	pass.sync([shape("01A")], GROUND, map, false, 64);
	gl.reset();
	pass.sync([shape("01A")], GROUND, map, true, 64);
	assert.equal(counts(gl, "clear"), 1, "a change of prefill kept the mask it was painted into");
});
