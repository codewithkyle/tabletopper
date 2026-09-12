import assert from "node:assert/strict";
import { test } from "node:test";
import type { FrameContext } from "../frame-context.ts";
import { empty } from "../../store.ts";
import { revisions } from "../../model/revisions.ts";
import { fakeDOM, recordingGL } from "./testing.ts";
import { newFrame } from "../frame-context.ts";
import { newOverlay } from "../../model/overlay.ts";
import { newStageList } from "./list.ts";
function scene(role: "gm" | "player"): { frame: FrameContext; list: ReturnType<typeof newStageList>; gl: ReturnType<typeof recordingGL> } {
	const gl = recordingGL();
	const list = newStageList(gl.gl, role, () => {});
	const state = empty();
	const frame = newFrame(
		gl.gl,
		{ x: 0, y: 0, zoom: 1 },
		{ width: 100, height: 100 },
		role,
		"01USER",
		state,
		revisions(),
		newOverlay(),
		list.resources(),
	);
	frame.rebuild = true;
	return { frame, list, gl };
}
test("every stage in the order can be built against a context", () => {
	const dom = fakeDOM();
	try {
		for (const role of ["gm", "player"] as const) {
			const { frame, list } = scene(role);
			assert.doesNotThrow(() => list.build(frame), role);
		}
	} finally {
		dom.restore();
	}
});
test("an empty room costs one draw call, and it is the grid", () => {
	const dom = fakeDOM();
	try {
		const { frame, list, gl } = scene("gm");
		list.build(frame);
		gl.reset();
		list.draw(frame);
		assert.deepEqual(gl.draws(), ["drawArrays"], "an empty scene did not cost exactly the grid");
	} finally {
		dom.restore();
	}
});
test("turning the grid off leaves an empty room costing nothing", () => {
	const dom = fakeDOM();
	try {
		const { frame, list, gl } = scene("gm");
		frame.state.table.grid.lines = "off";
		list.build(frame);
		gl.reset();
		list.draw(frame);
		assert.deepEqual(gl.draws(), []);
	} finally {
		dom.restore();
	}
});
test("nothing is settling when nothing is animating", () => {
	const dom = fakeDOM();
	try {
		const { frame, list } = scene("gm");
		list.build(frame);
		list.draw(frame);
		assert.equal(list.settling(frame), false);
	} finally {
		dom.restore();
	}
});
test("a ping keeps the frame loop awake until it fades", () => {
	const dom = fakeDOM();
	try {
		const { frame, list } = scene("gm");
		list.event({ type: "pinged", seq: 1, by: "01USER", layer: "", x: 0, y: 0 } as never);
		frame.now = performance.now();
		list.build(frame);
		list.draw(frame);
		assert.equal(list.settling(frame), true, "a fresh ping did not ask for another frame");
	} finally {
		dom.restore();
	}
});
test("the stage list reports what its stages fetched", () => {
	const dom = fakeDOM();
	try {
		const { list } = scene("gm");
		assert.equal(list.fetched(), 0);
	} finally {
		dom.restore();
	}
});
test("stage timings are off until they are asked for, and name every stage when they are", () => {
	const dom = fakeDOM();
	try {
		const { frame, list } = scene("gm");
		assert.ok(list.timings().every((timing) => timing.build === 0 && timing.draw === 0));
		list.timing(true);
		list.build(frame);
		list.draw(frame);
		const timings = list.timings();
		assert.equal(timings.length, 13, "a GM's order is thirteen stages");
		assert.ok(timings.every((timing) => timing.name !== ""), "a timing with no name says nothing");
		assert.equal(new Set(timings.map((timing) => timing.name)).size, timings.length);
	} finally {
		dom.restore();
	}
});
test("turning timing off clears what it measured, so a stale reading cannot be believed", () => {
	const dom = fakeDOM();
	try {
		const { frame, list } = scene("gm");
		list.timing(true);
		list.build(frame);
		list.draw(frame);
		list.timing(false);
		assert.ok(list.timings().every((timing) => timing.build === 0 && timing.draw === 0));
	} finally {
		dom.restore();
	}
});
