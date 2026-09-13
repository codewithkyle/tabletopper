import { FULL, level, newSpeaker } from "./sound.ts";
import type { AudioRamp, Speaker } from "./sound.ts";
const LOW = 1046.5;
const HIGH = 1568;
const NOTE = 0.055;
const GAP = 0.008;
const ATTACK = 0.004;
const RELEASE = 0.05;
const FLOOR = 0.0001;
export const BLIP = NOTE * 2 + RELEASE;
export const PEAK = 0.2;
export { FULL };
export type { AudioRamp };
const MIN_GAP = BLIP * 1000;
export function gainFor(volume: number): number {
	return level(volume, PEAK);
}
export function voice(pitch: AudioRamp, gain: AudioRamp, at: number, peak: number): void {
	pitch.setValueAtTime(LOW, at);
	pitch.setValueAtTime(HIGH, at + NOTE);
	gain.setValueAtTime(0, at);
	gain.linearRampToValueAtTime(peak, at + ATTACK);
	gain.setValueAtTime(peak, at + NOTE - GAP);
	gain.linearRampToValueAtTime(FLOOR, at + NOTE);
	gain.linearRampToValueAtTime(peak, at + NOTE + ATTACK);
	gain.setValueAtTime(peak, at + NOTE * 2);
	gain.exponentialRampToValueAtTime(FLOOR, at + BLIP);
}
export interface PingSound {
	volume(percent: number): void;
	play(): void;
}
export function newPingSound(speaker: Speaker = newSpeaker()): PingSound {
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
				const osc = ctx.createOscillator();
				const gain = ctx.createGain();
				osc.type = "sine";
				voice(osc.frequency, gain.gain, at, peak);
				osc.connect(gain).connect(ctx.destination);
				osc.start(at);
				osc.stop(at + BLIP);
			} catch {
			}
		},
	};
}
