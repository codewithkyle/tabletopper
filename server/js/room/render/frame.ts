// The frame loop, and it is the reason an open room costs nothing.
//
// A CANVAS APPLICATION THAT CALLS requestAnimationFrame IN A CIRCLE RENDERS
// FOREVER. Six players with a room open on a second monitor would each burn a
// core drawing the same unchanged map sixty times a second, with the fans to
// go with it. So the loop here is not a loop: invalidate() asks for ONE frame,
// that frame renders, and it asks for another only if the render says something
// is still moving -- a drag in progress, a tile waiting to be uploaded, a
// crossfade partway through.
//
// THE RENDER FUNCTION'S RETURN VALUE IS THAT ANSWER. True means "again", false
// means "I am finished until somebody tells me otherwise". Nothing else decides
// it, so a pass that starts an animation cannot forget to keep the loop alive
// and a pass that finishes one cannot leave it running.

// SAMPLES is ten seconds at 60Hz, which is the benchmark's window. The ring is
// allocated once and never grows.
const SAMPLES = 600;

export interface Timings {
	average: number;
	p95: number;
	samples: number;
}

export interface Frames {
	// invalidate asks for a frame. Calling it a hundred times before the next
	// one still renders once.
	invalidate(): void;

	// timings reports CPU time spent inside the render function, which is what
	// this renderer controls -- not the frame interval, which is the display's.
	timings(): Timings;
	resetTimings(): void;

	stop(): void;
}

export interface FrameOptions {
	// mount is watched for size changes, and canvas is sized to match it times
	// the device pixel ratio.
	mount: HTMLElement;
	canvas: HTMLCanvasElement;

	// render draws one frame and answers whether another is wanted.
	render(): boolean;

	// resized runs before the first render at a new size, so a pass holding a
	// viewport-derived value can recompute it.
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

	// THE OBSERVER SIZES THE BACKING STORE, NOT THE ELEMENT. The canvas is laid
	// out by CSS at whatever the mount is; width and height are how many device
	// pixels that area is drawn with, and they are a different number on a
	// retina display and after a browser zoom. Setting them is also what
	// invalidates the drawing buffer, so a resize always renders.
	const observer = new ResizeObserver(() => {
		const dpr = window.devicePixelRatio || 1;
		const rect = mount.getBoundingClientRect();
		const width = Math.max(1, Math.round(rect.width * dpr));
		const height = Math.max(1, Math.round(rect.height * dpr));

		// The assignment is guarded because writing width or height CLEARS the
		// drawing buffer even when the value does not change, but resized and
		// the frame are not: the observer's first call arrives with the canvas
		// at its default 300 by 150, and a mount that happens to be that size
		// would otherwise never establish a viewport.
		if (canvas.width !== width || canvas.height !== height) {
			canvas.width = width;
			canvas.height = height;
		}

		options.resized?.();
		request();
	});
	observer.observe(mount);

	// A move between a 1x and a 2x display changes devicePixelRatio without
	// changing any element's size, so the ResizeObserver never fires. This
	// media query is the event for that, and it is re-armed each time because
	// the query it watches is built from the ratio that has just changed.
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
