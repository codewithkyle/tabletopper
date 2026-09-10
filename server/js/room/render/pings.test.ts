// The rings, which are entirely the client's: the server sends a point and a
// person, and everything about what that looks like is decided in here.
//
// THE ARITHMETIC IS WHAT THESE TEST, which is why ringAt is exported. A ping
// that came out wrong would look like a ping -- a circle on the map, in the
// right colour, at the right place -- and be wrong in the two ways nobody
// squinting at a table would catch: a ring that stops short of the square it is
// pointing at, and one that never lets the frame loop go quiet.

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

// THE ONE THING A PING HAS TO DO. A ring that stopped halfway would point at a
// circle rather than at a square, and the whole gesture is the square.
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

// It opens at its widest and is gone by the end, both exactly.
test("a ring starts wide and then does not exist", () => {
	assert.equal(ringAt(0, 0)?.radius, RADIUS_MAX);
	assert.equal(ringAt(-1, 0), null);
	assert.equal(ringAt(LIFE, 0), null);
	assert.equal(ringAt(LIFE, RINGS - 1), null);
});

// THE STAGGER IS WHAT BUYS BACK WHAT CONVERGING GAVE UP. One sweep is easier to
// catch in the corner of an eye than one arrival, so there are three arrivals.
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

// A RING IS AT FULL STRENGTH WHILE IT IS DOING THE POINTING. One that faded the
// whole way through would be dimmest exactly when it is closest to the square.
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

// THE LAYER RIDES ALONG ON THE EVENT so that a GM working on another floor is
// not shown a marker floating over a map it does not belong to.
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

// TWO PEOPLE POINTING AT THE SAME DOOR ARE TWO PEOPLE, which is the whole of
// what the colour says -- there is no name on a ping and the glyph atlas could
// not spell one.
test("each ping keeps the colour it arrived with", () => {
	const pings = newPings();
	const into = recorder();

	pings.add(GROUND, 0, 0, RED, 1000);
	pings.add(GROUND, 100, 0, BLUE, 1000);
	pings.build(GROUND, 1000, CELL, into);

	assert.deepEqual(into.rings.map((r) => r.color), [RED, BLUE]);
});

// THE FRAME LOOP IS THE BUDGET. A room nobody is touching renders no frames at
// all, and a ping that never reported itself finished would be a tab kept awake
// for an evening by one click.
test("the loop is kept alive for exactly as long as a ping lasts", () => {
	const pings = newPings();

	assert.equal(pings.settling(1000), false);

	pings.add(GROUND, 0, 0, RED, 1000);

	assert.equal(pings.settling(1000), true);
	assert.equal(pings.settling(1000 + LIFE - 1), true);
	assert.equal(pings.settling(1000 + LIFE), false);
});

// AND SETTLING IS THE SWEEP AS WELL AS THE ANSWER, which is the reason it runs
// over every floor rather than the viewed one: build never sees a floor nobody
// is looking at, so this is the only thing that would ever drop what is on it.
test("a ping on a floor nobody looked at is still forgotten", () => {
	const pings = newPings();
	const into = recorder();

	pings.add(CELLAR, 10, 10, RED, 1000);

	// The GM spends the whole ping looking at the ground floor.
	for (let now = 1000; now < 1000 + LIFE; now += 16) {
		pings.build(GROUND, now, CELL, into);
		assert.equal(pings.settling(now), true);
	}

	assert.equal(pings.settling(1000 + LIFE), false);

	const cellar = recorder();
	pings.build(CELLAR, 1000 + LIFE, CELL, cellar);
	assert.equal(cellar.rings.length, 0);
});

// A BOUND ON A BUG RATHER THAN ON A PERSON: nobody makes seventeen of these in
// a second by hand, and a script is refused by the socket first.
test("a floor holds a bounded number of pings and drops the oldest", () => {
	const pings = newPings();
	const into = recorder();

	for (let i = 0; i < CAP + 4; i++) {
		pings.add(GROUND, i, 0, RED, 1000);
	}

	pings.build(GROUND, 1000, CELL, into);

	// One ring each, because they all arrived in the same millisecond.
	assert.equal(into.rings.length, CAP);
	assert.equal(into.rings[0].x, 4, "the oldest four should have gone");
	assert.equal(into.rings[CAP - 1].x, CAP + 3);
});
