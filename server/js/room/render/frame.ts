const SAMPLES = 600;
export interface Timings {
	average: number;
	p95: number;
	samples: number;
}
export interface Frames {
	invalidate(): void;
	timings(): Timings;
	resetTimings(): void;
	stop(): void;
}
export interface FrameOptions {
	mount: HTMLElement;
	canvas: HTMLCanvasElement;
	render(): boolean;
	resized?(): void;
}
export function startFrames(options: FrameOptions): Frames {
	const { mount, canvas, render } = options;
	const times = new Float64Array(SAMPLES);
	let count = 0;
	let next = 0;
	let pending = 0;
	let stopped = false;
	function frame(): void {
		pending = 0;
		if (stopped) {
			return;
		}
		const started = performance.now();
		const again = render();
		const elapsed = performance.now() - started;
		times[next] = elapsed;
		next = (next + 1) % SAMPLES;
		if (count < SAMPLES) {
			count++;
		}
		if (again) {
			request();
		}
	}
	function request(): void {
		if (pending === 0 && !stopped) {
			pending = requestAnimationFrame(frame);
		}
	}
	const observer = new ResizeObserver(() => {
		const dpr = window.devicePixelRatio || 1;
		const rect = mount.getBoundingClientRect();
		const width = Math.max(1, Math.round(rect.width * dpr));
		const height = Math.max(1, Math.round(rect.height * dpr));
		if (canvas.width !== width || canvas.height !== height) {
			canvas.width = width;
			canvas.height = height;
		}
		options.resized?.();
		request();
	});
	observer.observe(mount);
	let ratioQuery: MediaQueryList | null = null;
	function watchRatio(): void {
		ratioQuery?.removeEventListener("change", onRatioChange);
		ratioQuery = window.matchMedia(`(resolution: ${window.devicePixelRatio || 1}dppx)`);
		ratioQuery.addEventListener("change", onRatioChange);
	}
	function onRatioChange(): void {
		watchRatio();
		const dpr = window.devicePixelRatio || 1;
		const rect = mount.getBoundingClientRect();
		canvas.width = Math.max(1, Math.round(rect.width * dpr));
		canvas.height = Math.max(1, Math.round(rect.height * dpr));
		options.resized?.();
		request();
	}
	watchRatio();
	return {
		invalidate: request,
		timings() {
			if (count === 0) {
				return { average: 0, p95: 0, samples: 0 };
			}
			const sorted = Array.from(times.subarray(0, count)).sort((a, b) => a - b);
			let total = 0;
			for (const value of sorted) {
				total += value;
			}
			return {
				average: total / count,
				p95: sorted[Math.min(count - 1, Math.floor(count * 0.95))] ?? 0,
				samples: count,
			};
		},
		resetTimings() {
			count = 0;
			next = 0;
		},
		stop() {
			stopped = true;
			if (pending !== 0) {
				cancelAnimationFrame(pending);
				pending = 0;
			}
			observer.disconnect();
			ratioQuery?.removeEventListener("change", onRatioChange);
		},
	};
}
