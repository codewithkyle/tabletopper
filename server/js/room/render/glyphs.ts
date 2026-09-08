// THE ONLY TEXT ON THE CANVAS, and it is fourteen characters.
//
// A distance label says "30 ft." and nothing else, so the atlas holds the ten
// digits, a space, an f, a t and a full stop. That is the whole vocabulary, and
// path.ts's distanceLabel is written so it cannot produce anything outside it --
// a character with no quad renders a hole rather than an error.
//
// WHY AN ATLAS AND NOT A DOM ELEMENT. A label follows the pointer during a
// drag, which is the one moment the frame budget is actually spent, and a DOM
// element positioned per frame is a layout read and a style write inside the
// render loop -- the third performance rule, broken by the one feature that
// most needs it kept. Six quads out of one texture cost nothing.
//
// WHY AN ATLAS AND NOT ONE CANVAS TEXTURE PER LABEL. The text changes every
// time the dragged cell changes, which during a fast drag is several times a
// second per dragging player; re-rasterising a canvas and re-uploading it is a
// synchronous main-thread upload on exactly that schedule. The characters are
// rasterised once, at startup, and a label is an index into them.

// GLYPHS is the vocabulary, in the order they are packed.
export const GLYPHS = "0123456789 ft.";

// GLYPH_PIXELS is the height each character is rasterised at. Labels are drawn
// at a fixed size in DEVICE pixels rather than scaled with the camera -- a
// ruler that shrank as you zoomed out would stop being readable exactly when
// the distance got long enough to need reading -- so this is the size they are
// actually shown at on a HiDPI screen, and no filtering happens at all.
const GLYPH_PIXELS = 48;

// PADDING keeps a character's antialiased edge from bleeding into its
// neighbour's cell when the sampler filters across the boundary.
const PADDING = 2;

export interface Glyph {
	// The character's rectangle in the atlas, in texture coordinates.
	u0: number;
	v0: number;
	u1: number;
	v1: number;

	// width is how wide to draw it, in the same units GLYPH_PIXELS is in, and
	// advance is how far to move afterwards. They differ only for a space,
	// which is drawn as nothing at all.
	width: number;
	advance: number;
}

export interface GlyphAtlas {
	texture: WebGLTexture;

	// height is the line's height in atlas pixels, which is what a caller
	// scales a label by.
	height: number;

	// measure is the width of a whole label at height 1, so a caller can centre
	// it before laying it out.
	measure(text: string): number;

	// get is one character, or null for anything not in GLYPHS.
	get(char: string): Glyph | null;

	dispose(): void;
}

// createGlyphAtlas rasterises the vocabulary once. It answers null when there
// is no 2D canvas to draw on, which is a browser the renderer is not running
// in anyway -- the caller draws no labels and everything else works.
export function createGlyphAtlas(gl: WebGL2RenderingContext): GlyphAtlas | null {
	const measurer = document.createElement("canvas").getContext("2d");
	if (!measurer) {
		return null;
	}

	const font = `600 ${GLYPH_PIXELS}px system-ui, sans-serif`;
	measurer.font = font;

	// Widths first, so the texture is exactly as wide as it needs to be. A
	// fixed cell per character would be right for the digits and wrong for the
	// full stop, and a label whose full stop sat in the middle of its own empty
	// square is a label that looks broken.
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

	// WHITE ON TRANSPARENT, because the pass tints it. A label is drawn in the
	// dragging player's colour, and baking a colour in here would mean an atlas
	// per player.
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
			// A space is an advance and no quad, which saves a draw of nothing
			// and keeps "30 ft." at five quads rather than six.
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
