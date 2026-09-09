// The floating pill's pointer modes: which one is lit, and the space bar that
// borrows one for as long as it is down.
//
// THE PILL USED TO BE A CONTROL WITH NOTHING BEHIND IT. It kept its own
// aria-pressed in server/public/js/room.js and the canvas never asked which
// button was pressed. Two of the five modes are real now, so the state moved
// into this bundle -- the one the canvas is in -- and the pill became the place
// the table reads its meaning from rather than a radio group that agreed with
// nothing.
//
// THERE ARE FIVE BUTTONS AND THE TABLE ASKS TWO QUESTIONS OF THEM. Is the
// camera taking this gesture -- which Move answers yes to and nothing else does
// -- and is the ruler the mode we are in, which is Measure. Select is the table
// as it has always behaved, and Fog and Draw are buttons whose features are not
// built, so they leave it on Select rather than dropping the GM into a mode
// where nothing works and nothing says why.
//
// THE TWO QUESTIONS ARE ASKED OF DIFFERENT BUTTONS ON PURPOSE. panning is asked
// of the button that is LIT, because the space bar lighting Move is exactly what
// it means to be in Move for as long as it is down. measuring is asked of the
// button that was CHOSEN, because the hold borrows the pointer rather than
// changing the tool -- a ruler that vanished every time the map was shoved
// twenty feet sideways would be a ruler nobody could use across a battlemap.
//
// WHAT EACH BUTTON DOES IS THE MARKUP'S TO SAY. room.go renders
// data-room-tool-pans and data-room-tool-measures onto exactly one tool each and
// this finds them by those attributes, because the alternative is the names
// "move" and "measure" written out in Go and again in TypeScript -- and that is
// a gesture which quietly stops working the day the list is renamed. The mode a
// room OPENS in is read the same way, off the button that was rendered pressed.
//
// THE SPACE BAR IS A HELD KEY AND NOT A TOGGLE, which is the gesture every
// drawing program has trained every hand to expect: hold it, shove the map
// across, let go, and carry on from the tool you were already in. It is
// deliberately not the only way to pan -- a mode you can leave switched on is
// what a trackpad, a tablet, and a long drag across a battlemap need, which is
// why the Move button is still there.
//
// IT IS TAKEN EVERYWHERE EXCEPT IN A FIELD, and taking it means the browser does
// not get it: a focused button would read the space bar as a press. Enter
// activates a button as well, and is what a keyboard is left with here. The one
// place the bar is untouched is text, which is what keys.ts is asked about.
//
// NO CLASS NAME IS WRITTEN IN THIS FILE. server/js is not a Tailwind source, so
// a class named here would never be emitted. What it writes is aria-pressed,
// which room.templ styles, and one inline cursor.

import { typing } from "./keys.ts";

export interface Tools {
	// panning is whether the camera has every gesture: the Move tool is lit, or
	// the space bar is down. It is asked at the moment of a press rather than
	// subscribed to, so a mode changed mid-drag cannot cut a gesture in half.
	panning(): boolean;

	// measuring is whether the ruler is the tool that was chosen, which the
	// space bar does not change. It is asked at a press AND on the way to a
	// frame, because it is what a measurement already on the table outlives:
	// switching to another tool is how one is put away.
	measuring(): boolean;

	// onChange runs when the mode changes: a click on the pill, or the space bar
	// going down or up.
	//
	// IT EXISTS FOR ONE FRAME. The table draws what the chosen tool put on it --
	// a ruler, once Measure has been used -- and a mode changed with the pointer
	// sitting still would otherwise leave that on screen until something else
	// happened to ask for a frame. Everything else about a mode is asked for
	// rather than announced; see panning.
	onChange(fn: () => void): void;

	stop(): void;
}

// PAN_KEY is the physical key rather than the character it produces, because
// this one is HELD rather than typed: `code` is the same key on every layout,
// and it does not change under a modifier the way `key` does.
const PAN_KEY = "Space";

// GRAB is the cursor while the camera has the pointer. It is set inline rather
// than as a class for the reason in the header, and it is the whole of the
// feedback that lands where the eyes already are -- the pill is in the corner,
// and a GM holding the space bar is looking at the map.
const GRAB = "grab";

// showing is which button the pill lights: the one that was chosen, or the
// panning one for as long as the space bar is down. A page that renders no
// panning tool goes on showing what it had, which is what makes the space bar
// harmless rather than special-cased.
export function showing<T>(chosen: T, pans: T | null, held: boolean): T {
	return held && pans !== null ? pans : chosen;
}

export function mountTools(mount: HTMLElement): Tools | null {
	const found = mount.querySelector("[data-room-tools]");
	if (!(found instanceof HTMLElement)) {
		return null;
	}

	// The re-declaration is what carries the narrowing above into the closures
	// below: TypeScript does not keep a control-flow narrowing across a
	// function boundary, and every one of these is one.
	const root: HTMLElement = found;

	const buttons = Array.from(root.querySelectorAll("[data-room-tool]"));
	const pans = root.querySelector("[data-room-tool-pans]");
	const measures = root.querySelector("[data-room-tool-measures]");

	// The canvas, for the cursor alone. A closed room and a browser without
	// WebGL2 both render none, and the modes go on working without one.
	const canvas = mount.querySelector("[data-tabletop-canvas]");

	// chosen is the button a hand last pressed, and held is the space bar. The
	// mode is the two of them together; see showing.
	let chosen = buttons.find((b) => b.getAttribute("aria-pressed") === "true") ?? buttons[0] ?? null;
	let held = false;
	let changed: (() => void) | null = null;

	function lit(): Element | null {
		return showing(chosen, pans, held);
	}

	function panning(): boolean {
		return pans !== null && lit() === pans;
	}

	function measuring(): boolean {
		return measures !== null && chosen === measures;
	}

	function paint(): void {
		const current = lit();

		for (const button of buttons) {
			button.setAttribute("aria-pressed", String(button === current));
		}

		if (canvas instanceof HTMLElement) {
			canvas.style.cursor = panning() ? GRAB : "";
		}

		changed?.();
	}

	function onClick(e: Event): void {
		if (!(e.target instanceof Element)) {
			return;
		}

		const button = e.target.closest("[data-room-tool]");
		if (!button) {
			return;
		}

		// A TOOL PRESSED WHILE THE BAR IS DOWN IS STILL THE TOOL CHOSEN. The
		// hold is on top of the choice rather than instead of it, so letting go
		// lands on what was just pressed rather than on what came before it.
		chosen = button;
		paint();
	}

	function onKeyDown(e: KeyboardEvent): void {
		if (e.code !== PAN_KEY || typing(e.target)) {
			return;
		}

		// Every time, including the repeats a held key produces: the default is
		// what a focused button would take as a press.
		e.preventDefault();

		if (held) {
			return;
		}

		held = true;
		paint();
	}

	function onKeyUp(e: KeyboardEvent): void {
		if (e.code !== PAN_KEY || !held) {
			return;
		}

		held = false;
		paint();
	}

	// A KEY HELD WHEN THE TAB LOST FOCUS IS NEVER RELEASED. Alt-tabbing out
	// mid-pan and coming back to a table that has been in Move mode ever since
	// is the one way a hold can get stuck, and the browser tells us about it.
	function onBlur(): void {
		if (!held) {
			return;
		}

		held = false;
		paint();
	}

	root.addEventListener("click", onClick);
	document.addEventListener("keydown", onKeyDown);
	document.addEventListener("keyup", onKeyUp);
	window.addEventListener("blur", onBlur);

	paint();

	return {
		panning,
		measuring,

		onChange(fn) {
			changed = fn;
		},

		stop() {
			root.removeEventListener("click", onClick);
			document.removeEventListener("keydown", onKeyDown);
			document.removeEventListener("keyup", onKeyUp);
			window.removeEventListener("blur", onBlur);
		},
	};
}
