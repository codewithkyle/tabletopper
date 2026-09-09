import assert from "node:assert/strict";
import { test } from "node:test";

import { normalize } from "./color.ts";

// THE FIELD SHOWS THE ALPHA OR THE FIELD IS POINTLESS. vanilla-colorful drops
// the alpha pair the moment a colour reaches full opacity, so a GM dragging the
// alpha slider up to 100% would watch the two digits they were aiming at
// disappear from the box.
test("an opaque colour still spells its alpha out", () => {
	assert.equal(normalize("#ff0000"), "#FF0000FF");
	assert.equal(normalize("#ff0000ff"), "#FF0000FF");
});

// Shorthand is what people type and #F0C is a colour; the server stores the
// long form, so the expansion happens before the value is shown rather than
// after it is posted.
test("shorthand is expanded both ways", () => {
	assert.equal(normalize("#f0c"), "#FF00CCFF");
	assert.equal(normalize("#f0c8"), "#FF00CC88");
});

// The hash is optional in the field -- gridForm puts it back -- so a value
// pasted without one still drives the picker.
test("the hash is optional and the case is not", () => {
	assert.equal(normalize("ff0000cc"), "#FF0000CC");
	assert.equal(normalize("  #Ff0000Cc  "), "#FF0000CC");
});

// HALF AN ENTRY IS NOT A COLOUR. Every keystroke in the field is read, so
// "#ff" arrives on the way to "#ff0000" -- and answering something for it would
// push a colour nobody chose into the picker and repaint the swatch mid-word.
test("anything that is not a colour yet is refused", () => {
	for (const entry of ["", "#", "#f", "#ff", "#fffff", "#fffffff", "#gggggg", "#ff 00 00", "red"]) {
		assert.equal(normalize(entry), "", `${entry} was accepted`);
	}
});
