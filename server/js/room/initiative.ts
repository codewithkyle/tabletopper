// The four things the turn-order strip does in the browser, and there are only
// four because everything else about it is markup the server rendered.
//
// THE TIMER, THE SCROLL, THE DRAG AND THE N KEY. None of them is room state.
// Whose turn it is, what order they act in and how hurt each of them looks are
// all the server's and arrive as markup; what is left is one clock, one scroll
// offset, one pointer gesture and one key -- and each of those belongs to the
// person looking rather than to the table.
//
// THE CLOCK STARTS WHEN THIS BROWSER SEES THE ACTIVE LINE CHANGE, which is the
// honest reading of the one clock a client has. A reload starts it from zero: a
// turn that began before the tab was opened is a turn whose length this tab did
// not watch. Reordering the strip mid-turn does not restart it, because the
// active ENTRY has not changed -- which is why this compares the id rather than
// reacting to the swap.
//
// AND THE GM DOES NOT GET A COPY. A clock the whole table can read is a
// stopwatch on whoever is thinking, and the GM already knows a turn is dragging
// without being handed a number to quote. The server renders the timer into the
// acting line for its owner alone; this file writes digits into whatever it
// finds, so the rule is in one place.
//
// THE KEY PRESSES THE BUTTON THAT IS ALREADY IN THE PAGE rather than posting.
// initiative.next is authorized for the GM or for whoever owns a pawn in the
// acting line, so the button exists on exactly the screens where the key should
// work: a player pressing N out of turn finds no button and nothing happens,
// instead of a 403 in the alert modal. This file learns no route and no rule.
//
// ON THE GM'S SCREEN THAT BUTTON IS HIDDEN, and on a player's it is the End
// turn on their own acting line. The strip carried a visible Next until it did
// not: it was a control inside a display, it moved every time the order
// changed, and what a GM running a fight actually presses is this key. The
// Initiative menu is the other door, and prints the key beside its label.
//
// A REFETCH MID-DRAG WOULD DROP THE HELD LINE, so the strip's own hx-trigger
// declines while data-dragging is set and this is what sets it. An update that
// arrives during a drag is lost and the POST on the drop brings a fresh one back
// a moment later; see InitiativeTrigger in templ/pages/room-initiative.go.
//
// NO CLASS NAME IS WRITTEN IN THIS FILE. server/js is not a Tailwind source, so
// a class named here would never be emitted -- which is why the two Sortable
// classes below are defined in server/css/app.css and only NAMED here.

import Sortable from "sortablejs";

import { typing } from "./keys.ts";
import type { State } from "./protocol.ts";

export interface Turns {
	// changed is called after the store has reduced a snapshot or an
	// initiative.updated. It is a push rather than a subscription for the
	// reason everything else in this bundle is: the reducer mutates the state
	// in place and there is nothing to subscribe to.
	changed(): void;

	stop(): void;
}

// WARNING and DANGER are where the digits change tone, in seconds. Two steps
// rather than three: a "safe" green for the first thirty seconds answered a
// question nobody was asking.
const WARNING = 60;
const DANGER = 120;

// GRACE is how long after a drop a click on a line is thrown away, in
// milliseconds. A pointer that has just dragged a card across the strip and let
// go is not a pointer asking to give that creature the turn -- and the browser
// fires the click anyway, because the press and the release both landed on the
// same element.
const GRACE = 250;

// The two classes SortableJS is told to use. They are named here and DEFINED in
// server/css/app.css; see the header.
const GHOST = "initiative-ghost";
const CHOSEN = "initiative-chosen";

// turnTone is which of the three the digits are drawn in.
export function turnTone(seconds: number): string {
	if (seconds >= DANGER) {
		return "danger";
	}
	if (seconds >= WARNING) {
		return "warning";
	}

	return "plain";
}

// clockText is MM:SS. It is written out rather than reached for through
// Intl or toISOString because a turn is minutes long and neither of those is
// shorter than this once a turn passes an hour.
export function clockText(seconds: number): string {
	const whole = Math.max(0, Math.floor(seconds));
	const minutes = Math.floor(whole / 60);
	const rest = whole % 60;

	return `${minutes}:${rest < 10 ? "0" : ""}${rest}`;
}

export function mountTurns(mount: HTMLElement, state: State, now = () => performance.now()): Turns {
	// seen is the active entry this browser last knew about and since is when
	// it first saw it. They are separate from the store because the store holds
	// WHICH line is acting and this holds HOW LONG it has been, which nothing
	// on the wire carries.
	let seen: string | null = null;
	let since: number | null = null;
	let ticker: ReturnType<typeof setInterval> | null = null;

	let sortable: Sortable | null = null;
	let dropped: number | null = null;

	function strip(): HTMLElement | null {
		const found = mount.querySelector("[data-turns]");

		return found instanceof HTMLElement ? found : null;
	}

	function halt(): void {
		if (ticker === null) {
			return;
		}

		clearInterval(ticker);
		ticker = null;
	}

	// write fills the digits and reports whether there were any to fill. The
	// first tick that finds no element stops the interval, which is what makes
	// the GM's screen -- where the server renders no clock at all -- cost
	// nothing rather than one wakeup a second for the whole fight.
	//
	// IT RUNS ON THE SECOND AND AFTER EVERY SWAP, because a swap replaces the
	// span with the empty one the server rendered and an interval-only clock is
	// blank for up to a second after each refetch.
	function write(): boolean {
		const found = mount.querySelector("[data-turn-timer]");
		if (!(found instanceof HTMLElement)) {
			halt();

			return false;
		}

		const seconds = since === null ? 0 : (now() - since) / 1000;
		found.textContent = clockText(seconds);
		found.setAttribute("data-turn-tone", turnTone(seconds));

		return true;
	}

	// reveal keeps the acting line on screen. A twelve-combatant fight is wider
	// than the strip, and a turn order whose current turn has scrolled off the
	// side is a turn order nobody can read.
	//
	// AND NEVER WHILE A DRAG IS IN PROGRESS, which is the same gate the refetch
	// carries: scrolling the container under a held pointer moves the drop
	// target out from under it.
	function reveal(): void {
		const root = strip();
		if (root?.hasAttribute("data-dragging")) {
			return;
		}

		const found = mount.querySelector("[data-turn-active]");
		if (found instanceof HTMLElement && typeof found.scrollIntoView === "function") {
			found.scrollIntoView({ inline: "nearest", block: "nearest", behavior: "smooth" });
		}
	}

	// run owns the interval, and it is its own function because the moment the
	// turn moves and the moment the clock appears are not the same moment.
	//
	// THE CLOCK IS NOT ON SCREEN YET WHEN THE TURN BECOMES THIS PLAYER'S. The
	// server renders it into the acting line for its owner alone, so the strip
	// standing in the page while somebody else was up holds no clock at all.
	// initiative.updated arrives, panels.ts raises room:initiative, the strip
	// starts an async GET, and changed() runs in that same tick against the
	// markup the GET has not replaced yet -- so write() found nothing, started
	// nothing, and the digits the refetch delivered a moment later read 0:00
	// for the whole turn. Nothing failed; the number simply never moved.
	//
	// SO THE SWAP STARTS IT AS WELL, and both callers ask the same question:
	// is there a clock on this screen, and is somebody acting. A screen with
	// neither -- the GM's, and a tracker nobody has started -- still never has
	// an interval, which is the whole reason write() reports what it found.
	function run(): void {
		halt();

		if (write() && state.initiative.active !== null) {
			ticker = setInterval(write, 1000);
		}
	}

	function changed(): void {
		const active = state.initiative.active;

		if (active !== seen) {
			seen = active;
			since = active === null ? null : now();
		}

		run();
		reveal();
	}

	// order is the drop: the ids as they now stand, written onto the hidden
	// button and pressed.
	//
	// IT PRESSES AN htmx BUTTON RATHER THAN BUILDING A REQUEST, which is
	// pawn-menu.ts's pattern and is not a shortcut -- every other control on
	// this strip is an ordinary hx-post, and one gesture that reached for fetch
	// would be a second way for a refusal to reach the reader.
	function order(box: HTMLElement, root: HTMLElement): void {
		const button = root.querySelector("[data-turn-order]");
		if (!(button instanceof HTMLElement)) {
			return;
		}

		const ids: string[] = [];
		for (const el of box.querySelectorAll("[data-entry]")) {
			const id = el.getAttribute("data-entry");
			if (id) {
				ids.push(id);
			}
		}

		button.setAttribute("hx-vals", JSON.stringify({ entries: ids.join(",") }));
		button.dispatchEvent(new MouseEvent("click", { bubbles: true }));
	}

	// remount destroys the instance and builds a new one on the element that
	// replaced it. The strip is swapped wholesale by htmx, so an instance bound
	// to the old element is bound to a node that is no longer in the document --
	// the old client created one per render and destroyed none, which is the bug
	// not to reproduce.
	function remount(): void {
		sortable?.destroy();
		sortable = null;

		const root = strip();
		if (!root?.hasAttribute("data-reorder")) {
			return;
		}

		const found = root.querySelector("[data-entries]");
		if (!(found instanceof HTMLElement)) {
			return;
		}

		const box: HTMLElement = found;

		sortable = new Sortable(box, {
			draggable: "[data-entry]",
			animation: 150,
			ghostClass: GHOST,
			chosenClass: CHOSEN,

			onStart() {
				root.setAttribute("data-dragging", "");
			},

			onEnd(event) {
				root.removeAttribute("data-dragging");
				dropped = now();

				if (event.oldIndex === event.newIndex) {
					return;
				}

				order(box, root);
			},
		});
	}

	// A CLICK THAT IS THE TAIL OF A DRAG IS NOT A CLICK. The browser fires one
	// after a press and release on the same element however far the pointer
	// travelled in between, and on this strip that click would hand the turn to
	// whichever creature was just dragged into place.
	//
	// IT IS TAKEN IN THE CAPTURE PHASE ON THE MOUNT, which is what lets it be
	// stopped before it reaches htmx's own listener on the line itself.
	function onClickCapture(event: Event): void {
		if (dropped === null || now() - dropped > GRACE) {
			return;
		}

		if (!(event.target instanceof Element) || !event.target.closest("[data-entry]")) {
			return;
		}

		event.preventDefault();
		event.stopPropagation();
	}

	function onKeyDown(event: KeyboardEvent): void {
		if (event.ctrlKey || event.metaKey || event.altKey || event.repeat) {
			return;
		}
		if (event.key.toLowerCase() !== "n" || typing(event.target)) {
			return;
		}

		const button = mount.querySelector("[data-turn-next]");
		if (button instanceof HTMLElement) {
			button.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		}
	}

	// A SWAP RESETS THE DIGITS, THE SCROLLBAR AND THE DRAG ALL AT ONCE, because
	// all three live on elements htmx has just replaced -- and on the swap that
	// hands a player their own turn it is what STARTS the clock, for the reason
	// spelled out over run(). The filter is what keeps a pawn window's own
	// refetch from tearing down a Sortable it has nothing to do with.
	//
	// AND IT IS THE ONLY THING THAT EVER BUILDS THE SORTABLE. The strip the
	// page renders is an empty placeholder that fetches itself, so the
	// remount() beneath these listeners runs against markup with no
	// [data-entries] in it and returns having done nothing. Every strip a GM
	// can actually drag arrived in a swap, which makes a listener that does not
	// fire here not a stale clock but a turn order that cannot be reordered at
	// all.
	//
	// THE NAME IS SPELLED WITH COLONS BECAUSE htmx 4 SPELLS IT WITH COLONS --
	// htmx:after:swap, htmx:before:request, htmx:after:settle -- and the
	// camelCase names of htmx 1 and 2 are not emitted by the library in
	// server/public/static. addEventListener for a name nothing dispatches is
	// not an error: it type checks, it bundles, it runs, and the feature is
	// simply dead. server/js/room/htmx-events.test.ts pins every name this
	// bundle listens for to the names that library actually contains.
	function onSwap(event: Event): void {
		if (!(event.target instanceof Element) || !event.target.hasAttribute("data-turns")) {
			return;
		}

		remount();
		run();
		reveal();
	}

	mount.addEventListener("click", onClickCapture, true);
	document.addEventListener("keydown", onKeyDown);
	document.addEventListener("htmx:after:swap", onSwap);

	remount();

	return {
		changed,

		stop() {
			halt();
			sortable?.destroy();
			sortable = null;
			mount.removeEventListener("click", onClickCapture, true);
			document.removeEventListener("keydown", onKeyDown);
			document.removeEventListener("htmx:after:swap", onSwap);
		},
	};
}
