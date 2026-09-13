import type { FrameContext } from "./frame-context.ts";
import type { GlyphAtlas } from "./glyphs.ts";
import type { Attribute, QuadBatch } from "../gl/quads.ts";
import type { Grid } from "../protocol.ts";
import type { Rgb } from "../model/types.ts";
import { blended } from "../gl/blend.ts";
import { createProgram } from "../gl/program.ts";
import { createQuadBatch } from "../gl/quads.ts";
import { fragmentSource, uniforms, vertexSource } from "./shaders/path.ts";
const LABEL_PIXELS = 13;
const LABEL_LIFT = 10;
const HALO_COLOR: Rgb = [0.04, 0.04, 0.06];
const HALO_PIXELS = 1.25;
const HALO_RING: readonly (readonly [number, number])[] = [
	[1, 0],
	[0.7071, 0.7071],
	[0, 1],
	[-0.7071, 0.7071],
	[-1, 0],
	[-0.7071, -0.7071],
	[0, -1],
	[0.7071, -0.7071],
];
const PATH_QUAD: readonly Attribute[] = [{ size: 4 }, { size: 4 }, { size: 4 }, { size: 4 }];
export interface PathPass {
	begin(worldPerPixel: number): void;
	cell(x: number, y: number, size: number, type: Grid["type"], color: Rgb, alpha: number): void;
	line(
		x0: number, y0: number, x1: number, y1: number,
		width: number, color: Rgb, alpha: number,
	): void;
	label(text: string, x: number, y: number, color: Rgb, alpha: number): void;
	draw(frame: FrameContext): void;
	dispose(): void;
}
function push(
	batch: QuadBatch,
	ox: number, oy: number, axx: number, axy: number,
	ayx: number, ayy: number, textured: number,
	color: Rgb, alpha: number,
	u0: number, v0: number, u1: number, v1: number,
): void {
	const i = batch.cursor();
	const data = batch.data;
	data[i] = ox;
	data[i + 1] = oy;
	data[i + 2] = axx;
	data[i + 3] = axy;
	data[i + 4] = ayx;
	data[i + 5] = ayy;
	data[i + 6] = textured;
	data[i + 7] = 0;
	data[i + 8] = color[0];
	data[i + 9] = color[1];
	data[i + 10] = color[2];
	data[i + 11] = alpha;
	data[i + 12] = u0;
	data[i + 13] = v0;
	data[i + 14] = u1;
	data[i + 15] = v1;
}
export function createPathPass(gl: WebGL2RenderingContext, atlas: GlyphAtlas | null): PathPass {
	const program = createProgram(gl, vertexSource, fragmentSource, uniforms);
	const batch = createQuadBatch(gl, PATH_QUAD, 128);
	let scale = 1;
	const flush = () => batch.draw();
	const blank = gl.createTexture();
	gl.bindTexture(gl.TEXTURE_2D, blank);
	gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, new Uint8Array([255, 255, 255, 255]));
	gl.bindTexture(gl.TEXTURE_2D, null);
	function segment(
		x: number, y: number, dx: number, dy: number,
		width: number, color: Rgb, alpha: number,
	): void {
		const half = width / 2 / Math.hypot(dx, dy);
		const nx = -dy * half;
		const ny = dx * half;
		push(batch, x - nx, y - ny, dx, dy, nx * 2, ny * 2, 0, color, alpha, 0, 0, 0, 0);
	}
	function run(
		text: string, left: number, top: number, height: number,
		color: Rgb, alpha: number,
	): void {
		if (!atlas) {
			return;
		}
		let pen = left;
		for (const char of text) {
			const glyph = atlas.get(char);
			if (!glyph) {
				continue;
			}
			if (glyph.width > 0) {
				push(batch, pen, top, glyph.width * height, 0, 0, height, 1, color, alpha, glyph.u0, glyph.v0, glyph.u1, glyph.v1);
			}
			pen += glyph.advance * height;
		}
	}
	return {
		begin(worldPerPixel) {
			batch.begin();
			scale = worldPerPixel;
		},
		cell(x, y, size, type, color, alpha) {
			if (type === "square") {
				push(batch, x, y, size, 0, 0, size, 0, color, alpha, 0, 0, 0, 0);
				return;
			}
			const cx = x + size / 2;
			const cy = y + size / 2;
			const radius = size / Math.sqrt(3);
			const start = type === "hexFlat" ? 0 : -30;
			for (let i = 0; i < 6; i += 2) {
				const a = ((start + 60 * i) * Math.PI) / 180;
				const b = ((start + 60 * (i + 2)) * Math.PI) / 180;
				push(
					batch, cx, cy,
					radius * Math.cos(a), radius * Math.sin(a),
					radius * Math.cos(b), radius * Math.sin(b),
					0, color, alpha, 0, 0, 0, 0,
				);
			}
		},
		line(x0, y0, x1, y1, width, color, alpha) {
			const dx = x1 - x0;
			const dy = y1 - y0;
			const length = Math.hypot(dx, dy);
			if (length <= 0) {
				return;
			}
			const grow = HALO_PIXELS * scale;
			const ux = (dx / length) * grow;
			const uy = (dy / length) * grow;
			segment(
				x0 - ux, y0 - uy, dx + ux * 2, dy + uy * 2,
				width * scale + grow * 2, HALO_COLOR, alpha,
			);
			segment(x0, y0, dx, dy, width * scale, color, alpha);
		},
		label(text, x, y, color, alpha) {
			if (!atlas) {
				return;
			}
			const height = LABEL_PIXELS * scale;
			const width = atlas.measure(text) * height;
			const left = x - width / 2;
			const top = y - LABEL_LIFT * scale - height;
			const reach = HALO_PIXELS * scale;
			for (const [ox, oy] of HALO_RING) {
				run(text, left + ox * reach, top + oy * reach, height, HALO_COLOR, alpha);
			}
			run(text, left, top, height, color, alpha);
		},
		draw(frame) {
			if (batch.count === 0) {
				return;
			}
			batch.upload();
			program.use();
			gl.activeTexture(gl.TEXTURE0);
			gl.bindTexture(gl.TEXTURE_2D, atlas ? atlas.texture : blank);
			gl.uniform1i(program.at.u_atlas, 0);
			gl.uniformMatrix3fv(program.at.u_clip, false, frame.clip);
			blended(gl, flush);
		},
		dispose() {
			program.dispose();
			batch.dispose();
			gl.deleteTexture(blank);
		},
	};
}
