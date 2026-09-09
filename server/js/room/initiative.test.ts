// The turn order's four client-side jobs, and the two of them that are silent
// when they break.
//
// THE CLOCK RESTARTS ON A NEW TURN AND NOT ON A NEW STRIP. The strip refetches
// every time anything in the fight changes -- a hit, a condition, a drag -- and
// a clock that restarted on the swap would read a few seconds on a turn that
// had been running for two minutes. It compares the ACTIVE ENTRY, which is the
// only thing that means "somebody else is up now".
//
// AND NOTHING HAPPENS UNDER A HELD POINTER. A scroll or a refetch that moved
// the strip mid-drag would take the drop target out from under the hand.
//
// THE DOM HERE IS A STAND-IN AND IT IS DELIBERATELY SMALL. node has no
// document, and the parts of one this module touches are an attribute bag, a
// [name] selector and an event listener. Anything this fake cannot express is
// something the verification checklist looks at in a browser rather than
// something asserted here against a second implementation of the platform.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

// El is the whole of the DOM this module uses. It has to be installed as
// HTMLElement and Element BEFORE initiative.ts is imported, because the module
// narrows with instanceof -- which is what makes the import below dynamic.
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

	// Only [name] selectors, which is all this module writes.
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

	// fire is the test's way in: it runs the listeners this element holds with
	// whatever shape the module reads off the event.
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

// THE DOCUMENT IS INSTALLED AFTER THE IMPORT AND THAT ORDER IS LOAD-BEARING.
// initiative.ts imports SortableJS, which sniffs the browser at module scope --
// it builds an element and reads properties off it to find out what this engine
// supports. With no document at all it skips that and defines its class, which
// is what this file wants: the drag is a pointer gesture and belongs to the
// verification checklist, not to a second implementation of the DOM written
// here. The module reads `document` only when something is mounted.
const { mountTurns, clockText, turnTone } = await import("./initiative.ts");

Object.assign(globalThis, { document: doc });
type State = Parameters<typeof mountTurns>[1];

// MM:SS, and it is written out rather than reached for through Intl because a
// turn is minutes long and neither of those is shorter than this.
test("the clock reads as minutes and seconds", () => {
	assert.equal(clockText(0), "0:00");
	assert.equal(clockText(7), "0:07");
	assert.equal(clockText(59), "0:59");
	assert.equal(clockText(60), "1:00");
	assert.equal(clockText(605), "10:05");
	assert.equal(clockText(-4), "0:00");
});

// TWO STEPS AND NOT THREE. A "safe" green for the first thirty seconds answered
// a question nobody was asking.
test("the tone turns at a minute and again at two", () => {
	assert.equal(turnTone(0), "plain");
	assert.equal(turnTone(59), "plain");
	assert.equal(turnTone(60), "warning");
	assert.equal(turnTone(119), "warning");
	assert.equal(turnTone(120), "danger");
});

// AND EVERY TONE IT CAN REACH IS ONE THE STYLESHEET PAINTS. The attribute was
// written onto the clock once a second for a whole phase with no rule anywhere
// reading it: the tone stepped at a minute, nothing changed colour, and nothing
// failed -- the same silent shape as a listener for an event nobody dispatches.
//
// THE TWO HALVES CANNOT SEE EACH OTHER. server/js writes the value and
// server/css reads it, and no import runs between them, so the seam is checked
// here against the built stylesheet rather than assumed.
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
		// plain IS THE BUTTON AS RENDERED, so it is the one tone with no rule
		// of its own -- and the reason an unpainted step is invisible instead
		// of obvious.
		if (tone === "plain") {
			continue;
		}

		assert.ok(
			css.includes(`data-turn-tone="${tone}"`),
			`nothing in public/css/app.css reads data-turn-tone="${tone}"`,
		);
	}
});

// strip is a mounted fight: the root, the acting line, the clock and the
// button, plus a clock this test drives by hand.
function strip(active: string | null) {
	const timer = new El("data-turn-timer");
	const next = new El("data-turn-next");
	const acting = new El("data-turn-active");
	const root = new El("data-turns").append(acting, timer, next);
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

// The clock is this browser's own reading of how long the turn has been
// running: nothing on the wire carries it, so a reload starts it from zero.
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

// A LINE DRAGGED MID-TURN DOES NOT RESTART THE CLOCK, which is the whole reason
// this compares the active entry rather than reacting to the swap.
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

// AND A TRACKER THAT HAS NOT STARTED HAS NO CLOCK RUNNING, which is what keeps
// a room with a built-but-unstarted order from ticking a second at a time for
// the whole session.
test("a tracker with nobody acting writes no time", () => {
	const fight = strip(null);

	fight.turns.changed();
	fight.tick(90_000);
	fight.turns.changed();

	assert.equal(fight.timer.textContent, "0:00");

	fight.turns.stop();
});

// THE ACTING LINE IS SCROLLED INTO VIEW, because a twelve-combatant fight is
// wider than the strip and a turn order whose current turn has scrolled off the
// side is a turn order nobody can read.
test("the acting line is brought into view when the turn moves", () => {
	const fight = strip("01ARI");

	fight.turns.changed();
	assert.equal(fight.acting.scrolled, 1);

	// AND NEVER WHILE A DRAG IS IN PROGRESS. Scrolling the container under a
	// held pointer moves the drop target out from under it.
	fight.root.setAttribute("data-dragging", "");
	fight.activate("01GOBLIN");
	fight.turns.changed();

	assert.equal(fight.acting.scrolled, 1);

	fight.turns.stop();
});

// N PRESSES THE BUTTON THAT IS ALREADY ON THE SCREEN rather than posting, which
// is what makes one binding serve both roles with no rule of its own: the
// button exists on exactly the screens where the key should work.
test("n presses the turn button and nothing else does", () => {
	const fight = strip("01ARI");

	doc.fire("keydown", { key: "n", target: null });
	assert.deepEqual(fight.next.pressed, ["click"]);

	doc.fire("keydown", { key: "N", target: null });
	assert.equal(fight.next.pressed.length, 2);

	// A modifier makes it somebody else's key, and a repeat is a finger resting
	// on it.
	doc.fire("keydown", { key: "n", target: null, ctrlKey: true });
	doc.fire("keydown", { key: "n", target: null, repeat: true });
	doc.fire("keydown", { key: "m", target: null });
	assert.equal(fight.next.pressed.length, 2);

	// A KEY PRESSED IN A FIELD IS NOT A KEY PRESSED ON THE TABLE, which is
	// keys.ts's rule and is asked rather than reimplemented.
	doc.fire("keydown", { key: "n", target: { tagName: "INPUT" } });
	assert.equal(fight.next.pressed.length, 2);

	fight.turns.stop();
});

// A PLAYER PRESSING N OUT OF TURN FINDS NO BUTTON AND NOTHING HAPPENS, instead
// of a 403 in the alert modal. The client learns no route and no rule.
test("n does nothing on a screen with no turn button", () => {
	const root = new El("data-turns");
	const mount = new El().append(root);
	const state = { initiative: { active: "01ARI", entries: [], round: 1 } } as unknown as State;

	const turns = mountTurns(mount as unknown as HTMLElement, state, () => 0);

	doc.fire("keydown", { key: "n", target: null });

	turns.stop();
});
