














import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";

import type { Event, State } from "./protocol.ts";
import { reduce } from "./store.ts";

interface Step {
	name: string;
	events: Event[];
	state: State;
}

interface Fixture {
	role: string;
	initial: State;
	steps: Step[];
}

const fixtures = join(import.meta.dirname, "..", "..", "internal", "room", "testdata", "reducer");

for (const role of ["gm", "player"]) {
	test(`the ${role} audience converges on the server's state`, () => {
		const fixture = JSON.parse(readFileSync(join(fixtures, `${role}.json`), "utf8")) as Fixture;

		assert.equal(fixture.role, role, "the fixture is for another audience");
		assert.ok(fixture.steps.length > 0, "the fixture has no steps");

		const state = structuredClone(fixture.initial);

		for (const [at, step] of fixture.steps.entries()) {
			for (const event of step.events) {
				reduce(state, event);
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
		const fixture = JSON.parse(readFileSync(join(fixtures, `${role}.json`), "utf8")) as Fixture;
		for (const step of fixture.steps) {
			for (const event of step.events) {
				seen.add(event.type);
			}
		}
	}

	const missing = [
		"room.updated",
		"table.updated",
		"player.joined",
		"player.updated",
		"player.left",
		"pawn.spawned",
		"pawn.updated",
		"pawn.removed",
		"pawn.moved",
		"initiative.updated",
		"fog.added",
		"fog.removed",
		"fog.cleared",
		"stroke.began",
		"stroke.extended",
		"stroke.ended",
		"stroke.erased",
		"stroke.cleared",
	].filter((type) => !seen.has(type));

	assert.deepStrictEqual(missing, [], "the scenario never produced these events");
});
