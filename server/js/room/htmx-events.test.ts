// EVERY htmx EVENT THIS APP LISTENS FOR IS ONE THE VENDORED htmx EMITS, and
// this test exists because the alternative failure is completely silent.
//
// htmx 4 NAMES ITS EVENTS WITH COLONS: htmx:after:swap, htmx:before:request,
// htmx:finally:request. htmx 1 and 2 named the same moments in camelCase, and
// a listener written against one of those old names type checks, bundles,
// loads and runs. addEventListener validates nothing; a listener for an event
// no library dispatches simply never fires, so whatever it powers is dead with
// nothing anywhere failing -- no throw, no warning, no red test.
//
// IT COST THE TURN ORDER ITS DRAG. mountTurns builds its SortableJS instance
// in the swap handler, because the strip the page renders is an empty
// placeholder that fetches its own content; with the listener spelled
// htmx:afterSettle that instance was never built and a GM could not reorder a
// fight. See the note in initiative.ts.
//
// THE NAMES ARE CHECKED AGAINST THE LIBRARY rather than against a list written
// here, so the next htmx upgrade that renames an event fails this test instead
// of quietly switching a handler off.
//
// IT LIVES IN room/ BECAUSE THAT IS WHERE THE TOOLING LOOKS -- the Makefile's
// js-test glob and the tsconfig include both stop there -- and it reaches out
// to cover server/js and server/public/js, which is every hand-written script
// this app ships. server/public/static is skipped: it is build output, and it
// is where htmx itself lives.

import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

// The three directories this file has to reach, as paths from itself.
const room = new URL(".", import.meta.url).pathname;
const js = join(room, "..");
const server = join(js, "..");
const library = join(server, "public", "static", "htmx.min.js");

// SELF is skipped by the scan below because it spells names wrongly on
// purpose, both in the prose above and in the assertion at the end.
const SELF = "htmx-events.test.ts";

// A NAME IN A STRING LITERAL AND NOWHERE ELSE. Prose above a listener explains
// which event was chosen and often names the ones that were not, so a scan
// that read comments would fail on its own documentation.
const NAME = /(["'`])(htmx:[^"'`\s]+)\1/g;

// emitted is every htmx event name that appears in the minified library. It is
// read out of the file rather than listed here for the reason in the header.
function emitted(): Set<string> {
	const found = new Set<string>();

	for (const match of readFileSync(library, "utf8").matchAll(/htmx:[a-zA-Z:]+/g)) {
		found.add(match[0].replace(/:$/, ""));
	}

	return found;
}

// scripts is every hand-written script, as [path, source] pairs.
function scripts(dir: string): [string, string][] {
	const out: [string, string][] = [];

	for (const entry of readdirSync(dir, { withFileTypes: true })) {
		const path = join(dir, entry.name);

		if (entry.isDirectory()) {
			out.push(...scripts(path));
		} else if ((entry.name.endsWith(".ts") || entry.name.endsWith(".js")) && entry.name !== SELF) {
			out.push([path, readFileSync(path, "utf8")]);
		}
	}

	return out;
}

test("every htmx event name in this app is one htmx dispatches", () => {
	const known = emitted();

	// The extractor found the library and not an empty file, and it found the
	// shape htmx 4 uses rather than the shape it does not.
	assert.ok(known.size > 20, `only ${known.size} event names found in ${library}`);
	assert.ok(known.has("htmx:after:swap"));

	const seen: string[] = [];

	for (const [path, source] of [...scripts(js), ...scripts(join(server, "public", "js"))]) {
		for (const match of source.matchAll(NAME)) {
			const name = match[2];
			seen.push(name);

			assert.ok(
				known.has(name),
				`${path} listens for ${name}, which the vendored htmx never dispatches`,
			);
		}
	}

	// AND THE SCAN ACTUALLY FOUND THE LISTENERS. A regex that matched nothing
	// would pass this test for ever while every name in the bundle rotted.
	assert.ok(seen.length >= 8, `only ${seen.length} htmx event names found in the scripts`);

	// The spelling that caused all this. It is asserted rather than assumed so
	// that a future htmx which brought the camelCase names back would say so
	// here rather than leave the header above quietly wrong.
	assert.ok(!known.has("htmx:afterSettle"));
});
