export interface GrowableBuffer {
	readonly data: Float32Array;
	reserve(floats: number): void;
	upload(gl: WebGL2RenderingContext, buffer: WebGLBuffer, floats: number): void;
}
export function createGrowableBuffer(floats: number): GrowableBuffer {
	let data = new Float32Array(Math.max(1, floats));
	return {
		get data() {
			return data;
		},
		reserve(needed) {
			if (needed <= data.length) {
				return;
			}
			let size = data.length;
			while (size < needed) {
				size *= 2;
			}
			const grown = new Float32Array(size);
			grown.set(data);
			data = grown;
		},
		upload(gl, buffer, used) {
			gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
			gl.bufferData(gl.ARRAY_BUFFER, data.subarray(0, used), gl.DYNAMIC_DRAW);
			gl.bindBuffer(gl.ARRAY_BUFFER, null);
		},
	};
}
