// THE COLOUR FIELD AND THE PICKER BEHIND IT.
//
// THE NATIVE <input type="color"> CANNOT DO ALPHA, which is the whole reason
// this file exists. The grid colour is drawn over a map and is almost never
// wanted at full opacity, so the alpha pair is the half of the value that
// actually gets tuned -- and the only way to reach it was to type two hex
// digits and guess. The `alpha` attribute on the native input would answer
// that on Chromium and Safari; it is not yet somewhere every GM is, and a
// browser that ignores it fails silently back to guessing.
//
// SO THE PICKER IS vanilla-colorful's <hex-alpha-color-picker>, a custom
// element whose every style is inside its shadow root. That is why it was
// chosen over the popup pickers: there is no stylesheet to ship, nothing for
// Tailwind to scan, nothing to re-theme against two DaisyUI themes, and no
// class name written anywhere in here -- which server/js could not do anyway.
//
// THE TEXT FIELD IS STILL THE FORM FIELD. It keeps its name, its pattern and
// its value, so the form posts exactly what it posted before and a browser
// with no scripts still edits the colour by hand. The picker writes into it;
// it is not a control the server has ever heard of.
//
// IT IS ONE LISTENER PER EVENT ON THE DOCUMENT rather than anything a panel
// binds, for the reason hp.ts gives: a panel is markup htmx swapped into a
// window, it has no mount of its own to run, and it goes away when the window
// closes. The custom element needs no such help -- it upgrades itself the
// moment it is inserted -- so all that is delegated here is the wiring
// between the three elements of a group.
//
// THE FORM IS TOLD ONCE THE DRAG HAS SETTLED. `color-changed` fires on every
// pointer move, and the grid form saves on `change`; forwarding each one would
// be a POST per frame. The field's value is written live so the hex and the
// swatch track the drag, and the `change` that htmx is listening for is held
// until the picker has been quiet for SETTLE.

import type { HexAlphaColorPicker } from "vanilla-colorful/hex-alpha-color-picker.js";

// SETTLE is how long the picker has to stop moving before the form saves. It
// is longer than a frame and shorter than a thought: a GM dragging across the
// saturation square saves once at the end of the drag, and a GM who picks a
// colour and looks up sees the table change without having touched anything
// else.
const SETTLE = 300;

// The elements of one colour control, found from any of the others. The group
// is the boundary, so a second one on the same panel would not cross-wire.
const GROUP = "[data-color]";
const OPEN = "[data-color-open]";
const SWATCH = "[data-color-swatch]";
const FIELD = "[data-color-field]";
const PICKER = "[data-color-picker]";

// Timers are per field rather than one shared, so a panel with two colours in
// it does not have one drag cancel the other's save. A field that goes away
// with its window takes its entry with it.
const settling = new WeakMap<HTMLInputElement, ReturnType<typeof setTimeout>>();

// mountColorFields wires every colour control on the page, present and future.
//
// IT DOES NOT REGISTER THE CUSTOM ELEMENT. That import is in main.ts, because
// it runs customElements.define at module scope and the tests beside this file
// are run by node, which has no such registry. Nothing here touches the DOM
// until an event arrives.
export function mountColorFields(): void {
	document.addEventListener("click", opened);
	document.addEventListener("color-changed", picked);
	document.addEventListener("input", typed);
}

// The picker is folded away until it is asked for. A GM sets the grid colour
// once and then never again, and 176 pixels of permanent picker in a 420-pixel
// window would push half the settings below the fold for it.
function opened(e: Event): void {
	const button = closest(e.target, OPEN);
	if (button === null) {
		return;
	}

	const picker = find(button, PICKER);
	if (picker === null) {
		return;
	}

	picker.hidden = !picker.hidden;
	button.setAttribute("aria-expanded", picker.hidden ? "false" : "true");
}

// A drag on the picker. The value it emits drops the alpha pair when the
// colour is opaque and comes back lowercase, so it goes through normalize
// before it reaches a field a person reads.
function picked(e: Event): void {
	const picker = closest(e.target, PICKER);
	if (picker === null) {
		return;
	}

	const field = find(picker, FIELD);
	if (!(field instanceof HTMLInputElement)) {
		return;
	}

	const detail = (e as CustomEvent<{ value?: unknown }>).detail;
	const color = normalize(typeof detail?.value === "string" ? detail.value : "");
	if (color === "") {
		return;
	}

	field.value = color;
	paint(picker, color);
	settle(field);
}

// Somebody typing hex by hand, which stays possible and is sometimes faster.
//
// THE FIELD IS NOT REWRITTEN HERE. Normalising what is being typed would move
// the caret out from under the person typing it, and half of an entry is not
// a colour anyway -- so a partial value simply matches nothing and the picker
// waits. The server normalises what is finally posted.
function typed(e: Event): void {
	const field = e.target;
	if (!(field instanceof HTMLInputElement) || !field.matches(FIELD)) {
		return;
	}

	const color = normalize(field.value);
	if (color === "") {
		return;
	}

	paint(field, color);

	// Assigning `color` never fires `color-changed` -- only a pointer move
	// does -- so there is no loop to break here.
	const picker = find(field, PICKER);
	if (picker !== null) {
		(picker as HexAlphaColorPicker).color = color;
	}
}

// The chip on the disclosure button, over the white the template puts behind
// it. White is the honest backing: a colour at a fifth opacity has to look
// like a fifth of itself rather than like a darker panel.
//
// THE STYLE IS INLINE AND THAT IS DELIBERATE. server/js is not a Tailwind
// source, so a class name written here would never be emitted; geometry and
// colour set as properties are what window.ts does for the same reason. The
// first paint is the template's, not this function's.
function paint(from: Element, color: string): void {
	const swatch = find(from, SWATCH);
	if (swatch !== null) {
		swatch.style.backgroundColor = color;
	}
}

function settle(field: HTMLInputElement): void {
	const running = settling.get(field);
	if (running !== undefined) {
		clearTimeout(running);
	}

	settling.set(
		field,
		setTimeout(() => {
			settling.delete(field);
			field.dispatchEvent(new Event("change", { bubbles: true }));
		}, SETTLE),
	);
}

// normalize answers #RRGGBBAA in upper case, or "" for anything that is not a
// colour yet.
//
// EVERY ACCEPTED FORM BECOMES THE LONG ONE. The picker drops the alpha pair
// when a colour is opaque and shorthand is what people type, so without this
// the field would flip between #f00, #ff0000 and #ff0000cc as the drag crossed
// full opacity -- three spellings of the same setting, in a box whose whole
// job is to show the GM what the alpha currently is.
export function normalize(text: string): string {
	const digits = text.trim().replace(/^#/, "");
	if (!/^[0-9a-fA-F]+$/.test(digits)) {
		return "";
	}

	switch (digits.length) {
		case 3:
			return "#" + double(digits).toUpperCase() + "FF";
		case 4:
			return "#" + double(digits).toUpperCase();
		case 6:
			return "#" + digits.toUpperCase() + "FF";
		case 8:
			return "#" + digits.toUpperCase();
		default:
			return "";
	}
}

// double is the shorthand expansion: #f0c is #ff00cc.
function double(digits: string): string {
	let out = "";
	for (const digit of digits) {
		out += digit + digit;
	}

	return out;
}

// closest and find are the two directions a group is walked: up from whatever
// was clicked to the element that matters, and across from there to its
// siblings through the group they share.
function closest(target: unknown, selector: string): HTMLElement | null {
	if (!(target instanceof Element)) {
		return null;
	}

	const found = target.closest(selector);

	return found instanceof HTMLElement ? found : null;
}

function find(from: Element, selector: string): HTMLElement | null {
	const found = from.closest(GROUP)?.querySelector(selector);

	return found instanceof HTMLElement ? found : null;
}
