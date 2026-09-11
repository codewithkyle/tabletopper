import type { GrowableBuffer } from "./buffer.ts";
import { createGrowableBuffer } from "./buffer.ts";
export interface Attribute {
	size: 1 | 2 | 3 | 4;
}
export interface QuadBatch {
	readonly data: Float32Array;
	readonly stride: number;
	count: number;
	begin(): void;
	reserve(instances: number): void;
	cursor(): number;
	upload(): void;
	draw(): void;
	dispose(): void;
}
const CORNERS = new Float32Array([0, 0, 1, 0, 0, 1, 1, 1]);
export function strideOf(layout: readonly Attribute[]): number {
	let floats = 0;
	for (const attribute of layout) {
		floats += attribute.size;
	}
	return floats;
}
export function createQuadBatch(
	gl: WebGL2RenderingContext, layout: readonly Attribute[], initial: number,
): QuadBatch {
	const stride = strideOf(layout);
	const store: GrowableBuffer = createGrowableBuffer(Math.max(1, initial) * stride);
	const vao = gl.createVertexArray();
	gl.bindVertexArray(vao);
	const corners = gl.createBuffer();
	gl.bindBuffer(gl.ARRAY_BUFFER, corners);
	gl.bufferData(gl.ARRAY_BUFFER, CORNERS, gl.STATIC_DRAW);
	gl.enableVertexAttribArray(0);
	gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
	const instances = gl.createBuffer();
	gl.bindBuffer(gl.ARRAY_BUFFER, instances);
	const bytes = stride * 4;
	let offset = 0;
	for (let i = 0; i < layout.length; i++) {
		const location = i + 1;
		gl.enableVertexAttribArray(location);
		gl.vertexAttribPointer(location, layout[i].size, gl.FLOAT, false, bytes, offset);
		gl.vertexAttribDivisor(location, 1);
		offset += layout[i].size * 4;
	}
	gl.bindVertexArray(null);
	gl.bindBuffer(gl.ARRAY_BUFFER, null);
	return {
		get data() {
			return store.data;
		},
		stride,
		count: 0,
		begin() {
			this.count = 0;
		},
		reserve(wanted) {
			store.reserve(wanted * stride);
		},
		cursor() {
			const at = this.count * stride;
			store.reserve(at + stride);
			this.count++;
			return at;
		},
		upload() {
			store.upload(gl, instances, this.count * stride);
		},
		draw() {
			if (this.count === 0) {
				return;
			}
			gl.bindVertexArray(vao);
			gl.drawArraysInstanced(gl.TRIANGLE_STRIP, 0, 4, this.count);
			gl.bindVertexArray(null);
		},
		dispose() {
			gl.deleteVertexArray(vao);
			gl.deleteBuffer(corners);
			gl.deleteBuffer(instances);
		},
	};
}
