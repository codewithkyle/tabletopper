import assert from "node:assert/strict";
import { test } from "node:test";
import { NOTE_WINDOW, noteWindow, noteWindowID } from "./note-window.ts";
const ROOM = "01BX5ZZKBKACTAV9WEVGEMMVRY";
const LAYER = "01BX5ZZKBKACTAV9WEVGEMMVT7";
test("a hex window is keyed by its cell and asks for that cell's fragment", () => {
	const spec = noteWindow(ROOM, LAYER, 3, -2, "3, -2");
	assert.equal(spec.id, noteWindowID(LAYER, 3, -2));
	assert.ok(spec.id.startsWith(NOTE_WINDOW));
	assert.equal(spec.url, `/fragment/room/hex?room=${ROOM}&layer=${LAYER}&q=3&r=-2`);
});
test("a hex window is titled the way the table refers to the cell", () => {
	assert.equal(noteWindow(ROOM, LAYER, 3, -2, "3, -2").title, "Hex 3, -2");
	assert.equal(noteWindow(ROOM, LAYER, 3, -2, "47").title, "Hex 47", "a numbered table does not name the hex by its number");
});
