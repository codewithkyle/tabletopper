// Five hundred pawns that are not on anybody's table.
//
// THE ARCHITECTURE ASKED FOR THIS IN WRITING. The overview's verification
// section says to build a stress toggle in the first week of renderer work and
// profile it, so that decisions about workers and WASM kernels are made from a
// flame graph rather than from worry. This is that toggle for pawns; the tile
// side of it is the benchmark sweep the debug panel already has.
//
// NOTHING HERE IS SENT ANYWHERE. These are not commands, they never reach the
// store, and no other client learns they exist -- which is the point: what is
// being measured is the renderer, not the protocol. They are handed to the pawn
// pass beside the real ones and drawn in the same call.
//
// THEY BORROW THE PICTURES ALREADY LOADED, so the measurement is of five
// hundred pawns rather than of five hundred image fetches. A stress run that
// pulled five hundred distinct images would be measuring the network, and the
// sprite cache holds a hundred and twenty-eight layers anyway.

import type { Drawn } from "./pawn-pass.ts";
import type { HPBand, Pawn } from "../protocol.ts";

const KINDS: Pawn["kind"][] = ["player", "monster", "npc", "object"];
const SIZES: Pawn["size"][] = ["tiny", "small", "medium", "large", "huge", "gargantuan"];

// HEALTH is the six bands plus the pawn nobody wrote hit points for, so a stress
// run puts wounds, pulses and skulls on roughly five sixths of the table. That
// is more injury than a real fight has and is the point: the pawn pass is
// measured under a load nobody will ever hand it.
const HEALTH: (HPBand | null)[] = [null, "healthy", "bruised", "bloody", "veryBloody", "nearDeath", "dead"];

// SPREAD is how far across the map they are scattered, in cells. Thirty by
// thirty is a battle map's worth, which puts them in and out of the viewport as
// the benchmark sweep pans -- a stress test with everything on screen at once
// measures fill rate, and one with everything off screen measures nothing.
const SPREAD = 30;

// stressPawns builds the synthetic list. It is deterministic given the same
// inputs, because a benchmark whose scene changed between runs would report a
// difference that was the scene rather than the renderer.
export function stressPawns(count: number, from: readonly Drawn[], cellSize: number, cx: number, cy: number): Drawn[] {
	const cell = Math.max(1, cellSize);
	const images = from.map((pawn) => pawn.image).filter((image) => image !== "");

	const out: Drawn[] = [];
	let seed = 0x5eed;

	// A tiny linear congruential generator rather than Math.random, so two runs
	// of the benchmark on the same machine measure the same scene.
	const next = (): number => {
		seed = (seed * 1664525 + 1013904223) >>> 0;

		return seed / 0x100000000;
	};

	for (let i = 0; i < count; i++) {
		const kind = KINDS[Math.floor(next() * KINDS.length)] ?? "monster";
		const object = kind === "object";

		out.push({
			id: `stress-${i.toString().padStart(4, "0")}`,
			kind,
			name: `Stress ${i}`,
			image: images.length > 0 ? images[Math.floor(next() * images.length)] : "",
			x: cx + Math.round((next() - 0.5) * SPREAD * cell),
			y: cy + Math.round((next() - 0.5) * SPREAD * cell),
			z: i,
			size: object ? "medium" : SIZES[Math.floor(next() * SIZES.length)] ?? "medium",
			width: object ? cell * (1 + Math.floor(next() * 4)) : 0,
			height: object ? cell * (1 + Math.floor(next() * 4)) : 0,
			rotation: object ? Math.floor(next() * 360) : 0,
			hidden: next() < 0.15,
			health: HEALTH[Math.floor(next() * HEALTH.length)] ?? null,
		});
	}

	return out;
}
