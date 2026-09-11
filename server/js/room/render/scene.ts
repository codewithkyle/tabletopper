import type { Pawn } from "../protocol.ts";
import type { Drawn } from "./pawn-pass.ts";
import { healthOf } from "../model/health.ts";
export const CONDITION_RINGS_MAX = 16;
export const RING_GAP = 3;
export const RING_WIDTH = 2;
export function visiblePawns(
	pawns: readonly Pawn[], layerID: string, out: Drawn[],
	concealed?: (pawn: Pawn) => boolean,
): Drawn[] {
	let count = 0;
	for (const pawn of pawns) {
		if (pawn.layerId !== layerID) {
			continue;
		}
		if (concealed?.(pawn)) {
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
		drawn.hidden = !pawn.visible;
		drawn.health = healthOf(pawn);
		count++;
	}
	out.length = count;
	return out;
}
export function ringRadius(half: number, index: number, worldPerDevicePixel: number): number {
	return half + (RING_GAP + index * (RING_WIDTH + RING_GAP)) * worldPerDevicePixel;
}
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
