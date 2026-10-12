import assert from "node:assert/strict";
import { test } from "node:test";
import { whereToOpen } from "./window.ts";
const HOME = "/fragment/character/sheet?room=01ROOM";
test("a window reopens where its content last said it was", () => {
	const spells = `${HOME}&section=spells&level=3`;
	assert.equal(whereToOpen(HOME, spells), spells);
});
test("a window with nothing remembered opens on the url it was asked for", () => {
	assert.equal(whereToOpen(HOME, null), HOME);
});
test("a remembered place that is not a fragment is never loaded", () => {
	for (const saved of ["/characters/01CHAR/edit", "https://elsewhere.example/fragment/x", 42, { path: HOME }, ""]) {
		assert.equal(whereToOpen(HOME, saved), HOME, `opened ${JSON.stringify(saved)}`);
	}
});
