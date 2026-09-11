

































import type { HPBand, Pawn } from "../protocol.ts";












export function bandOf(hp: number | null, maxHp: number | null): HPBand | null {
	if (hp === null) {
		return null;
	}
	if (hp <= 0) {
		return "dead";
	}
	if (maxHp === null || maxHp < 1) {
		return null;
	}

	if (hp * 20 <= maxHp) {
		return "nearDeath";
	}
	if (hp * 4 <= maxHp) {
		return "veryBloody";
	}
	if (hp * 2 <= maxHp) {
		return "bloody";
	}
	if (hp * 4 <= maxHp * 3) {
		return "bruised";
	}

	return "healthy";
}





type Health = Pick<Pawn, "hp" | "maxHp" | "hpBand">;







export function healthOf(pawn: Health): HPBand | null {
	return pawn.hp !== null ? bandOf(pawn.hp, pawn.maxHp) : pawn.hpBand;
}



















export function severityOf(damage: number, maxHp: number | null): number {
	if (damage <= 0) {
		return 0;
	}
	if (maxHp === null || maxHp < 1) {
		return SEVERITY_UNKNOWN;
	}

	const fraction = damage / maxHp;

	return fraction >= 1 ? 1 : Math.sqrt(fraction);
}





export const SEVERITY_UNKNOWN = 0.4;
















export function hurt(band: HPBand | null): number {
	switch (band) {
		case "bloody":
			return 0.45;
		case "veryBloody":
			return 0.75;
		case "nearDeath":
			return 1;
		default:
			return 0;
	}
}














export const BEAT_NONE = 0;
export const BEAT_SLOW = 1;
export const BEAT_HEART = 2;


export function beats(band: HPBand | null): number {
	switch (band) {
		case "veryBloody":
			return BEAT_SLOW;
		case "nearDeath":
			return BEAT_HEART;
		default:
			return BEAT_NONE;
	}
}











export function bleeds(band: HPBand | null): boolean {
	return band === "veryBloody" || band === "nearDeath" || band === "dead";
}











export const BLOOD_FRESH: readonly [number, number, number] = [1, 0.13, 0.1];
export const BLOOD_DRIED: readonly [number, number, number] = [0.34, 0.06, 0.05];




export const BLOOD_VARIANTS = 9;





export function bloodSprite(variant: number): string {
	return `/images/blood/${(((variant % BLOOD_VARIANTS) + BLOOD_VARIANTS) % BLOOD_VARIANTS) + 1}.webp`;
}





export function seed(text: string): number {
	let value = 0x811c9dc5;
	for (let i = 0; i < text.length; i++) {
		value ^= text.charCodeAt(i);
		value = Math.imul(value, 0x01000193) >>> 0;
	}

	return value >>> 0;
}





export const BEAT_PERIOD = 1050;
export const SLOW_PERIOD = 1800;


const SLOW_PEAK = 0.55;
const BEAT_PEAK = 1;












const THUMPS: readonly (readonly [number, number])[] = [[0, 1], [220, 0.78]];
const ATTACK = 50;
const DECAY = 150;









export function heartbeat(now: number, period: number): number {
	const t = phase(now, period);

	let peak = 0;
	for (const [at, height] of THUMPS) {
		const since = t - at;
		if (since < 0 || since > ATTACK + DECAY) {
			continue;
		}

		const value = height * (since < ATTACK ? since / ATTACK : 1 - (since - ATTACK) / DECAY);
		if (value > peak) {
			peak = value;
		}
	}

	return peak;
}



export function slowBeat(now: number): number {
	return SLOW_PEAK * heartbeat(now, SLOW_PERIOD);
}

export function fastBeat(now: number): number {
	return BEAT_PEAK * heartbeat(now, BEAT_PERIOD);
}




function phase(now: number, period: number): number {
	return ((now % period) + period) % period;
}










export function splatters(severity: number): number {
	if (severity >= HEAVY) {
		return 3;
	}
	if (severity >= SOLID) {
		return 2;
	}

	return 1;
}




const SOLID = 0.5;
const HEAVY = 0.8;




export const DEATH_SPLATTERS = 4;
