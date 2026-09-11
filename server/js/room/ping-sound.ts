


































const LOW = 1046.5;
const HIGH = 1568;











const NOTE = 0.055;
const GAP = 0.008;




const ATTACK = 0.004;
const RELEASE = 0.05;



const FLOOR = 0.0001;




export const BLIP = NOTE * 2 + RELEASE;




export const PEAK = 0.2;








export const FULL = 100;








const MIN_GAP = BLIP * 1000;






export interface AudioRamp {
	setValueAtTime(value: number, startTime: number): unknown;
	linearRampToValueAtTime(value: number, endTime: number): unknown;
	exponentialRampToValueAtTime(value: number, endTime: number): unknown;
}












export function gainFor(volume: number): number {
	const percent = Number.isFinite(volume) ? Math.min(Math.max(volume, 0), FULL) : FULL;
	if (percent <= 0) {
		return 0;
	}

	const share = percent / FULL;

	return PEAK * share * share;
}
























export function voice(pitch: AudioRamp, level: AudioRamp, at: number, peak: number): void {
	pitch.setValueAtTime(LOW, at);
	pitch.setValueAtTime(HIGH, at + NOTE);

	level.setValueAtTime(0, at);
	level.linearRampToValueAtTime(peak, at + ATTACK);
	level.setValueAtTime(peak, at + NOTE - GAP);
	level.linearRampToValueAtTime(FLOOR, at + NOTE);
	level.linearRampToValueAtTime(peak, at + NOTE + ATTACK);
	level.setValueAtTime(peak, at + NOTE * 2);
	level.exponentialRampToValueAtTime(FLOOR, at + BLIP);
}

export interface PingSound {
	
	
	
	
	volume(percent: number): void;

	
	
	
	
	play(): void;

	stop(): void;
}

export function newPingSound(): PingSound {
	
	
	
	let peak = gainFor(FULL);

	let context: AudioContext | null = null;

	
	
	
	
	
	
	let last = Number.NEGATIVE_INFINITY;

	
	
	
	function resumed(): AudioContext | null {
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

		return context;
	}

	function wake(): void {
		void context?.resume().catch(() => {});
	}

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

			const ctx = resumed();
			if (ctx === null || ctx.state !== "running") {
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

		stop() {
			document.removeEventListener("pointerdown", wake);
			void context?.close().catch(() => {});
			context = null;
		},
	};
}
