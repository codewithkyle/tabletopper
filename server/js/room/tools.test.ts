






import assert from "node:assert/strict";
import { test } from "node:test";

import { showing } from "./tools.ts";
import { typing } from "./keys.ts";



test("the space bar shows the panning tool and lets go of it", () => {
	assert.equal(showing("select", "move", false), "select");
	assert.equal(showing("select", "move", true), "move");
	assert.equal(showing("draw", "move", true), "move");
	assert.equal(showing("draw", "move", false), "draw");
});



test("with no panning tool the space bar shows what was already there", () => {
	assert.equal(showing("select", null, true), "select");
	assert.equal(showing("select", null, false), "select");
});



function target(tagName: string, isContentEditable = false): EventTarget {
	return { tagName, isContentEditable } as unknown as EventTarget;
}



test("a key in a field is not a key on the table", () => {
	assert.equal(typing(target("INPUT")), true);
	assert.equal(typing(target("TEXTAREA")), true);
	assert.equal(typing(target("SELECT")), true);
	assert.equal(typing(target("DIV", true)), true);

	assert.equal(typing(target("DIV")), false);
	assert.equal(typing(target("BUTTON")), false);
	assert.equal(typing(null), false);
});
