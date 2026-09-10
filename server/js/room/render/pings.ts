// The rings that say "look here".
//
// A PING IS ONE SECOND OF ATTENTION AND NOTHING ELSE. Nothing here is sent,
// stored, reduced or restored -- the server's Pinged is Transient() and never
// reaches the reducer -- so this is the whole of the feature on the client: a
// pool of points with a birthday, and the arithmetic that turns one into three
// rings converging on a square.
//
// THEY CONVERGE RATHER THAN EXPANDING, and that is the one decision in here
// worth arguing about. The eye follows a moving edge, so a ring that grows
// leads it AWAY from the thing being pointed at and leaves it nowhere in
// particular; a ring that closes puts it on the square. What a ping means is
// "look HERE", and the animation should finish where the sentence does.
//
// The expanding version is better at exactly one thing -- it sweeps more area,
// so it is easier to catch in the corner of an eye -- and STAGGER is what buys
// that back. Three arrivals over a second is three chances to notice, covering
// the same ground one sweep would have.
//
// NO TIMER, WHICH IS THE BUDGET. render/frame.ts promises that a room nobody is
// touching renders no frames at all, so settling() is what keeps the loop alive
// and the loop stops on its own when the last ring is gone. A setTimeout would
// keep a tab awake that has nothing left to draw.
//
// IT HOLDS NO WebGL, which is decals.ts's rule and buys more here than it does
// there: everything interesting about a ping is arithmetic over time, and
// arithmetic over time is what a headless test can check.

// RingTarget is what a ping is drawn through, named here rather than imported
// for the reason in the header.
//
// IT IS ONE HOLLOW CIRCLE AND NOT THE RING PASS. That pass draws rotatable
// rectangles too and is already used three times a frame by things that are not
// this, so beginning the batch and drawing it belong to the renderer; what
// crosses this boundary is the one shape a ping is made of.
export interface RingTarget {
	ellipse(
		x: number, y: number, radius: number,
		color: readonly [number, number, number], alpha: number, thickness: number,
	): void;
}

// RINGS, RING_MS and STAGGER are the shape of one ping: three rings, each alive
// for RING_MS, each starting STAGGER after the one before it.
//
// A SECOND IN ALL, AND THAT IS THE NUMBER TO ARGUE WITH. Long enough to be
// caught out of the corner of an eye; short enough that a ping can never become
// furniture on the map, which is what separates this from the drawing tools --
// a line stays because somebody meant it to, and a ping means nothing five
// seconds after it was sent.
export const RINGS = 3;
const RING_MS = 700;
const STAGGER = 180;

// LIFE is the whole thing, and it is derived rather than written down twice.
export const LIFE = RING_MS + (RINGS - 1) * STAGGER;

// RADIUS_MAX and RADIUS_MIN are where a ring starts and stops, IN CELLS.
//
// IN CELLS BECAUSE A PING POINTS AT A SQUARE. Two cells across is a gesture
// aimed at a creature and its neighbours; the same distance in map pixels would
// be a different gesture on every map, since cellSize is the GM's to set.
export const RADIUS_MAX = 2;
export const RADIUS_MIN = 0.25;

// WIDTH is RING_WIDTH, the condition ring's, in DEVICE pixels -- the pass
// converts it against the zoom, so a ping is the same weight on screen however
// far the camera is out. It is not imported from scene.ts because the two are
// the same number for different reasons and coupling them would mean a change
// to a condition ring silently changing this.
const WIDTH = 2;

// FADE is the last quarter of a ring's life, and the only part of it that is
// not at full strength. A ring that faded the whole way through would be at
// half alpha exactly when it is closest to the point and doing the pointing.
const FADE = 0.25;

// CAP is how many live pings one floor holds, oldest dropped.
//
// IT IS A BOUND ON A BUG RATHER THAN ON A PERSON. Six people at a table cannot
// make sixteen of these in a second by hand, and a script trying to is refused
// by the socket's own token bucket long before it reaches here; what this
// catches is a loop somewhere in this bundle sending one per frame.
export const CAP = 16;

// ringAt is the whole animation, and it is a pure function of one ring's age so
// that the thing worth testing can be tested without a canvas, a clock or a
// pool. draw.ts exports strokeSegments and measure for the same reason: the
// geometry is the part that can be wrong in a way nobody notices.
//
// The radius comes back IN CELLS -- see RADIUS_MAX -- and null means this ring
// has not started yet or is already over.
export function ringAt(age: number, index: number): { radius: number; alpha: number } | null {
	const t = (age - index * STAGGER) / RING_MS;
	if (t < 0 || t >= 1) {
		return null;
	}

	// EASED OUT, so a ring moves fast and then settles onto the point. It reads
	// as landing rather than as sliding, which is the difference between a
	// gesture that arrives and one that merely stops.
	const eased = 1 - (1 - t) ** 3;

	return {
		radius: RADIUS_MAX + (RADIUS_MIN - RADIUS_MAX) * eased,
		alpha: t < 1 - FADE ? 1 : (1 - t) / FADE,
	};
}

interface Ping {
	x: number;
	y: number;
	born: number;

	// The pinger's actorColor, resolved at the door rather than held as an id.
	// It is the same colour their drag ghosts and their ruler are drawn in for
	// everybody else, which is how a ping is recognisably somebody's without a
	// name on it -- the glyph atlas holds `0123456789 ft.` and could not spell
	// one anyway.
	color: readonly [number, number, number];
}

export interface Pings {
	// add is one ping arriving off the wire, INCLUDING THIS VIEWER'S OWN. The
	// server sends it to everybody and the pinger draws it from the wire like
	// anybody else, because drawing your own locally would put your marker on
	// the map a round trip before everybody else's -- and the one thing a ping
	// has to be is in the same place at the same time on every screen. See
	// Ping.Apply in internal/room/ping.go.
	add(layerID: string, x: number, y: number, color: readonly [number, number, number], now: number): void;

	// build fills the target with one floor's rings. Only the floor being
	// LOOKED at is ever asked for: a marker floating over a map it does not
	// belong to is the case the layer rides along on the event to prevent.
	build(layerID: string, now: number, cell: number, into: RingTarget): void;

	// settling is the frame loop's question -- is anything still moving -- AND
	// IT IS ALSO THE SWEEP.
	//
	// THE TWO ARE ONE CALL ON PURPOSE, and it is sprites.end()'s arrangement
	// for sprites.end()'s reason: build only ever sees the floor being looked
	// at, so a ping left on a floor the GM walked away from would sit in the
	// map until the tab was closed. This runs over every floor, every frame,
	// which is a handful of subtractions.
	//
	// SO THE CALLER MUST NOT SHORT-CIRCUIT IT. See the note at the return of
	// drawPawns.
	settling(now: number): boolean;
}

export function newPings(): Pings {
	const byLayer = new Map<string, Ping[]>();

	return {
		add(layerID, x, y, color, now) {
			const list = byLayer.get(layerID) ?? [];
			byLayer.set(layerID, list);

			list.push({ x, y, born: now, color });

			if (list.length > CAP) {
				list.splice(0, list.length - CAP);
			}
		},

		build(layerID, now, cell, into) {
			const list = byLayer.get(layerID);
			if (!list) {
				return;
			}

			for (const ping of list) {
				const age = now - ping.born;

				for (let i = 0; i < RINGS; i++) {
					const ring = ringAt(age, i);
					if (!ring) {
						continue;
					}

					into.ellipse(ping.x, ping.y, cell * ring.radius, ping.color, ring.alpha, WIDTH);
				}
			}
		},

		settling(now) {
			let live = false;

			for (const [layerID, list] of byLayer) {
				let kept = 0;
				for (const ping of list) {
					if (now - ping.born >= LIFE) {
						continue;
					}
					list[kept++] = ping;
				}
				list.length = kept;

				// A FLOOR WITH NOTHING ON IT IS FORGOTTEN rather than left
				// holding an empty array, so a session that pings its way
				// through nine floors does not keep nine of them.
				if (kept === 0) {
					byLayer.delete(layerID);

					continue;
				}

				live = true;
			}

			return live;
		},
	};
}
