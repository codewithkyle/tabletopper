import assert from "node:assert/strict";
import { test } from "node:test";

import { ulid } from "./ulid.ts";

const CROCKFORD = /^[0-9A-HJKMNP-TV-Z]{26}$/;

test("a ulid is twenty-six characters of Crockford base32", () => {
	for (let i = 0; i < 100; i++) {
		const id = ulid();

		assert.equal(id.length, 26, `${id} is not 26 characters`);
		assert.match(id, CROCKFORD);
	}
});

// The Go side unmarshals these with github.com/oklog/ulid/v2, which reads the
// timestamp back out of the first ten characters -- so an encoder that got the
// division wrong would produce ids that parse and sort in the wrong order.
test("the timestamp is the first ten characters, most significant first", () => {
	const earlier = ulid(1_600_000_000_000);
	const later = ulid(1_600_000_001_000);

	assert.ok(earlier.slice(0, 10) < later.slice(0, 10), `${earlier} should sort before ${later}`);

	// And the same millisecond gives the same ten characters, whatever the
	// random half does.
	assert.equal(ulid(1_600_000_000_000).slice(0, 10), ulid(1_600_000_000_000).slice(0, 10));
});

// THE STORE SORTS STROKES BY ID, "the newest stroke on this floor" is what
// Ctrl+Z removes, and the finished buffer decides whether to rebuild by looking
// at the newest id. All three read draw order out of these, so two minted in
// the same millisecond have to come out in the order they were made.
test("two ulids in the same millisecond increase", () => {
	const at = 1_600_000_000_000;
	const ids = [ulid(at), ulid(at), ulid(at), ulid(at)];

	for (let i = 1; i < ids.length; i++) {
		assert.ok(ids[i] > ids[i - 1], `${ids[i]} should sort after ${ids[i - 1]}`);
	}
});

test("a ulid in a later millisecond sorts after one before it", () => {
	const first = ulid(1_600_000_000_000);
	const second = ulid(1_600_000_000_001);

	assert.ok(second > first, `${second} should sort after ${first}`);
});

// A timestamp past 2^32 is the ordinary case -- the clock passed it in 1970 --
// and JavaScript's bitwise operators are 32-bit, which is the mistake this
// pins: a shift-based encoder answers the top of the BOTTOM half of the number.
test("a ulid encodes a timestamp past thirty-two bits", () => {
	const now = ulid(Date.now());
	const epoch = ulid(0);

	assert.notEqual(now.slice(0, 10), epoch.slice(0, 10));
	assert.equal(epoch.slice(0, 10), "0000000000");
	assert.ok(now.slice(0, 10) > epoch.slice(0, 10));
});
