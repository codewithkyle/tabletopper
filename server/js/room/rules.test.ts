import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import type { Grid } from "./protocol.ts";
import { evaluate } from "./hp.ts";
import { snapAxis } from "./model/grid.ts";
import { bandOf } from "./model/health.ts";
const rules = join(import.meta.dirname, "..", "..", "internal", "room", "testdata", "rules");
function fixture<T>(name: string): T[] {
	const cases = JSON.parse(readFileSync(join(rules, `${name}.json`), "utf8")) as T[];
	assert.ok(cases.length > 0, `${name}.json has no cases`);
	return cases;
}
interface BandCase {
	hp: number;
	maxHp: number;
	band: string;
}
test("bandOf agrees with hpBand on every cut", () => {
	for (const c of fixture<BandCase>("bands")) {
		assert.equal(bandOf(c.hp, c.maxHp), c.band, `${c.hp} of ${c.maxHp}`);
	}
});
interface SnapCase {
	cell: number;
	offset: number;
	footprint: number;
	mode: Grid["snap"];
	value: number;
	snapped: number;
}
test("snapAxis agrees with SnapAxis in every mode and both parities", () => {
	for (const c of fixture<SnapCase>("snap")) {
		assert.equal(
			snapAxis(c.cell, c.offset, c.footprint, c.mode, c.value),
			c.snapped,
			`cell ${c.cell} offset ${c.offset} footprint ${c.footprint} ${c.mode} at ${c.value}`,
		);
	}
});
interface HPCase {
	entry: string;
	current: number | null;
	value: number | null;
	refused: boolean;
}
test("evaluate agrees with EvaluateHP on every entry", () => {
	for (const c of fixture<HPCase>("hp")) {
		const got = evaluate(c.entry, c.current === null ? "" : String(c.current));
		const label = `${JSON.stringify(c.entry)} against ${c.current}`;
		if (c.refused || c.value === null) {
			assert.equal(got, null, label);
		} else {
			assert.equal(got, c.value, label);
		}
	}
});
