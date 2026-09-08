// The thin layer between this renderer and WebGL2: get a context, build a
// program, find its uniforms. Nothing here knows what a map or a grid is.
//
// THERE IS NO getError ANYWHERE IN THIS RENDERER, and that is a performance
// decision rather than an oversight. gl.getError forces the driver to finish
// what it is doing before it can answer, which turns an asynchronous command
// stream into a synchronous one -- the fourth performance rule in the overview.
// Compile and link status are different: they are read once per program at
// startup, they are the only two failures that produce a black screen with no
// other symptom, and they are checked below unconditionally.

// contextOptions is the whole configuration of the drawing buffer.
//
// alpha: false because the canvas covers the table completely and a compositor
// that has been told the layer is opaque does not have to blend it.
// depth and stencil: false because everything here is drawn back to front in
// one pass and there is nothing to test against.
// antialias: false because there is no geometry to alias -- the tiles are axis
// aligned quads and the grid antialiases itself in its own fragment shader,
// where it can be exactly one device pixel wide instead of whatever the
// driver's multisampling happens to give it.
// preserveDrawingBuffer: false so the browser may discard the buffer after
// compositing rather than keeping a copy nothing reads.
const contextOptions: WebGLContextAttributes = {
	alpha: false,
	antialias: false,
	depth: false,
	stencil: false,
	preserveDrawingBuffer: false,
	powerPreference: "high-performance",
};

// createContext answers null rather than throwing. A browser with no WebGL2 is
// a browser somebody is sitting in front of, and the room page's other half --
// the menus, the windows, the player list -- works perfectly well without a
// canvas. The caller shows the message and carries on.
export function createContext(canvas: HTMLCanvasElement): WebGL2RenderingContext | null {
	return canvas.getContext("webgl2", contextOptions);
}

// createProgram compiles, links, and throws with the driver's own log on
// failure. The log is the only useful thing about a shader error and it is
// thrown rather than logged so a broken program cannot half-run.
export function createProgram(gl: WebGL2RenderingContext, vertexSource: string, fragmentSource: string): WebGLProgram {
	const vertex = compile(gl, gl.VERTEX_SHADER, vertexSource);
	const fragment = compile(gl, gl.FRAGMENT_SHADER, fragmentSource);

	const program = gl.createProgram();
	gl.attachShader(program, vertex);
	gl.attachShader(program, fragment);
	gl.linkProgram(program);

	// THE SHADERS ARE DELETED IMMEDIATELY AFTER LINKING. The program holds its
	// own reference until it is itself deleted, so this frees the compiled
	// source rather than the linked code.
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

// uniforms looks every name up once, at startup.
//
// A NAME THAT IS NOT THERE THROWS, which catches the mistake this is for: a
// uniform the shader declares and never reads is optimised out by the compiler,
// so a typo and a dead uniform look identical at run time and both silently do
// nothing every frame forever.
export function uniforms<K extends string>(gl: WebGL2RenderingContext, program: WebGLProgram, names: readonly K[]): Record<K, WebGLUniformLocation> {
	const found = {} as Record<K, WebGLUniformLocation>;

	for (const name of names) {
		const location = gl.getUniformLocation(program, name);
		if (!location) {
			throw new Error(`uniform ${name} is not active in this program`);
		}

		found[name] = location;
	}

	return found;
}

// fullscreenTriangle is the geometry every screen-space pass uses: ONE triangle
// large enough to cover the clip cube, not two making a quad. A quad's diagonal
// is a seam that the rasteriser visits twice and across which derivatives --
// which is to say fwidth, which is to say the grid's line width -- are computed
// from different neighbourhoods. One triangle has no interior edge.
export function fullscreenTriangle(gl: WebGL2RenderingContext): WebGLVertexArrayObject {
	const vao = gl.createVertexArray();
	gl.bindVertexArray(vao);

	const buffer = gl.createBuffer();
	gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
	gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW);
	gl.enableVertexAttribArray(0);
	gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);

	gl.bindVertexArray(null);
	gl.bindBuffer(gl.ARRAY_BUFFER, null);

	return vao;
}
