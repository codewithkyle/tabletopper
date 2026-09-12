import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import type { Frame, State } from "./protocol.ts";
import { reduce } from "./store.ts";
interface Step {
	name: string;
	frames: Frame[];
	state: State;
}
interface Fixture {
	role: string;
	initial: State;
	steps: Step[];
}
const fixtures = join(import.meta.dirname, "..", "..", "internal", "room", "testdata", "reducer");
function read(role: string): Fixture {
	return JSON.parse(readFileSync(join(fixtures, `${role}.json`), "utf8")) as Fixture;
}
function apply(state: State, frame: Frame): void {
	if (frame.type !== "changes") {
		reduce(state, frame);
		return;
	}
	for (const change of frame.events) {
		reduce(state, change);
	}
}
for (const role of ["gm", "player"]) {
	test(`the ${role} audience converges on the server's state`, () => {
		const fixture = read(role);
		assert.equal(fixture.role, role, "the fixture is for another audience");
		assert.ok(fixture.steps.length > 0, "the fixture has no steps");
		const state = structuredClone(fixture.initial);
		for (const [at, step] of fixture.steps.entries()) {
			for (const frame of step.frames) {
				apply(state, frame);
			}
			assert.deepStrictEqual(
				state,
				step.state,
				`step ${at + 1} (${step.name}) left the client's room different from the server's`,
			);
		}
	});
}
test("the fixtures between them exercise every event that changes state", () => {
	const seen = new Set<string>();
	for (const role of ["gm", "player"]) {
		for (const step of read(role).steps) {
			for (const frame of step.frames) {
				if (frame.type !== "changes") {
					continue;
				}
				for (const change of frame.events) {
					seen.add(change.type);
				}
			}
		}
	}
	const missing = [
		"room.updated",
		"table.updated",
		"layers.updated",
		"players.upserted",
		"players.removed",
		"pawns.upserted",
		"pawns.removed",
		"pawns.moved",
		"initiative.updated",
		"fog.upserted",
		"fog.removed",
		"strokes.upserted",
		"strokes.removed",
		"strokes.extended",
		"strokes.ended",
	].filter((type) => !seen.has(type));
	assert.deepStrictEqual(missing, [], "the scenario never produced these events");
});
test("every command that changed the room is one frame per audience", () => {
	for (const role of ["gm", "player"]) {
		for (const step of read(role).steps) {
			const carried = step.frames.filter((frame) => frame.type === "changes");
			assert.ok(
				carried.length <= 1,
				`step ${step.name} sent the ${role} ${carried.length} frames for one command`,
			);
		}
	}
});
