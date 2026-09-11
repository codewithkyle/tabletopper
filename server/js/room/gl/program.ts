export interface Program<K extends string> {
	readonly handle: WebGLProgram;
	readonly at: Record<K, WebGLUniformLocation>;
	use(): void;
	dispose(): void;
}
export function createProgram<K extends string>(
	gl: WebGL2RenderingContext, vertex: string, fragment: string, uniforms: readonly K[],
): Program<K> {
	const handle = link(gl, vertex, fragment);
	const at = {} as Record<K, WebGLUniformLocation>;
	for (const name of uniforms) {
		const location = gl.getUniformLocation(handle, name);
		if (!location) {
			gl.deleteProgram(handle);
			throw new Error(`uniform ${name} is not active in this program`);
		}
		at[name] = location;
	}
	return {
		handle,
		at,
		use() {
			gl.useProgram(handle);
		},
		dispose() {
			gl.deleteProgram(handle);
		},
	};
}
function link(gl: WebGL2RenderingContext, vertexSource: string, fragmentSource: string): WebGLProgram {
	const vertex = compile(gl, gl.VERTEX_SHADER, vertexSource);
	const fragment = compile(gl, gl.FRAGMENT_SHADER, fragmentSource);
	const program = gl.createProgram();
	gl.attachShader(program, vertex);
	gl.attachShader(program, fragment);
	gl.linkProgram(program);
	gl.deleteShader(vertex);
	gl.deleteShader(fragment);
	if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
		const log = gl.getProgramInfoLog(program);
		gl.deleteProgram(program);
		throw new Error(`link failed: ${log ?? "no log"}`);
	}
	return program;
}
function compile(gl: WebGL2RenderingContext, type: number, source: string): WebGLShader {
	const shader = gl.createShader(type);
	if (!shader) {
		throw new Error("could not create a shader");
	}
	gl.shaderSource(shader, source);
	gl.compileShader(shader);
	if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
		const log = gl.getShaderInfoLog(shader);
		gl.deleteShader(shader);
		throw new Error(`compile failed: ${log ?? "no log"}`);
	}
	return shader;
}
