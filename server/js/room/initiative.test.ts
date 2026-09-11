

















import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";




class El {
	attrs = new Map<string, string>();
	children: El[] = [];
	textContent = "";
	scrolled = 0;
	pressed: string[] = [];

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

	removeAttribute(name: string): void {
		this.attrs.delete(name);
	}

	append(...kids: El[]): El {
		this.children.push(...kids);

		return this;
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

	closest(selector: string): El | null {
		return this.hasAttribute(selector.slice(1, -1)) ? this : null;
	}

	scrollIntoView(): void {
		this.scrolled++;
	}

	addEventListener(type: string, fn: (e: unknown) => void): void {
		this.listeners.set(type, [...(this.listeners.get(type) ?? []), fn]);
	}

	removeEventListener(type: string, fn: (e: unknown) => void): void {
		this.listeners.set(type, (this.listeners.get(type) ?? []).filter((f) => f !== fn));
	}

	dispatchEvent(event: { type: string }): void {
		this.pressed.push(event.type);
		for (const fn of this.listeners.get(event.type) ?? []) {
			fn(event);
		}
	}

	
	
	fire(type: string, event: Record<string, unknown> = {}): void {
		for (const fn of this.listeners.get(type) ?? []) {
			fn({ type, ...event });
		}
	}
}

const doc = new El();

Object.assign(globalThis, {
	HTMLElement: El,
	Element: El,
	MouseEvent: class {
		type: string;
		constructor(type: string) {
			this.type = type;
		}
	},
});








const { mountTurns, clockText, turnTone } = await import("./initiative.ts");

Object.assign(globalThis, { document: doc });
type State = Parameters<typeof mountTurns>[1];



test("the clock reads as minutes and seconds", () => {
	assert.equal(clockText(0), "0:00");
	assert.equal(clockText(7), "0:07");
	assert.equal(clockText(59), "0:59");
	assert.equal(clockText(60), "1:00");
	assert.equal(clockText(605), "10:05");
	assert.equal(clockText(-4), "0:00");
});



test("the tone turns at a minute and again at two", () => {
	assert.equal(turnTone(0), "plain");
	assert.equal(turnTone(59), "plain");
	assert.equal(turnTone(60), "warning");
	assert.equal(turnTone(119), "warning");
	assert.equal(turnTone(120), "danger");
});









test("the stylesheet paints every tone the clock can reach", () => {
	const css = readFileSync(
		join(new URL(".", import.meta.url).pathname, "..", "..", "public", "css", "app.css"),
		"utf8",
	);

	const tones = new Set<string>();
	for (let seconds = 0; seconds <= 600; seconds += 1) {
		tones.add(turnTone(seconds));
	}

	assert.deepEqual([...tones], ["plain", "warning", "danger"]);

	for (const tone of tones) {
		
		
		
		if (tone === "plain") {
			continue;
		}

		assert.ok(
			css.includes(`data-turn-tone="${tone}"`),
			`nothing in public/css/app.css reads data-turn-tone="${tone}"`,
		);
	}
});



function strip(active: string | null) {
	
	
	
	
	
	const timer = new El("data-turn-timer");
	const next = new El("data-turn-next").append(timer);
	const acting = new El("data-turn-active");
	const root = new El("data-turns").append(acting, next);
	const mount = new El().append(root);

	const state = { initiative: { active, entries: [], round: 1 } } as unknown as State;

	let clock = 0;
	const turns = mountTurns(mount as unknown as HTMLElement, state, () => clock);

	return {
		mount,
		root,
		timer,
		next,
		acting,
		state,
		turns,
		tick(ms: number) {
			clock += ms;
		},
		activate(id: string | null) {
			(state as { initiative: { active: string | null } }).initiative.active = id;
		},
	};
}



test("the clock counts from the moment the turn changed", () => {
	const fight = strip("01ARI");

	fight.turns.changed();
	assert.equal(fight.timer.textContent, "0:00");

	fight.tick(65_000);
	fight.root.fire("nothing");
	fight.turns.changed();

	assert.equal(fight.timer.textContent, "1:05");
	assert.equal(fight.timer.getAttribute("data-turn-tone"), "warning");

	fight.turns.stop();
});














test("the swap that brings the clock in starts it running", () => {
	const root = new El("data-turns").append(new El("data-turn-active"));
	const mount = new El().append(root);
	const state = { initiative: { active: "01ARI", entries: [], round: 1 } } as unknown as State;

	let clock = 0;
	const turns = mountTurns(mount as unknown as HTMLElement, state, () => clock);

	
	
	turns.changed();

	
	const timer = new El("data-turn-timer");
	root.append(new El("data-turn-next").append(timer));

	const ticks: (() => void)[] = [];
	const realSet = globalThis.setInterval;
	const realClear = globalThis.clearInterval;
	globalThis.setInterval = ((fn: () => void) => {
		ticks.push(fn);

		return 0;
	}) as unknown as typeof setInterval;
	globalThis.clearInterval = (() => {}) as unknown as typeof clearInterval;

	try {
		doc.fire("htmx:after:swap", { target: root });
	} finally {
		globalThis.setInterval = realSet;
		globalThis.clearInterval = realClear;
	}

	assert.equal(timer.textContent, "0:00");
	assert.equal(ticks.length, 1, "the swap that brought the clock in started nothing");

	
	
	clock = 75_000;
	ticks[0]();

	assert.equal(timer.textContent, "1:15");
	assert.equal(timer.getAttribute("data-turn-tone"), "warning");

	turns.stop();
});



test("a change that is not a new turn leaves the clock alone", () => {
	const fight = strip("01ARI");

	fight.turns.changed();
	fight.tick(30_000);
	fight.turns.changed();

	assert.equal(fight.timer.textContent, "0:30");

	fight.activate("01GOBLIN");
	fight.turns.changed();

	assert.equal(fight.timer.textContent, "0:00");

	fight.turns.stop();
});




test("a tracker with nobody acting writes no time", () => {
	const fight = strip(null);

	fight.turns.changed();
	fight.tick(90_000);
	fight.turns.changed();

	assert.equal(fight.timer.textContent, "0:00");

	fight.turns.stop();
});




test("the acting line is brought into view when the turn moves", () => {
	const fight = strip("01ARI");

	fight.turns.changed();
	assert.equal(fight.acting.scrolled, 1);

	
	
	fight.root.setAttribute("data-dragging", "");
	fight.activate("01GOBLIN");
	fight.turns.changed();

	assert.equal(fight.acting.scrolled, 1);

	fight.turns.stop();
});




test("n presses the turn button and nothing else does", () => {
	const fight = strip("01ARI");

	doc.fire("keydown", { key: "n", target: null });
	assert.deepEqual(fight.next.pressed, ["click"]);

	doc.fire("keydown", { key: "N", target: null });
	assert.equal(fight.next.pressed.length, 2);

	
	
	doc.fire("keydown", { key: "n", target: null, ctrlKey: true });
	doc.fire("keydown", { key: "n", target: null, repeat: true });
	doc.fire("keydown", { key: "m", target: null });
	assert.equal(fight.next.pressed.length, 2);

	
	
	doc.fire("keydown", { key: "n", target: { tagName: "INPUT" } });
	assert.equal(fight.next.pressed.length, 2);

	fight.turns.stop();
});



test("n does nothing on a screen with no turn button", () => {
	const root = new El("data-turns");
	const mount = new El().append(root);
	const state = { initiative: { active: "01ARI", entries: [], round: 1 } } as unknown as State;

	const turns = mountTurns(mount as unknown as HTMLElement, state, () => 0);

	doc.fire("keydown", { key: "n", target: null });

	turns.stop();
});
