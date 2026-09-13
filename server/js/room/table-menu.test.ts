import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import type { Grid } from "./protocol.ts";
class El {
	attrs = new Map<string, string>();
	children: El[] = [];
	style: Record<string, string> = {};
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
		lines: "solid", cellSize: 64, offsetX: 0, offsetY: 0,
		color: "#000000ff", snap: "cells", feetPerCell: 5, diagonals: "equal",
	};
}
function room(): { mount: El; host: El; wheel: El; party: El; drawn: number[] } {
	const party = new El(
		"data-table-menu-party",
		"data-set-label=Party starts here",
		"data-clear-label=Take it back",
	);
	const wheel = new El("data-table-menu", "data-reach=60").append(party);
	const host = new El("data-table-menu-host").append(wheel);
	const mount = new El().append(host);
	return { mount, host, wheel, party, drawn: [] };
}
function menuFor(parts: ReturnType<typeof room>, start: { x: number; y: number } | null = null) {
	let drawn = 0;
	const menu = mountTableMenu(parts.mount as unknown as HTMLElement, {
		grid,
		viewed: () => "01FLOOR",
		partyStart: () => start,
		invalidate: () => {
			drawn++;
		},
	});
	return { menu, redraws: () => drawn };
}
test("a secondary click over empty ground opens the wheel on the cell under it", () => {
	const parts = room();
	const { menu } = menuFor(parts);
	assert.equal(menu.open({ x: 100, y: 40 }, { x: 400, y: 300 }), true);
	assert.equal(parts.wheel.hidden, false);
	const cell = menu.marked();
	assert.deepEqual(cell, { x: 64, y: 0, size: 64, centreX: 96, centreY: 32 });
	assert.equal(parts.wheel.style.transform, "translate(400px, 300px)");
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
		invalidate: () => {},
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
test("the module writes no class name", () => {
	const src = readFileSync(new URL("./table-menu.ts", import.meta.url), "utf8");
	for (const written of ["classList", "className", 'setAttribute("class"']) {
		assert.equal(src.includes(written), false, `${written} is in table-menu.ts`);
	}
});
test("the cell under a point is the one the grid offset says it is", () => {
	const offset = { ...grid(), offsetX: 10, offsetY: 10 };
	assert.deepEqual(cellUnder(offset, { x: 10, y: 10 }), {
		x: 10, y: 10, size: 64, centreX: 42, centreY: 42,
	});
});
