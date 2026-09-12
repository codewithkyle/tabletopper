import assert from "node:assert/strict";
import { test } from "node:test";
import {
	AGAIN_DRAGGING,
	AGAIN_FADING,
	AGAIN_STAGES,
	AGAIN_SWEEPING,
	AGAIN_TRAVELLING,
	AGAIN_UPLOADS,
	bit,
	reasonNames,
} from "./reasons.ts";
test("a frame that asked for no other is idle", () => {
	assert.deepEqual(reasonNames(0), []);
});
test("every reason a frame can ask for another has its own bit", () => {
	const flags = [
		AGAIN_STAGES,
		AGAIN_UPLOADS,
		AGAIN_DRAGGING,
		AGAIN_FADING,
		AGAIN_SWEEPING,
		AGAIN_TRAVELLING,
	];
	assert.equal(new Set(flags).size, flags.length);
	for (const flag of flags) {
		assert.equal(reasonNames(flag).length, 1, `${flag} should name exactly one reason`);
	}
});
test("a mask names every reason holding the loop open", () => {
	assert.deepEqual(reasonNames(AGAIN_STAGES | AGAIN_DRAGGING), ["stages", "dragging"]);
});
test("the names come out in the order the flags are declared", () => {
	assert.deepEqual(reasonNames(AGAIN_TRAVELLING | AGAIN_UPLOADS), ["uploads", "travelling"]);
});
test("bit contributes a flag only when its condition holds", () => {
	assert.equal(bit(true, AGAIN_FADING), AGAIN_FADING);
	assert.equal(bit(false, AGAIN_FADING), 0);
});
