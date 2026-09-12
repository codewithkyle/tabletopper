import assert from "node:assert/strict";
import { test } from "node:test";
import { GM, PLAYER, at, cleared, pawn, table } from "./testing.ts";
const DARK = {
	role: "player", user: PLAYER, fogEnabled: true, fogPrefill: true,
} as const;
test("another player's drag draws ghosts until a move ends it", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, ghosts } = table([goblin]);
	controller.preview({
		type: "pawn.dragging", seq: 4, by: "01OTHER",
		pawns: [{ id: "goblin", x: 160, y: 160 }],
	});
	const drawn = ghosts();
	assert.equal(drawn.length, 1);
	assert.deepEqual([drawn[0].x, drawn[0].y], [160, 160]);
	controller.preview({
		type: "pawn.moved", seq: 5, pawns: [{ id: "goblin", x: 160, y: 160 }],
	});
	assert.deepEqual(ghosts(), []);
});
test("a client ignores its own dragging event", () => {
	const goblin = pawn({ id: "goblin", x: 32, y: 32 });
	const { controller, ghosts } = table([goblin]);
	controller.preview({
		type: "pawn.dragging", seq: 4, by: GM,
		pawns: [{ id: "goblin", x: 160, y: 160 }],
	});
	assert.deepEqual(ghosts(), []);
});
test("a player is not shown the path of a pawn the fog is hiding", () => {
	const goblin = pawn({ id: "goblin", x: 400, y: 400 });
	const { controller, segments, ghosts } = table([goblin], { ...DARK, fog: [cleared()] });
	controller.preview({
		type: "pawn.dragging", seq: 4, by: "01OTHER",
		pawns: [{ id: "goblin", x: 500, y: 500 }],
	});
	assert.deepEqual(segments(), [], "the move distance traced a pawn under the fog");
	assert.deepEqual(ghosts(), [], "a ghost was drawn for a pawn under the fog");
});
test("a player is still shown the path of a pawn standing in the clear", () => {
	const goblin = pawn({ id: "goblin", x: 64, y: 64 });
	const { controller, segments, ghosts } = table([goblin], { ...DARK, fog: [cleared()] });
	controller.preview({
		type: "pawn.dragging", seq: 4, by: "01OTHER",
		pawns: [{ id: "goblin", x: 96, y: 96 }],
	});
	assert.equal(segments().length, 1);
	assert.equal(ghosts().length, 1);
});
test("a drag of one's own draws a path across the squares it crossed", () => {
	const goblin = pawn({ id: "goblin", x: 0, y: 0 });
	const { controller, segments, labels, cells } = table([goblin]);
	controller.tool.press(at(0, 0), at(0, 0), { shift: false, alt: false });
	controller.tool.drag(at(192, 0), at(192, 0), { shift: false, alt: false });
	assert.equal(segments().length, 1, "the drag drew no ruler");
	assert.deepEqual(labels().map((l) => l.text), ["15 ft."]);
	assert.equal(cells().length, 4, "the squares between the start and the ghost were not tinted");
});
