

























import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";


const room = new URL(".", import.meta.url).pathname;
const js = join(room, "..");
const server = join(js, "..");
const library = join(server, "public", "static", "htmx.min.js");



const SELF = "htmx-events.test.ts";




const NAME = /(["'`])(htmx:[^"'`\s]+)\1/g;



function emitted(): Set<string> {
	const found = new Set<string>();

	for (const match of readFileSync(library, "utf8").matchAll(/htmx:[a-zA-Z:]+/g)) {
		found.add(match[0].replace(/:$/, ""));
	}

	return found;
}


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

	
	
	assert.ok(seen.length >= 8, `only ${seen.length} htmx event names found in the scripts`);

	
	
	
	assert.ok(!known.has("htmx:afterSettle"));
});
