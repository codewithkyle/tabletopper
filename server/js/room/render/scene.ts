// What the canvas draws, out of what the store holds.
//
// THE VIEWED LAYER IS THE FILTER AND IT IS APPLIED HERE, ONCE. Rendering, hit
// testing, the marquee and the riders lookup all read the same list, so the
// floor is decided in one place rather than four -- and a pawn on another floor
// is not "drawn faintly" or "skipped in the loop", it is simply not in the
// array anything downstream sees.
//
// A PLAYER'S STORE HOLDS NOTHING BUT THE ACTIVE FLOOR ANYWAY, because the
// projection removed the rest before the event was encoded. This filter is
// therefore the GM's: they may view a floor other than the active one to
// prepare it, and everything about that choice lives on the client.

import type { Pawn } from "../protocol.ts";
import type { Drawn } from "./pawn-pass.ts";

// CONDITION_RINGS_MAX is how many rings are drawn round one creature. The
// protocol caps conditions at sixteen and this matches it, so the drawing and
// the rule agree -- a seventeenth would be a ring the server would not have
// accepted.
export const CONDITION_RINGS_MAX = 16;

// RING_GAP and RING_WIDTH are the spacing and the weight of a condition ring,
// in DEVICE pixels. They are screen sizes rather than table sizes for the reason
// the pawn border is: a ring that scaled with the camera would be a hairline
// zoomed out, which is where a GM is when they most want to count them.
//
// DEVICE AND NOT CSS, unlike the distance label, and the two are different on
// purpose. A hairline wants to be an exact number of the pixels that actually
// exist -- which is the argument the grid's fragment shader makes for drawing
// itself exactly one device pixel wide -- and text wants to be a readable size
// to a person, which is CSS pixels. Halving a border on a retina screen makes it
// crisper; halving the label makes it unreadable.
export const RING_GAP = 3;
export const RING_WIDTH = 2;

// visiblePawns is the store's pawns on one floor, as the passes take them.
//
// IT WRITES INTO A REUSED ARRAY, because it runs on every rebuild and a fresh
// array of five hundred objects per rebuild is the allocation the second
// performance rule is about. The objects inside it are reused too: a rebuild
// overwrites the fields rather than replacing the entry.
export function visiblePawns(pawns: readonly Pawn[], layerID: string, out: Drawn[]): Drawn[] {
	let count = 0;

	for (const pawn of pawns) {
		if (pawn.layerId !== layerID) {
			continue;
		}

		const drawn = out[count] ?? (out[count] = blank());

		drawn.id = pawn.id;
		drawn.kind = pawn.kind;
		drawn.name = pawn.name;
		drawn.image = pawn.image;
		drawn.x = pawn.x;
		drawn.y = pawn.y;
		drawn.z = pawn.z;
		drawn.size = pawn.size;
		drawn.footprintW = pawn.footprintW;
		drawn.footprintH = pawn.footprintH;

		// A pawn a player cannot see never reaches their store at all, so this
		// is only ever true on the GM's copy -- which is exactly what the
		// desaturated draw is for.
		drawn.hidden = !pawn.visible;

		// DEAD IS A NUMBER THE VIEWER WAS ACTUALLY GIVEN. A monster in a room
		// that hides its hit points arrives with hp null, and the honest answer
		// is that the player does not know it is down -- inferring it from a
		// band would leak the thing the setting exists to withhold.
		drawn.dead = pawn.hp !== null && pawn.hp <= 0;

		count++;
	}

	out.length = count;

	return out;
}

// ringRadius is the outer radius of the nth condition ring, in map pixels. They
// are concentric, outside the pawn's own border, one gap apart.
export function ringRadius(half: number, index: number, worldPerDevicePixel: number): number {
	return half + (RING_GAP + index * (RING_WIDTH + RING_GAP)) * worldPerDevicePixel;
}

// blank is one reusable slot in the output array.
function blank(): Drawn {
	return {
		id: "",
		kind: "monster",
		name: "",
		image: "",
		x: 0,
		y: 0,
		z: 0,
		size: "medium",
		footprintW: 0,
		footprintH: 0,
		hidden: false,
		dead: false,
	};
}
