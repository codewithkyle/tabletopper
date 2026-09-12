const IN_FLIGHT = 8;
const RETRY_FLOOR = 1_000;
const RETRY_CEILING = 60_000;
const UPLOADS_PER_FRAME = 4;
export interface Loader {
	begin(): void;
	want(key: string, url: string, priority: number): void;
	end(): void;
	drain(upload: (key: string, bitmap: ImageBitmap) => void): number;
	fetched(): number;
	stop(): void;
}
interface Wanted {
	key: string;
	url: string;
	priority: number;
}
export type Decode = (blob: Blob) => Promise<ImageBitmap | null>;
function decodeAsIs(blob: Blob): Promise<ImageBitmap | null> {
	return createImageBitmap(blob, { premultiplyAlpha: "none", colorSpaceConversion: "none" });
}
export function newLoader(invalidate: () => void, decode: Decode = decodeAsIs): Loader {
	const queue: Wanted[] = [];
	const inFlight = new Map<string, AbortController>();
	const ready: { key: string; bitmap: ImageBitmap }[] = [];
	const missing = new Set<string>();
	const wantedThisFrame = new Set<string>();
	const failures = new Map<string, number>();
	const retryAt = new Map<string, number>();
	function failed(key: string): void {
		const n = (failures.get(key) ?? 0) + 1;
		failures.set(key, n);
		retryAt.set(key, performance.now() + Math.min(RETRY_CEILING, RETRY_FLOOR * 2 ** (n - 1)));
	}
	function recovered(key: string): void {
		failures.delete(key);
		retryAt.delete(key);
	}
	let total = 0;
	let stopped = false;
	function start(item: Wanted): void {
		const controller = new AbortController();
		inFlight.set(item.key, controller);
		total++;
		let decoding = false;
		fetch(item.url, { credentials: "same-origin", signal: controller.signal })
			.then((response) => {
				if (response.status === 404) {
					missing.add(item.key);
					return null;
				}
				if (!response.ok) {
					failed(item.key);
					return null;
				}
				return response.blob();
			})
			.then((blob) => {
				if (!blob) {
					return null;
				}
				decoding = true;
				return decode(blob);
			})
			.then((bitmap) => {
				if (bitmap) {
					recovered(item.key);
					ready.push({ key: item.key, bitmap });
					invalidate();
				}
			})
			.catch((err: unknown) => {
				if (err instanceof DOMException && err.name === "AbortError") {
					return;
				}
				if (decoding) {
					missing.add(item.key);
				} else {
					failed(item.key);
				}
			})
			.finally(() => {
				inFlight.delete(item.key);
			});
	}
	return {
		begin() {
			queue.length = 0;
			wantedThisFrame.clear();
		},
		want(key, url, priority) {
			wantedThisFrame.add(key);
			if (missing.has(key) || inFlight.has(key)) {
				return;
			}
			const until = retryAt.get(key);
			if (until !== undefined && until > performance.now()) {
				return;
			}
			queue.push({ key, url, priority });
		},
		end() {
			if (stopped || queue.length === 0) {
				return;
			}
			queue.sort((a, b) => a.priority - b.priority);
			let spare = IN_FLIGHT - inFlight.size;
			for (const [key, controller] of inFlight) {
				if (spare >= queue.length) {
					break;
				}
				if (wantedThisFrame.has(key)) {
					continue;
				}
				controller.abort();
				inFlight.delete(key);
				spare++;
			}
			for (const item of queue) {
				if (inFlight.size >= IN_FLIGHT) {
					break;
				}
				if (!inFlight.has(item.key)) {
					start(item);
				}
			}
		},
		drain(upload) {
			const count = Math.min(ready.length, UPLOADS_PER_FRAME);
			for (let i = 0; i < count; i++) {
				const item = ready[i];
				upload(item.key, item.bitmap);
				item.bitmap.close();
			}
			ready.splice(0, count);
			return ready.length;
		},
		fetched: () => total,
		stop() {
			stopped = true;
			for (const controller of inFlight.values()) {
				controller.abort();
			}
			inFlight.clear();
			for (const item of ready) {
				item.bitmap.close();
			}
			ready.length = 0;
		},
	};
}
