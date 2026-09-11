import { createGrowableBuffer } from "./buffer.ts";
export interface VertexStream {
	bind(): void;
	upload(values: readonly number[], floats: number): void;
	dispose(): void;
}
export function createVertexStream(
	gl: WebGL2RenderingContext, size: 1 | 2 | 3 | 4, initial: number,
): VertexStream {
	const store = createGrowableBuffer(initial);
	const vao = gl.createVertexArray();
	const buffer = gl.createBuffer();
	gl.bindVertexArray(vao);
	gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
	gl.enableVertexAttribArray(0);
	gl.vertexAttribPointer(0, size, gl.FLOAT, false, 0, 0);
	gl.bindVertexArray(null);
	gl.bindBuffer(gl.ARRAY_BUFFER, null);
	return {
		bind() {
			gl.bindVertexArray(vao);
		},
		upload(values, floats) {
			store.reserve(floats);
			for (let i = 0; i < floats; i++) {
				store.data[i] = values[i];
			}
			store.upload(gl, buffer, floats);
		},
		dispose() {
			gl.deleteVertexArray(vao);
			gl.deleteBuffer(buffer);
		},
	};
}
