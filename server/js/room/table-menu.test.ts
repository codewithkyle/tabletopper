import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import type { Grid } from "./protocol.ts";
class El {
	attrs = new Map<string, string>();
	children: El[] = [];
	style: Record<string, string> = {};
	textContent = "";
	hidden = true;
	clientWidth = 1000;
	clientHeight = 800;
	parent: El | null = null;
	private listeners = new Map<string, ((e: unknown) => void)[]>();
	constructor(...attrs: string[]) {
		for (const name of attrs) {
			const [key, value] = name.split("=");
			this.attrs.set(key, value ?? "");
		}
	}
	setAttribute(name: string, value: string): void {
		this.attrs.set(name, value);
	}
	getAttribute(name: string): string | null {
		return this.attrs.get(name) ?? null;
	}
	hasAttribute(name: string): boolean {
		return this.attrs.has(name);
	}
	append(...kids: El[]): El {
		for (const kid of kids) {
			kid.parent = this;
		}
		this.children.push(...kids);
		return this;
	}
	closest(selector: string): El | null {
		const name = selector.slice(1, -1);
		for (let at: El | null = this; at; at = at.parent) {
			if (at.hasAttribute(name)) {
				return at;
			}
		}
		return null;
	}
	contains(node: El): boolean {
		for (let at: El | null = node; at; at = at.parent) {
			if (at === this) {
				return true;
			}
		}
		return false;
	}
	querySelector(selector: string): El | null {
		return this.querySelectorAll(selector)[0] ?? null;
	}
	querySelectorAll(selector: string): El[] {
		const name = selector.slice(1, -1);
		const found: El[] = [];
		for (const kid of this.children) {
			if (kid.hasAttribute(name)) {
				found.push(kid);
			}
			found.push(...kid.querySelectorAll(selector));
		}
		return found;
	}
	addEventListener(type: string, fn: (e: unknown) => void): void {
		this.listeners.set(type, [...(this.listeners.get(type) ?? []), fn]);
	}
	removeEventListener(type: string, fn: (e: unknown) => void): void {
		this.listeners.set(type, (this.listeners.get(type) ?? []).filter((f) => f !== fn));
	}
	fire(type: string, event: Record<string, unknown> = {}): void {
		for (const fn of [...(this.listeners.get(type) ?? [])]) {
			fn({ type, ...event });
		}
	}
}
const doc = new El();
Object.assign(globalThis, { HTMLElement: El, Element: El, Node: El, document: doc });
const { mountTableMenu, cellUnder } = await import("./table-menu.ts");
function grid(): Grid {
	return {
		type: "square", lines: "solid", cellSize: 64, offsetX: 0, offsetY: 0,
		color: "#000000ff", snap: "cells", feetPerCell: 5, units: "feet", diagonals: "equal", numbered: false,
	};
}
function room(): {
	mount: El; host: El; wheel: El; party: El; erase: El; note: El;
	pine: El; picture: El; drawn: number[];
} {
	const party = new El(
		"data-table-menu-party",
		"data-set-label=Party starts here",
		"data-clear-label=Take it back",
	);
	const erase = new El("data-table-menu-erase");
	const note = new El("data-table-menu-note");
	const picture = new El();
	const pine = new El(
		"data-table-menu-art=01ARTPINE",
		"data-image=/pine.webp",
	).append(picture);
	const wheel = new El("data-table-menu", "data-reach=60").append(pine, erase, note, party);
	const host = new El("data-table-menu-host").append(wheel);
	const mount = new El().append(host);
	return { mount, host, wheel, party, erase, note, pine, picture, drawn: [] };
}
function menuFor(
	parts: ReturnType<typeof room>,
	start: { x: number; y: number } | null = null,
	scale = 1,
	board: () => Grid = grid,
) {
	let drawn = 0;
	const sent: Record<string, unknown>[] = [];
	const opened: { q: number; r: number }[] = [];
	const menu = mountTableMenu(parts.mount as unknown as HTMLElement, {
		grid: board,
		viewed: () => "01FLOOR",
		partyStart: () => start,
		scale: () => scale,
		invalidate: () => {
			drawn++;
		},
		send: (command) => {
			sent.push(command as unknown as Record<string, unknown>);
		},
		note: (q, r) => {
			opened.push({ q, r });
		},
	});
	return { menu, redraws: () => drawn, sent, opened };
}
test("a secondary click over empty ground opens the wheel on the cell under it", () => {
	const parts = room();
	const { menu } = menuFor(parts);
	assert.equal(menu.open({ x: 100, y: 40 }, { x: 400, y: 300 }), true);
	assert.equal(parts.wheel.hidden, false);
	const cell = menu.marked();
	assert.deepEqual(cell, { x: 64, y: 0, size: 64, centreX: 96, centreY: 32, q: 1, r: 0 });
	assert.equal(
		parts.wheel.style.transform,
		"translate(396px, 292px)",
		"the wheel opened under the pointer rather than over the cell",
	);
	menu.stop();
});
test("the wheel opens over the cell's centre however far in the table is zoomed", () => {
	const parts = room();
	const { menu } = menuFor(parts, null, 4);
	menu.open({ x: 100, y: 40 }, { x: 400, y: 300 });
	assert.equal(parts.wheel.style.transform, "translate(399px, 298px)");
	menu.stop();
});
test("the wheel shifts inward where the pointer is nearer an edge than its reach", () => {
	const parts = room();
	const { menu } = menuFor(parts);
	menu.open({ x: 0, y: 0 }, { x: 4, y: 790 });
	assert.equal(parts.wheel.style.transform, "translate(60px, 740px)");
	menu.stop();
});
test("the cell stays marked while the wheel is open and goes when it closes", () => {
	const parts = room();
	const { menu } = menuFor(parts);
	menu.open({ x: 10, y: 10 }, { x: 300, y: 300 });
	assert.notEqual(menu.marked(), null);
	menu.close();
	assert.equal(menu.marked(), null);
	assert.equal(parts.wheel.hidden, true);
	menu.stop();
});
test("escape, an outside press, a scroll and a pick all close it", () => {
	for (const shut of [
		() => doc.fire("keydown", { key: "Escape" }),
		() => doc.fire("pointerdown", { target: new El() }),
		() => doc.fire("wheel", {}),
	]) {
		const parts = room();
		const { menu } = menuFor(parts);
		menu.open({ x: 10, y: 10 }, { x: 300, y: 300 });
		shut();
		assert.equal(menu.marked(), null);
		assert.equal(parts.wheel.hidden, true);
		menu.stop();
	}
	const parts = room();
	const { menu } = menuFor(parts);
	menu.open({ x: 10, y: 10 }, { x: 300, y: 300 });
	doc.fire("click", { target: parts.party });
	assert.equal(menu.marked(), null);
	menu.stop();
});
test("a press inside the wheel leaves it open for the button underneath", () => {
	const parts = room();
	const { menu } = menuFor(parts);
	menu.open({ x: 10, y: 10 }, { x: 300, y: 300 });
	doc.fire("pointerdown", { target: parts.party });
	assert.notEqual(menu.marked(), null);
	menu.stop();
});
test("a swap inside the host drops the mark rather than leaving it drawn", () => {
	const parts = room();
	const { menu } = menuFor(parts);
	menu.open({ x: 10, y: 10 }, { x: 300, y: 300 });
	doc.fire("htmx:after:swap", { target: parts.wheel });
	assert.equal(menu.marked(), null);
	menu.stop();
});
test("party starts here carries the centre of the cell, not the pointer", () => {
	const parts = room();
	const { menu } = menuFor(parts);
	menu.open({ x: 100, y: 40 }, { x: 400, y: 300 });
	assert.equal(
		parts.party.getAttribute("hx-vals"),
		JSON.stringify({ layer: "01FLOOR", x: 96, y: 32 }),
	);
	menu.stop();
});
test("the cell the party already starts on offers to take it back instead", () => {
	const parts = room();
	const { menu } = menuFor(parts, { x: 96, y: 32 });
	menu.open({ x: 100, y: 40 }, { x: 400, y: 300 });
	assert.equal(
		parts.party.getAttribute("hx-vals"),
		JSON.stringify({ layer: "01FLOOR" }),
	);
	assert.equal(parts.party.getAttribute("data-tip"), "Take it back");
	assert.equal(parts.party.getAttribute("aria-label"), "Take it back");
	menu.close();
	menu.open({ x: 200, y: 40 }, { x: 400, y: 300 });
	assert.equal(
		parts.party.getAttribute("hx-vals"),
		JSON.stringify({ layer: "01FLOOR", x: 224, y: 32 }),
	);
	assert.equal(parts.party.getAttribute("data-tip"), "Party starts here");
	menu.stop();
});
test("a party start set under an older grid still matches the cell it lands in", () => {
	const parts = room();
	const { menu } = menuFor(parts, { x: 70, y: 10 });
	menu.open({ x: 100, y: 40 }, { x: 400, y: 300 });
	assert.equal(parts.party.getAttribute("hx-vals"), JSON.stringify({ layer: "01FLOOR" }));
	menu.stop();
});
test("a player has no wheel to open", () => {
	const host = new El("data-table-menu-host");
	const mount = new El().append(host);
	const menu = mountTableMenu(mount as unknown as HTMLElement, {
		grid,
		viewed: () => "01FLOOR",
		partyStart: () => null,
		scale: () => 1,
		invalidate: () => {},
		send: () => {},
	});
	assert.equal(menu.open({ x: 10, y: 10 }, { x: 300, y: 300 }), false);
	assert.equal(menu.marked(), null);
	menu.stop();
});
test("opening and closing asks for a redraw", () => {
	const parts = room();
	const { menu, redraws } = menuFor(parts);
	menu.open({ x: 10, y: 10 }, { x: 300, y: 300 });
	assert.equal(redraws(), 1);
	menu.close();
	assert.equal(redraws(), 2);
	menu.close();
	assert.equal(redraws(), 2);
	menu.stop();
});
test("picking a picture stamps the cell the wheel was opened on", () => {
	const parts = room();
	const { menu, sent } = menuFor(parts);
	menu.open({ x: 100, y: 40 }, { x: 400, y: 300 });
	doc.fire("click", { target: parts.picture });
	assert.deepEqual(sent, [{
		type: "tiles.stamp",
		layer: "01FLOOR",
		art: "01ARTPINE",
		rotation: 0,
		cells: [{ q: 1, r: 0 }],
	}]);
	assert.equal(menu.marked(), null, "the wheel stayed open after a pick");
	menu.stop();
});
test("hovering a picture previews it on the cell the wheel was opened on", () => {
	const parts = room();
	const { menu } = menuFor(parts);
	menu.open({ x: 100, y: 40 }, { x: 400, y: 300 });
	assert.equal(menu.preview(), null, "something was previewed before anything was hovered");
	doc.fire("pointerover", { target: parts.picture });
	assert.deepEqual(menu.preview(), { image: "/pine.webp", q: 1, r: 0, rotation: 0 });
	doc.fire("pointerover", { target: parts.erase });
	assert.equal(menu.preview(), null, "the preview outlived the picture under the pointer");
	menu.stop();
});
test("a preview belongs to an open wheel and goes with it", () => {
	const parts = room();
	const { menu } = menuFor(parts);
	menu.open({ x: 100, y: 40 }, { x: 400, y: 300 });
	doc.fire("pointerover", { target: parts.picture });
	menu.close();
	assert.equal(menu.preview(), null);
	menu.stop();
});
test("the brackets turn the preview by the step the cells have", () => {
	const parts = room();
	const { menu } = menuFor(parts);
	menu.open({ x: 100, y: 40 }, { x: 400, y: 300 });
	doc.fire("pointerover", { target: parts.picture });
	doc.fire("keydown", { key: "]" });
	assert.equal(menu.preview()?.rotation, 90);
	doc.fire("keydown", { key: "]" });
	doc.fire("keydown", { key: "]" });
	doc.fire("keydown", { key: "]" });
	assert.equal(menu.preview()?.rotation, 0, "four quarter turns is a whole one");
	doc.fire("keydown", { key: "[" });
	assert.equal(menu.preview()?.rotation, 270);
	menu.stop();
});
test("a picture is stamped at the turn the preview was showing", () => {
	const parts = room();
	const { menu, sent } = menuFor(parts);
	menu.open({ x: 100, y: 40 }, { x: 400, y: 300 });
	doc.fire("pointerover", { target: parts.picture });
	doc.fire("keydown", { key: "]" });
	doc.fire("click", { target: parts.picture });
	assert.equal(sent[0].rotation, 90);
	menu.stop();
});
test("the turn outlives the wheel, so a row goes down the same way up", () => {
	const parts = room();
	const { menu, sent } = menuFor(parts);
	menu.open({ x: 100, y: 40 }, { x: 400, y: 300 });
	doc.fire("keydown", { key: "]" });
	doc.fire("click", { target: parts.picture });
	menu.open({ x: 200, y: 40 }, { x: 400, y: 300 });
	doc.fire("click", { target: parts.picture });
	assert.deepEqual(sent.map((c) => (c as { rotation: number }).rotation), [90, 90]);
	menu.stop();
});
test("the brackets do nothing while the wheel is shut", () => {
	const parts = room();
	const { menu, sent } = menuFor(parts);
	doc.fire("keydown", { key: "]" });
	menu.open({ x: 100, y: 40 }, { x: 400, y: 300 });
	doc.fire("click", { target: parts.picture });
	assert.equal(sent[0].rotation, 0, "a key pressed over the table turned a wheel that was not open");
	menu.stop();
});
test("a turn carried over from a square grid is snapped to what a hex can do", () => {
	const parts = room();
	let type: Grid["type"] = "square";
	const { menu, sent } = menuFor(parts, null, 1, () => ({ ...grid(), type }));
	menu.open({ x: 100, y: 40 }, { x: 400, y: 300 });
	doc.fire("keydown", { key: "]" });
	menu.close();
	type = "hexPointy";
	menu.open({ x: 100, y: 40 }, { x: 400, y: 300 });
	doc.fire("pointerover", { target: parts.picture });
	assert.equal(menu.preview()?.rotation, 120, "a quarter turn was sent to a grid that turns in sixths");
	doc.fire("click", { target: parts.picture });
	assert.equal(sent[0].rotation, 120);
	menu.stop();
});
test("on a hex grid the brackets turn in sixths", () => {
	const parts = room();
	const { menu } = menuFor(parts, null, 1, () => ({ ...grid(), type: "hexPointy" }));
	menu.open({ x: 100, y: 40 }, { x: 400, y: 300 });
	doc.fire("pointerover", { target: parts.picture });
	doc.fire("keydown", { key: "]" });
	assert.equal(menu.preview()?.rotation, 60);
	doc.fire("keydown", { key: "[" });
	doc.fire("keydown", { key: "[" });
	assert.equal(menu.preview()?.rotation, 300);
	menu.stop();
});
test("erase takes the cell back rather than stamping over it", () => {
	const parts = room();
	const { menu, sent } = menuFor(parts);
	menu.open({ x: 100, y: 40 }, { x: 400, y: 300 });
	doc.fire("click", { target: parts.erase });
	assert.deepEqual(sent, [{
		type: "tiles.erase",
		layer: "01FLOOR",
		cells: [{ q: 1, r: 0 }],
	}]);
	menu.stop();
});
test("a pick with no cell remembered sends nothing", () => {
	const parts = room();
	const { menu, sent } = menuFor(parts);
	doc.fire("click", { target: parts.picture });
	assert.deepEqual(sent, []);
	menu.stop();
});
test("the module writes no class name", () => {
	const src = readFileSync(new URL("./table-menu.ts", import.meta.url), "utf8");
	for (const written of ["classList", "className", 'setAttribute("class"', "textContent"]) {
		assert.equal(src.includes(written), false, `${written} is in table-menu.ts`);
	}
});
test("the cell under a point is the one the grid offset says it is", () => {
	const offset = { ...grid(), offsetX: 10, offsetY: 10 };
	assert.deepEqual(cellUnder(offset, { x: 10, y: 10 }), {
		x: 10, y: 10, size: 64, centreX: 42, centreY: 42, q: 0, r: 0,
	});
});

test("the hex note opens on the cell the wheel was opened over", () => {
	const parts = room();
	const { menu, sent, opened } = menuFor(parts);
	menu.open({ x: 100, y: 40 }, { x: 400, y: 300 });
	doc.fire("click", { target: parts.note });
	assert.deepEqual(opened, [{ q: 1, r: 0 }]);
	assert.deepEqual(sent, [], "picking the hex note sent a command to the table");
	assert.equal(parts.wheel.hidden, true, "the wheel stayed open behind the note");
});
test("the hex note does nothing with no cell remembered", () => {
	const parts = room();
	const { menu, opened } = menuFor(parts);
	menu.close();
	doc.fire("click", { target: parts.note });
	assert.deepEqual(opened, []);
});
