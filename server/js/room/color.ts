import type { HexAlphaColorPicker } from "vanilla-colorful/hex-alpha-color-picker.js";

const SETTLE = 300;

const GROUP = "[data-color]";
const OPEN = "[data-color-open]";
const SWATCH = "[data-color-swatch]";
const FIELD = "[data-color-field]";
const PICKER = "[data-color-picker]";

const settling = new WeakMap<HTMLInputElement, ReturnType<typeof setTimeout>>();

export function mountColorFields(): void {
	document.addEventListener("click", opened);
	document.addEventListener("color-changed", picked);
	document.addEventListener("input", typed);
}

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

	const picker = find(field, PICKER);
	if (picker !== null) {
		(picker as HexAlphaColorPicker).color = color;
	}
}

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

function double(digits: string): string {
	let out = "";
	for (const digit of digits) {
		out += digit + digit;
	}

	return out;
}

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
