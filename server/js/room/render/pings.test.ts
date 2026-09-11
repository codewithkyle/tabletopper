import assert from "node:assert/strict";
import { test } from "node:test";
import type { RingTarget } from "./pings.ts";
import { CAP, LIFE, RADIUS_MAX, RADIUS_MIN, RINGS, newPings, ringAt } from "./pings.ts";
const GROUND = "01LAYERGROUND";
const CELLAR = "01LAYERCELLAR";
const CELL = 64;
const RED: readonly [number, number, number] = [1, 0, 0];
const BLUE: readonly [number, number, number] = [0, 0, 1];
interface Ring {
	x: number;
	y: number;
	radius: number;
	color: readonly [number, number, number];
	alpha: number;
	thickness: number;
}
function recorder(): RingTarget & { rings: Ring[] } {
	const rings: Ring[] = [];
	return {
		rings,
		ellipse(x, y, radius, color, alpha, thickness) {
			rings.push({ x, y, radius, color, alpha, thickness });
		},
	};
}
test("a ring closes on the point and never overshoots it", () => {
	let previous = Infinity;
	for (let age = 0; age < LIFE; age += 5) {
		const ring = ringAt(age, 0);
		if (!ring) {
			break;
		}
		assert.ok(ring.radius <= RADIUS_MAX, `${ring.radius} is outside the opening radius`);
		assert.ok(ring.radius >= RADIUS_MIN - 1e-9, `${ring.radius} is inside the closing radius`);
		assert.ok(ring.radius < previous, `${ring.radius} did not close on ${previous}`);
		previous = ring.radius;
	}
	assert.ok(previous < RADIUS_MAX / 2, "the ring never got close to the point");
});
test("a ring starts wide and then does not exist", () => {
	assert.equal(ringAt(0, 0)?.radius, RADIUS_MAX);
	assert.equal(ringAt(-1, 0), null);
	assert.equal(ringAt(LIFE, 0), null);
	assert.equal(ringAt(LIFE, RINGS - 1), null);
});
test("the three rings arrive one after another", () => {
	assert.notEqual(ringAt(0, 0), null);
	assert.equal(ringAt(0, 1), null);
	assert.equal(ringAt(0, 2), null);
	let most = 0;
	for (let age = 0; age < LIFE; age += 5) {
		let live = 0;
		for (let i = 0; i < RINGS; i++) {
			if (ringAt(age, i)) {
				live++;
			}
		}
		most = Math.max(most, live);
	}
	assert.equal(most, RINGS, "the three rings never overlapped");
});
test("a ring fades at the end and not before it", () => {
	assert.equal(ringAt(0, 0)?.alpha, 1);
	let last = 1;
	let full = 0;
	for (let age = 0; age < LIFE; age++) {
		const ring = ringAt(age, 0);
		if (!ring) {
			break;
		}
		assert.ok(ring.alpha <= 1 && ring.alpha >= 0, `${ring.alpha} is not an alpha`);
		assert.ok(ring.alpha <= last, "a ring brightened partway through");
		if (ring.alpha === 1) {
			full++;
		}
		last = ring.alpha;
	}
	assert.ok(last < 0.1, "the ring did not fade out");
	assert.ok(full > 0, "the ring was never at full strength");
});
test("a ping is drawn at its point, in its own colour, in map pixels", () => {
	const pings = newPings();
	const into = recorder();
	pings.add(GROUND, 300, -50, RED, 1000);
	pings.build(GROUND, 1000, CELL, into);
	assert.equal(into.rings.length, 1);
	assert.equal(into.rings[0].x, 300);
	assert.equal(into.rings[0].y, -50);
	assert.equal(into.rings[0].color, RED);
	assert.equal(into.rings[0].radius, RADIUS_MAX * CELL);
});
test("only the floor being looked at is drawn", () => {
	const pings = newPings();
	pings.add(CELLAR, 10, 10, RED, 1000);
	const ground = recorder();
	pings.build(GROUND, 1000, CELL, ground);
	assert.equal(ground.rings.length, 0);
	const cellar = recorder();
	pings.build(CELLAR, 1000, CELL, cellar);
	assert.ok(cellar.rings.length > 0);
});
test("each ping keeps the colour it arrived with", () => {
	const pings = newPings();
	const into = recorder();
	pings.add(GROUND, 0, 0, RED, 1000);
	pings.add(GROUND, 100, 0, BLUE, 1000);
	pings.build(GROUND, 1000, CELL, into);
	assert.deepEqual(into.rings.map((r) => r.color), [RED, BLUE]);
});
test("the loop is kept alive for exactly as long as a ping lasts", () => {
	const pings = newPings();
	assert.equal(pings.settling(1000), false);
	pings.add(GROUND, 0, 0, RED, 1000);
	assert.equal(pings.settling(1000), true);
	assert.equal(pings.settling(1000 + LIFE - 1), true);
	assert.equal(pings.settling(1000 + LIFE), false);
});
test("a ping on a floor nobody looked at is still forgotten", () => {
	const pings = newPings();
	const into = recorder();
	pings.add(CELLAR, 10, 10, RED, 1000);
	for (let now = 1000; now < 1000 + LIFE; now += 16) {
		pings.build(GROUND, now, CELL, into);
		assert.equal(pings.settling(now), true);
	}
	assert.equal(pings.settling(1000 + LIFE), false);
	const cellar = recorder();
	pings.build(CELLAR, 1000 + LIFE, CELL, cellar);
	assert.equal(cellar.rings.length, 0);
});
test("a floor holds a bounded number of pings and drops the oldest", () => {
	const pings = newPings();
	const into = recorder();
	for (let i = 0; i < CAP + 4; i++) {
		pings.add(GROUND, i, 0, RED, 1000);
	}
	pings.build(GROUND, 1000, CELL, into);
	assert.equal(into.rings.length, CAP);
	assert.equal(into.rings[0].x, 4, "the oldest four should have gone");
	assert.equal(into.rings[CAP - 1].x, CAP + 3);
});
