// A ULID, minted here rather than asked for.
//
// THE CLIENT MINTS A STROKE'S ID BECAUSE IT CANNOT WAIT FOR ONE. A pen begins a
// line and then sends chunks of it at roughly ten hertz, and it cannot ask what
// to call the thing its hand is already drawing -- so the id goes out with the
// first message and the server's half of the bargain is narrow: it must parse,
// and it must be unused. internal/room/stroke.go says the same from its side.
//
// IT IS THIRTY LINES RATHER THAN A DEPENDENCY. The room bundle ships to every
// player at the table, and this is smaller than the import statement's share of
// one. fog.ts writes its own ear clipping and path.ts its own supercover
// rasteriser for the same reason.
//
// THE LAYOUT IS THE SPEC'S: 48 bits of millisecond timestamp then 80 bits of
// randomness, Crockford base32, 26 characters -- which is exactly what
// github.com/oklog/ulid/v2 unmarshals from JSON.

// CROCKFORD is base32 without I, L, O and U, so nothing in an id can be
// misread as something else when a person reads one out of a log.
const CROCKFORD = "0123456789ABCDEFGHJKMNPQRSTVWXYZ";

// TIME_CHARS covers the 48-bit timestamp in ten characters of five bits, and
// RANDOM_CHARS covers the 80 random bits in sixteen. Ten and sixteen is
// twenty-six, which is the length every ULID has.
const TIME_CHARS = 10;
const RANDOM_BYTES = 10;

// The last id's ingredients, kept so that two minted in the same millisecond
// come out in the order they were made.
//
// IT IS THE MONOTONIC VARIANT AND IT IS NOT DECORATION. Strokes are SORTED BY
// ID in the store, "the newest stroke on this floor" is what Ctrl+Z removes,
// and the finished buffer decides whether to rebuild by looking at the newest
// id -- so three separate things read draw order out of these, and eighty
// random bits would put two ids made in the same millisecond in whichever order
// they happened to land. Nothing a hand does produces two strokes a millisecond
// apart; this is here so that nothing has to depend on that being true.
let lastTime = 0;
const lastRandom = new Uint8Array(RANDOM_BYTES);

export function ulid(now: number = Date.now()): string {
	const time = Math.max(0, Math.floor(now));

	if (time === lastTime) {
		increment(lastRandom);
	} else {
		lastTime = time;
		crypto.getRandomValues(lastRandom);
	}

	return encodeTime(time) + encodeRandom(lastRandom);
}

// encodeTime is the timestamp, most significant character first.
//
// IT DIVIDES RATHER THAN SHIFTS. A millisecond timestamp passed 2^32 in 1970 and
// JavaScript's bitwise operators are 32-bit, so `t >>> 5` on a real clock reading
// is not the top of the number -- it is the top of the bottom half of it.
function encodeTime(time: number): string {
	let out = "";
	let left = time;

	for (let i = 0; i < TIME_CHARS; i++) {
		out = CROCKFORD[left % 32] + out;
		left = Math.floor(left / 32);
	}

	return out;
}

// encodeRandom packs ten bytes into sixteen characters, five bits at a time.
// Eighty bits divides by five exactly, so there is no remainder to pad.
function encodeRandom(bytes: Uint8Array): string {
	let out = "";
	let value = 0;
	let bits = 0;

	for (const byte of bytes) {
		// At most twelve bits are ever in flight -- four left over plus the
		// eight just added -- so this stays well inside what a bitwise operator
		// can hold.
		value = (value << 8) | byte;
		bits += 8;

		while (bits >= 5) {
			bits -= 5;
			out += CROCKFORD[(value >>> bits) & 31];
		}
	}

	return out;
}

// increment adds one to an 80-bit number held as bytes, from the bottom up. An
// overflow wraps, which is a run of 2^80 ids inside one millisecond and is not
// a case worth a branch.
function increment(bytes: Uint8Array): void {
	for (let i = bytes.length - 1; i >= 0; i--) {
		if (bytes[i] < 255) {
			bytes[i]++;

			return;
		}

		bytes[i] = 0;
	}
}
