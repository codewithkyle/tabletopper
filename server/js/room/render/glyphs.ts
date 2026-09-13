export const GLYPHS = "0123456789 ft.mikhexsq";
const GLYPH_PIXELS = 48;
const PADDING = 2;
export interface Glyph {
	u0: number;
	v0: number;
	u1: number;
	v1: number;
	width: number;
	advance: number;
}
export interface GlyphAtlas {
	texture: WebGLTexture;
	height: number;
	measure(text: string): number;
	get(char: string): Glyph | null;
	dispose(): void;
}
export function createGlyphAtlas(gl: WebGL2RenderingContext): GlyphAtlas | null {
	const measurer = document.createElement("canvas").getContext("2d");
	if (!measurer) {
		return null;
	}
	const font = `600 ${GLYPH_PIXELS}px system-ui, sans-serif`;
	measurer.font = font;
	const widths = [...GLYPHS].map((char) => Math.ceil(measurer.measureText(char).width));
	const total = widths.reduce((sum, width) => sum + width + PADDING * 2, 0);
	const height = Math.ceil(GLYPH_PIXELS * 1.35);
	const canvas = document.createElement("canvas");
	canvas.width = Math.max(1, total);
	canvas.height = height;
	const ctx = canvas.getContext("2d");
	if (!ctx) {
		return null;
	}
	ctx.font = font;
	ctx.fillStyle = "rgb(255 255 255)";
	ctx.textAlign = "left";
	ctx.textBaseline = "middle";
	const glyphs = new Map<string, Glyph>();
	let x = 0;
	for (let i = 0; i < GLYPHS.length; i++) {
		const char = GLYPHS[i];
		const width = widths[i];
		ctx.fillText(char, x + PADDING, height / 2);
		glyphs.set(char, {
			u0: (x + PADDING) / canvas.width,
			v0: 0,
			u1: (x + PADDING + width) / canvas.width,
			v1: 1,
			width: char === " " ? 0 : width / height,
			advance: (width + PADDING) / height,
		});
		x += width + PADDING * 2;
	}
	const texture = gl.createTexture();
	gl.bindTexture(gl.TEXTURE_2D, texture);
	gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, canvas.width, canvas.height, 0, gl.RGBA, gl.UNSIGNED_BYTE, canvas);
	gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
	gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
	gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
	gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
	gl.bindTexture(gl.TEXTURE_2D, null);
	return {
		texture,
		height,
		measure(text) {
			let width = 0;
			for (const char of text) {
				width += glyphs.get(char)?.advance ?? 0;
			}
			return width;
		},
		get: (char) => glyphs.get(char) ?? null,
		dispose() {
			gl.deleteTexture(texture);
		},
	};
}
