import type { Drawn } from "../model/overlay.ts";
import type { HPBand, Pawn } from "../protocol.ts";
const KINDS: Pawn["kind"][] = ["player", "monster", "npc", "object"];
const SIZES: Pawn["size"][] = ["tiny", "small", "medium", "large", "huge", "gargantuan"];
const HEALTH: (HPBand | null)[] = [null, "healthy", "bruised", "bloody", "veryBloody", "nearDeath", "dead"];
const SPREAD = 30;
export function stressPawns(count: number, from: readonly Drawn[], cellSize: number, cx: number, cy: number): Drawn[] {
	const cell = Math.max(1, cellSize);
	const images = from.map((pawn) => pawn.image).filter((image) => image !== "");
	const out: Drawn[] = [];
	let seed = 0x5eed;
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
