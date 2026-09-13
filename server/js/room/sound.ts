export const FULL = 100;
export interface AudioRamp {
	setValueAtTime(value: number, startTime: number): unknown;
	linearRampToValueAtTime(value: number, endTime: number): unknown;
	exponentialRampToValueAtTime(value: number, endTime: number): unknown;
}
export function level(volume: number, peak: number): number {
	const percent = Number.isFinite(volume) ? Math.min(Math.max(volume, 0), FULL) : FULL;
	if (percent <= 0) {
		return 0;
	}
	const share = percent / FULL;
	return peak * share * share;
}
export interface Speaker {
	running(): AudioContext | null;
	stop(): void;
}
export function newSpeaker(): Speaker {
	let context: AudioContext | null = null;
	function wake(): void {
		void context?.resume().catch(() => {});
	}
	return {
		running() {
			if (context === null) {
				try {
					context = new AudioContext();
				} catch {
					return null;
				}
			}
			if (context.state === "suspended") {
				void context.resume().catch(() => {});
				document.addEventListener("pointerdown", wake, { once: true });
			}
			return context.state === "running" ? context : null;
		},
		stop() {
			document.removeEventListener("pointerdown", wake);
			void context?.close().catch(() => {});
			context = null;
		},
	};
}
