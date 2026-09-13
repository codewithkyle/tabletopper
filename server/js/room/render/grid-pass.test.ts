import assert from "node:assert/strict";
import { test } from "node:test";
import type { Grid } from "../protocol.ts";
import type { Call } from "./stages/testing.ts";
import { createGridPass } from "./grid-pass.ts";
import { recordingGL } from "./stages/testing.ts";
function grid(over: Partial<Grid> = {}): Grid {
	return {
		type: "square",
		lines: "solid",
		cellSize: 64,
		offsetX: 0,
		offsetY: 0,
		color: "#000000FF",
		snap: "cells",
		feetPerCell: 5,
		units: "feet",
		diagonals: "equal",
		...over,
	};
}
const frame = { clipInverse: new Float32Array(9) } as never;
function uniform(gl: ReturnType<typeof recordingGL>, name: string): unknown[] | null {
	let found: Call | null = null;
	for (const call of gl.calls) {
		const at = call.args[0] as { uniform?: string } | null;
		if (call.name.startsWith("uniform") && at && at.uniform === name) {
			found = call;
		}
	}
	return found ? found.args.slice(1) : null;
}
test("the grid pass tells the shader which grid it is drawing", () => {
	for (const [type, want] of [["square", 0], ["hexPointy", 1], ["hexFlat", 2]] as const) {
		const gl = recordingGL();
		const pass = createGridPass(gl.gl);
		pass.draw(frame, grid({ type }));
		assert.deepEqual(uniform(gl, "u_type"), [want], `${type} asked for ${JSON.stringify(uniform(gl, "u_type"))}`);
	}
});
test("a square grid still wraps its origin by one cell", () => {
	const gl = recordingGL();
	const pass = createGridPass(gl.gl);
	pass.draw(frame, grid({ offsetX: 200, offsetY: -40 }));
	assert.deepEqual(uniform(gl, "u_offset"), [8, 24]);
});
test("a hex grid wraps its origin by the lattice and not by one cell", () => {
	const tall = Math.round(64 * Math.sqrt(3) * 1e6) / 1e6;
	const pointy = recordingGL();
	createGridPass(pointy.gl).draw(frame, grid({ type: "hexPointy", offsetX: 0, offsetY: 0 }));
	const [px, py] = uniform(pointy, "u_offset") as number[];
	assert.equal(px, 32);
	assert.ok(py === 32 && py < tall, `a pointy origin wrapped to ${py}`);
	const flat = recordingGL();
	createGridPass(flat.gl).draw(frame, grid({ type: "hexFlat", offsetX: 500, offsetY: 500 }));
	const [fx, fy] = uniform(flat, "u_offset") as number[];
	assert.ok(fx >= 0 && fx < tall, `a flat origin wrapped across to ${fx}`);
	assert.ok(fy >= 0 && fy < 64, `a flat origin wrapped down to ${fy}`);
});
test("a grid with no lines is not drawn at all", () => {
	const gl = recordingGL();
	createGridPass(gl.gl).draw(frame, grid({ lines: "off" }));
	assert.equal(gl.draws().length, 0);
});
