import assert from "node:assert/strict";
import { test } from "node:test";
import { normalize } from "./color.ts";
test("an opaque colour still spells its alpha out", () => {
	assert.equal(normalize("#ff0000"), "#FF0000FF");
	assert.equal(normalize("#ff0000ff"), "#FF0000FF");
});
test("shorthand is expanded both ways", () => {
	assert.equal(normalize("#f0c"), "#FF00CCFF");
	assert.equal(normalize("#f0c8"), "#FF00CC88");
});
test("the hash is optional and the case is not", () => {
	assert.equal(normalize("ff0000cc"), "#FF0000CC");
	assert.equal(normalize("  #Ff0000Cc  "), "#FF0000CC");
});
test("anything that is not a colour yet is refused", () => {
	for (const entry of ["", "#", "#f", "#ff", "#fffff", "#fffffff", "#gggggg", "#ff 00 00", "red"]) {
		assert.equal(normalize(entry), "", `${entry} was accepted`);
	}
});
