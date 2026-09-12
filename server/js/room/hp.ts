const LIMIT = 24;
const EXPRESSION = /^[+-]?\d+([+-]\d+)*$/;
const ENTRY_SUFFIX = "Entry";
export function mountHitPoints(): void {
	document.addEventListener("change", resolve, true);
}
function resolve(e: Changed): void {
	const field = e.target;
	if (!isField(field) || !field.hasAttribute("data-hp-math")) {
		return;
	}
	const raw = field.value;
	const value = evaluate(raw, field.defaultValue);
	const twin = field.form?.elements.namedItem(field.name + ENTRY_SUFFIX);
	if (isField(twin)) {
		twin.value = value !== null && isRelative(raw) ? raw.trim() : "";
	}
	if (value === null) {
		return;
	}
	const text = String(value);
	field.value = text;
	field.defaultValue = text;
}
export function evaluate(entry: string, current: string): number | null {
	const text = entry.trim().replace(/\s*([+-])\s*/g, "$1");
	if (text.length === 0 || text.length > LIMIT || !EXPRESSION.test(text)) {
		return null;
	}
	const total = sum(text);
	if (text[0] === "+" || text[0] === "-") {
		return number(current) + total;
	}
	return total;
}
function isRelative(entry: string): boolean {
	const text = entry.trim();
	return text.length > 0 && (text[0] === "+" || text[0] === "-");
}
function sum(text: string): number {
	let total = 0;
	let sign = 1;
	let digits = "";
	for (const char of text) {
		if (char !== "+" && char !== "-") {
			digits += char;
			continue;
		}
		total += sign * number(digits);
		sign = char === "-" ? -1 : 1;
		digits = "";
	}
	return total + sign * number(digits);
}
function number(text: string): number {
	const value = Number.parseInt(text, 10);
	return Number.isFinite(value) ? value : 0;
}
interface Changed {
	target: unknown;
}
interface Field {
	value: string;
	defaultValue: string;
	name: string;
	form: { elements: { namedItem(name: string): unknown } } | null;
	hasAttribute(name: string): boolean;
}
function isField(target: unknown): target is Field {
	const el = target as Partial<Field> | null;
	return typeof el?.hasAttribute === "function" && typeof el.value === "string";
}
