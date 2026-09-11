import type { Attribute, QuadBatch } from "./quads.ts";
import { strideOf } from "./quads.ts";
export interface FakeBatch extends QuadBatch {
	instance(index: number): number[];
	uploads: number;
	draws: number;
}
export function createFakeBatch(layout: readonly Attribute[]): FakeBatch {
	const stride = strideOf(layout);
	let store = new Float32Array(stride * 8);
	return {
		get data() {
			return store;
		},
		stride,
		count: 0,
		uploads: 0,
		draws: 0,
		begin() {
			this.count = 0;
		},
		reserve(instances) {
			grow(instances * stride);
		},
		cursor() {
			const at = this.count * stride;
			grow(at + stride);
			this.count++;
			return at;
		},
		upload() {
			this.uploads++;
		},
		draw() {
			this.draws++;
		},
		dispose() {},
		instance(index) {
			return Array.from(store.subarray(index * stride, (index + 1) * stride));
		},
	};
	function grow(needed: number): void {
		if (needed <= store.length) {
			return;
		}
		let size = store.length;
		while (size < needed) {
			size *= 2;
		}
		const grown = new Float32Array(size);
		grown.set(store);
		store = grown;
	}
}
