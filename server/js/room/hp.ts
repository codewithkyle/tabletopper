// THE HIT-POINT BOXES DO ARITHMETIC, and they do it in the field.
//
// "The goblin takes 7" is the single most repeated entry in a fight, and doing
// it by hand means reading 23, subtracting, and typing 16 -- three steps of
// which two are the player's job and not the interface's. So the box takes a
// sum: 23-7 is 16, 23-7-4 is 12, and a box cleared and given +5 counts from
// what the pawn has now.
//
// THE SERVER EVALUATES THE SAME STRINGS AND IS THE AUTHORITY. This is the same
// arrangement path.ts has with snap.go: the client resolves it so the number
// appears the moment the field is left, and the server resolves it again
// because the server is what actually holds the pawn. The two must agree, and
// evaluateHP in internal/controllers/room-pawns.go is the other half.
//
// IT RUNS ON change AND THEREFORE ON BLUR OR ENTER, which is why the field is
// not debounced on input like the rest of the panel: a sum typed a character
// at a time is a different sum at every keystroke, and "23-" is not a number
// at all. The whole entry is read once, when the person has finished it.
//
// THE LISTENER CAPTURES, and that is load-bearing rather than a habit. htmx
// binds its own change handler to the form, which is an ancestor; a bubbling
// listener here would run after htmx had already read the field and posted
// "23-7" as the new value. Capturing runs from the document down, before the
// target and long before the form.

// LIMIT is how long an entry may be. It is not a rule about hit points -- the
// core decides those -- but a bound on the arithmetic, so a pasted essay is
// refused as an entry rather than summed a character at a time.
const LIMIT = 24;

const EXPRESSION = /^[+-]?\d+([+-]\d+)*$/;

// mountHitPoints wires every hit-point box on the page, present and future, to
// one listener. The panels come and go with the windows they are in, so there
// is nothing here to bind per field and nothing to unbind when one closes.
export function mountHitPoints(): void {
	document.addEventListener("change", resolve, true);
}

function resolve(e: Changed): void {
	const field = e.target;
	if (!isField(field) || !field.hasAttribute("data-hp-math")) {
		return;
	}

	const value = evaluate(field.value, field.defaultValue);
	if (value === null) {
		return;
	}

	const text = String(value);
	field.value = text;

	// AND THE DEFAULT MOVES WITH IT. defaultValue is what a later relative
	// entry counts from -- it is the number the server last rendered -- so a
	// box that has just been resolved to 16 and is then given +5 has to answer
	// 21 rather than counting from the 23 that was there when the window
	// opened. The panel's own refetch overwrites both a moment later.
	field.defaultValue = text;
}

// evaluate answers the number an entry resolves to, or null for anything that
// is not a sum -- which is left in the field exactly as it was typed, so the
// server answers with a message about it rather than this quietly discarding
// what somebody meant.
//
// A LEADING SIGN IS RELATIVE AND ITS ABSENCE IS NOT. "7" in a box showing 23
// means seven: a GM setting a monster's hit points off a sheet types the
// number. "-7" in the same box means sixteen, which is what a GM applying
// damage says out loud.
export function evaluate(entry: string, current: string): number | null {
	// Space AROUND an operator is how people type and is closed up; space
	// between two digits is not, and "23 7" is refused rather than quietly
	// becoming two hundred and thirty-seven.
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

// sum adds the terms left to right. There is no precedence to get wrong: the
// only operators are plus and minus, which is the whole of what a table does to
// a hit-point total.
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

// Changed and isField are the narrowing the listener needs without reaching for
// the DOM's own types in a module the tests import: a change event's target is
// anything on the page, and what this wants is an input with a value.
interface Changed {
	target: unknown;
}

interface Field {
	value: string;
	defaultValue: string;
	hasAttribute(name: string): boolean;
}

function isField(target: unknown): target is Field {
	const el = target as Partial<Field> | null;

	return typeof el?.hasAttribute === "function" && typeof el.value === "string";
}
