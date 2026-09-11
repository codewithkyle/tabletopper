const contextOptions: WebGLContextAttributes = {
	alpha: false,
	antialias: false,
	depth: false,
	stencil: false,
	preserveDrawingBuffer: false,
	powerPreference: "high-performance",
};
export function createContext(canvas: HTMLCanvasElement): WebGL2RenderingContext | null {
	return canvas.getContext("webgl2", contextOptions);
}
