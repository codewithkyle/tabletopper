export interface Slot {
	layer: number;
	w: number;
	h: number;
	used: number;
}
export class Slots {
	private readonly byKey = new Map<string, Slot>();
	private readonly free: number[] = [];
	private frame = 0;
	readonly capacity: number;
	constructor(capacity: number) {
		this.capacity = capacity;
		for (let i = capacity - 1; i >= 0; i--) {
			this.free.push(i);
		}
	}
	tick(): void {
		this.frame++;
	}
	get(key: string): Slot | undefined {
		const slot = this.byKey.get(key);
		if (slot) {
			slot.used = this.frame;
		}
		return slot;
	}
	has(key: string): boolean {
		return this.byKey.has(key);
	}
	claim(key: string, w: number, h: number): Slot | null {
		const existing = this.byKey.get(key);
		if (existing) {
			existing.w = w;
			existing.h = h;
			existing.used = this.frame;
			return existing;
		}
		let layer = this.free.pop();
		if (layer === undefined) {
			layer = this.evict();
		}
		if (layer === undefined) {
			return null;
		}
		const slot: Slot = { layer, w, h, used: this.frame };
		this.byKey.set(key, slot);
		return slot;
	}
	private evict(): number | undefined {
		let victim: string | undefined;
		let oldest = Infinity;
		for (const [key, slot] of this.byKey) {
			if (slot.used === this.frame) {
				continue;
			}
			if (slot.used < oldest) {
				oldest = slot.used;
				victim = key;
			}
		}
		if (victim === undefined) {
			return undefined;
		}
		const slot = this.byKey.get(victim);
		this.byKey.delete(victim);
		return slot?.layer;
	}
	get size(): number {
		return this.byKey.size;
	}
}
export interface TextureArray {
	readonly texture: WebGLTexture;
	readonly capacity: number;
	tick(): void;
	get(key: string): Slot | undefined;
	upload(key: string, source: TexImageSource, w: number, h: number): Slot | null;
	dispose(): void;
}
export function createTextureArray(
	gl: WebGL2RenderingContext, size: number, layers: number,
): TextureArray {
	const capacity = Math.min(layers, gl.getParameter(gl.MAX_ARRAY_TEXTURE_LAYERS) as number);
	const texture = gl.createTexture();
	gl.bindTexture(gl.TEXTURE_2D_ARRAY, texture);
	gl.texStorage3D(gl.TEXTURE_2D_ARRAY, 1, gl.RGBA8, size, size, capacity);
	gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
	gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
	gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
	gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
	gl.bindTexture(gl.TEXTURE_2D_ARRAY, null);
	const slots = new Slots(capacity);
	return {
		texture,
		capacity,
		tick() {
			slots.tick();
		},
		get(key) {
			return slots.get(key);
		},
		upload(key, source, w, h) {
			const slot = slots.claim(key, w, h);
			if (!slot) {
				return null;
			}
			gl.bindTexture(gl.TEXTURE_2D_ARRAY, texture);
			gl.texSubImage3D(gl.TEXTURE_2D_ARRAY, 0, 0, 0, slot.layer, w, h, 1, gl.RGBA, gl.UNSIGNED_BYTE, source);
			gl.bindTexture(gl.TEXTURE_2D_ARRAY, null);
			return slot;
		},
		dispose() {
			gl.deleteTexture(texture);
		},
	};
}
