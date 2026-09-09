// The words over a pawn, and the fact that there is a word for every band.
//
// THE LIST HAS A TWIN IN GO. pages.PawnBandText prints the same six inside the
// pawn's window while this prints them on the table, so a band added to
// internal/room and forgotten in one of the two is one goblin described two
// ways on one screen. TestEveryBandHasAWord is the other half of this file.

import assert from "node:assert/strict";
import { test } from "node:test";

import { bandWord } from "./overlay.ts";
import type { HPBand } from "./protocol.ts";

// Written out rather than derived, because a generated type cannot be iterated
// at run time and a list that agreed with itself would assert nothing. This is
// the list a reader checks against internal/room/state.go.
const BANDS: HPBand[] = ["healthy", "bruised", "bloody", "veryBloody", "nearDeath", "dead"];

test("every band has a word", () => {
	for (const band of BANDS) {
		assert.notEqual(bandWord(band), "", band);
	}
});

test("the words are the ones the setting promises", () => {
	assert.equal(bandWord("healthy"), "Healthy");
	assert.equal(bandWord("bruised"), "Bruised");
	assert.equal(bandWord("bloody"), "Bloody");
	assert.equal(bandWord("veryBloody"), "Very bloody");
	assert.equal(bandWord("nearDeath"), "Near death");
	assert.equal(bandWord("dead"), "Dead");
});

// A band this build has never heard of prints nothing rather than its own name.
// The panel is one line of text over a monster; "bloodied" from a server a
// version ahead would read as a word the GM chose.
test("a band from another version prints nothing", () => {
	assert.equal(bandWord("bloodied"), "");
	assert.equal(bandWord(""), "");
});
