import { FULL, level, newSpeaker } from "./sound.ts";
import type { AudioRamp, Speaker } from "./sound.ts";
export const NOTES = [659.25, 987.77, 1318.51];
export const RINGS = [0.2, 0.2, 0.5];
export const STEP = 0.09;
const ATTACK = 0.006;
const FLOOR = 0.0001;
export const VOICES = NOTES.length;
export const CHIME = STEP * (VOICES - 1) + RINGS[VOICES - 1];
export const PEAK = 0.16;
export { FULL };
export type { AudioRamp };
const MIN_GAP = CHIME * 1000;
export function gainFor(volume: number): number {
	return level(volume, PEAK);
}
export function strike(pitch: AudioRamp, gain: AudioRamp, note: number, at: number, peak: number): number {
	const start = at + STEP * note;
	const end = start + RINGS[note];
	pitch.setValueAtTime(NOTES[note], start);
	gain.setValueAtTime(0, start);
	gain.linearRampToValueAtTime(peak, start + ATTACK);
	gain.exponentialRampToValueAtTime(FLOOR, end);
	return end;
}
export interface TurnSound {
	volume(percent: number): void;
	play(): void;
}
export function newTurnSound(speaker: Speaker = newSpeaker()): TurnSound {
	let peak = gainFor(FULL);
	let last = Number.NEGATIVE_INFINITY;
	return {
		volume(percent) {
			peak = gainFor(percent);
		},
		play() {
			if (peak <= 0) {
				return;
			}
			const now = performance.now();
			if (now - last < MIN_GAP) {
				return;
			}
			const ctx = speaker.running();
			if (ctx === null) {
				return;
			}
			last = now;
			try {
				const at = ctx.currentTime;
				for (let note = 0; note < VOICES; note++) {
					const osc = ctx.createOscillator();
					const gain = ctx.createGain();
					osc.type = "triangle";
					const end = strike(osc.frequency, gain.gain, note, at, peak);
					osc.connect(gain).connect(ctx.destination);
					osc.start(at + STEP * note);
					osc.stop(end);
				}
			} catch {
			}
		},
	};
}
