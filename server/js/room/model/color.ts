import type { HPBand, Pawn } from "../protocol.ts";
import type { Rgb } from "./types.ts";
export const KIND_COLORS: Record<Pawn["kind"], Rgb> = {
	player: [0.29, 0.55, 0.9],
	monster: [0.82, 0.28, 0.28],
	npc: [0.33, 0.67, 0.44],
	object: [0.55, 0.51, 0.46],
};
export const CONDITION_COLORS: Record<string, Rgb> = {
	red: [0.94, 0.27, 0.27],
	orange: [0.98, 0.57, 0.24],
	yellow: [0.98, 0.83, 0.25],
	green: [0.3, 0.76, 0.42],
	blue: [0.3, 0.6, 0.96],
	purple: [0.65, 0.4, 0.94],
	pink: [0.96, 0.5, 0.75],
	white: [0.95, 0.95, 0.95],
};
const ACTOR_COLORS: readonly Rgb[] = [
	[0.36, 0.65, 0.98],
	[0.99, 0.6, 0.28],
	[0.42, 0.82, 0.5],
	[0.85, 0.44, 0.9],
	[0.98, 0.78, 0.3],
	[0.4, 0.85, 0.83],
	[0.95, 0.45, 0.5],
	[0.72, 0.72, 0.78],
];
export const SELF_COLOR: Rgb = [0.98, 0.98, 0.99];
export const SELECT_COLOR: Rgb = [0.4, 0.78, 1.0];
export const BLOOD_FRESH: Rgb = [1, 0.13, 0.1];
export const BLOOD_DRIED: Rgb = [0.34, 0.06, 0.05];
export const AURA_GOLD: Rgb = [1, 0.78, 0.35];
export function auraColor(band: HPBand | null): Rgb {
	switch (band) {
		case "bloody":
		case "veryBloody":
		case "nearDeath":
			return BLOOD_FRESH;
		default:
			return AURA_GOLD;
	}
}
export function actorColor(id: string): Rgb {
	let hash = 0;
	for (let i = 0; i < id.length; i++) {
		hash = (hash * 31 + id.charCodeAt(i)) >>> 0;
	}
	return ACTOR_COLORS[hash % ACTOR_COLORS.length] ?? ACTOR_COLORS[0];
}
export function hexColor(rgb: Rgb): string {
	let out = "#";
	for (const channel of rgb) {
		out += Math.round(Math.min(Math.max(channel, 0), 1) * 255)
			.toString(16)
			.padStart(2, "0");
	}
	return out.toUpperCase();
}
export function parseColor(value: string, out: Float32Array): Float32Array {
	out[0] = 0;
	out[1] = 0;
	out[2] = 0;
	out[3] = 1;
	const hex = value.startsWith("#") ? value.slice(1) : value;
	if ((hex.length !== 6 && hex.length !== 8) || !/^[0-9a-fA-F]+$/.test(hex)) {
		return out;
	}
	out[0] = parseInt(hex.slice(0, 2), 16) / 255;
	out[1] = parseInt(hex.slice(2, 4), 16) / 255;
	out[2] = parseInt(hex.slice(4, 6), 16) / 255;
	if (hex.length === 8) {
		out[3] = parseInt(hex.slice(6, 8), 16) / 255;
	}
	return out;
}
