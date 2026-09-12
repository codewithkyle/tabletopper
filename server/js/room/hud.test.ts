import assert from "node:assert/strict";
import { test } from "node:test";
import { bandWord } from "./hud.ts";
import type { HPBand } from "./protocol.ts";
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
test("a band from another version prints nothing", () => {
	assert.equal(bandWord("bloodied"), "");
	assert.equal(bandWord(""), "");
});
