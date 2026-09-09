// Which button the pill lights, and the field the space bar is not taken in.
//
// THE TWO RULES HERE ARE THE TWO THAT ARE SILENT WHEN THEY BREAK. A hold that
// forgot what was chosen underneath it leaves a GM in Move mode with no way to
// tell why the pointer stopped selecting; a space bar taken from a text field
// puts a room in panning mode every time somebody types a goblin's name.

import assert from "node:assert/strict";
import { test } from "node:test";

import { showing } from "./tools.ts";
import { typing } from "./keys.ts";

// THE HOLD IS ON TOP OF THE CHOICE AND NOT INSTEAD OF IT, which is what lets
// the bar be let go of. What was chosen is still chosen the whole time.
test("the space bar shows the panning tool and lets go of it", () => {
	assert.equal(showing("select", "move", false), "select");
	assert.equal(showing("select", "move", true), "move");
	assert.equal(showing("draw", "move", true), "move");
	assert.equal(showing("draw", "move", false), "draw");
});

// A page with no panning tool rendered is not a page where the space bar means
// something else. It means nothing.
test("with no panning tool the space bar shows what was already there", () => {
	assert.equal(showing("select", null, true), "select");
	assert.equal(showing("select", null, false), "select");
});

// node has no HTMLElement to build one of, and typing duck-types its target for
// exactly that reason. This is the shape it reads and nothing else.
function target(tagName: string, isContentEditable = false): EventTarget {
	return { tagName, isContentEditable } as unknown as EventTarget;
}

// The space bar belongs to whatever has the caret, and the table hears every
// key on the document.
test("a key in a field is not a key on the table", () => {
	assert.equal(typing(target("INPUT")), true);
	assert.equal(typing(target("TEXTAREA")), true);
	assert.equal(typing(target("SELECT")), true);
	assert.equal(typing(target("DIV", true)), true);

	assert.equal(typing(target("DIV")), false);
	assert.equal(typing(target("BUTTON")), false);
	assert.equal(typing(null), false);
});
