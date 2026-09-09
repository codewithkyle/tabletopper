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
import { healthOf } from "./wounds.ts";

// Stacked is the part of a pawn that decides what is on top of what. It is a
// Pick rather than Pawn because the stress test's synthetic pawns are not in
// the store and the drag ghosts are not pawns yet.
export type Stacked = Pick<Drawn, "id" | "kind" | "z">;

// compareStack is the draw order, and it is the SAME order the hit test and the
// riders lookup read. Negative means a is underneath b.
//
// A TOKEN IS ALWAYS UNDER A CREATURE, WHATEVER z SAYS, which is the whole
// reason this is a function rather than a subtraction. Objects are the floor's
// furniture -- a rug, a road, a bloodstain, a wagon -- and a party that walked
// onto a rug spawned after them would otherwise vanish underneath it. z decides
// order WITHIN a kind, so two rugs still stack in the order they were laid and
// a goblin spawned later still stands in front of one spawned earlier.
//
// AND THE HIT TEST FOLLOWS IT EXACTLY. Clicking where a goblin overlaps a rug
// picks the goblin, because the goblin is what you can see there -- a hit test
// that disagreed with the draw order would be a click that selected something
// hidden behind what it landed on.
//
// THE TIE-BREAK IS THE ID, so two pawns that somehow share a z are drawn in an
// order that is the same on every client rather than whichever way the array
// happened to be built.
export function compareStack(a: Stacked, b: Stacked): number {
	const kinds = stackRank(a.kind) - stackRank(b.kind);
	if (kinds !== 0) {
		return kinds;
	}
	if (a.z !== b.z) {
		return a.z - b.z;
	}

	return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
}

function stackRank(kind: Pawn["kind"]): number {
	return kind === "object" ? 0 : 1;
}

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
		drawn.width = pawn.width;
		drawn.height = pawn.height;
		drawn.rotation = pawn.rotation;

		// A pawn a player cannot see never reaches their store at all, so this
		// is only ever true on the GM's copy -- which is exactly what the
		// desaturated draw is for.
		drawn.hidden = !pawn.visible;

		// HEALTH IS WHATEVER THE VIEWER WAS ACTUALLY GIVEN, WHICHEVER OF THE
		// TWO IT WAS -- and it is now the number for nearly everybody, because
		// projectPawn sends hit points to the whole table.
		//
		// THE ROOM'S LABEL SETTING IS NOT READ HERE AND MUST NOT BE. It governs
		// TEXT: the word under the pointer and the line in the details window.
		// What is drawn inside the disc is the creature -- blood, pallor, a
		// heartbeat -- and a table that has turned the words off has not turned
		// off the fact that the goblin is bleeding. See PawnLabels in
		// internal/room/state.go, which holds the same line from the other end.
		//
		// Null still reaches here and still draws nothing at all: it is a pawn
		// whose hit points were never set. See healthOf in wounds.ts.
		drawn.health = healthOf(pawn);

		count++;
	}

	out.length = count;

	return out;
}

// ringRadius is the outer radius of the nth condition ring, in map pixels. They
// are concentric, outside the pawn's own border, one gap apart.
//
// EVERY RING OUT HERE IS A CONDITION AND NOTHING ELSE SHARES THE STACK, which is
// what makes it countable. How hurt a creature is used to take index 0 and push
// them all out by one; it is drawn inside the disc now, by the pawn's own
// shader, for that reason and because the red it needed is the red a condition
// ring is already drawn in. See wounds.ts.
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
		width: 0,
		height: 0,
		rotation: 0,
		hidden: false,
		health: null,
	};
}
