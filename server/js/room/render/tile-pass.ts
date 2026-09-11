import type { Camera } from "./camera.ts";
import type { Rect } from "../model/types.ts";
import type { TileRange } from "./tiles.ts";
import type { Attribute } from "../gl/quads.ts";
import type { Slot, TextureArray } from "../gl/texture-array.ts";
import type { MapRef } from "../protocol.ts";
import { blended } from "../gl/blend.ts";
import { clipMatrix, visibleRect } from "./camera.ts";
import { createProgram } from "../gl/program.ts";
import { createQuadBatch } from "../gl/quads.ts";
import { createTextureArray } from "../gl/texture-array.ts";
import { newLoader } from "../gl/loader.ts";
import {
	LAYERS,
	levelFor,
	newRange,
	rangeCount,
	tileKey,
	tileRect,
	tileURL,
	uvFor,
	visibleRange,
} from "./tiles.ts";
import { fragmentSource, uniforms, vertexSource } from "./shaders/tile.ts";
const TILE_QUAD: readonly Attribute[] = [{ size: 4 }, { size: 4 }, { size: 1 }];
export interface TilePass {
	begin(): void;
	end(): boolean;
	draw(cam: Camera, map: MapRef, alpha: number, deviceWidth: number, deviceHeight: number, dpr: number): void;
	fetched(): number;
	dispose(): void;
}
export function createTilePass(gl: WebGL2RenderingContext, invalidate: () => void): TilePass {
	const program = createProgram(gl, vertexSource, fragmentSource, uniforms);
	const stores = new Map<number, TextureArray>();
	const tileSizeByKey = new Map<string, number>();
	const loader = newLoader(invalidate);
	const batch = createQuadBatch(gl, TILE_QUAD, 256);
	const matrix = new Float32Array(9);
	const view: Rect = { x1: 0, y1: 0, x2: 0, y2: 0 };
	const rect: Rect = { x1: 0, y1: 0, x2: 0, y2: 0 };
	const uv: Rect = { x1: 0, y1: 0, x2: 0, y2: 0 };
	const range: TileRange = newRange();
	const flush = () => batch.draw();
	function storeFor(tileSize: number): TextureArray {
		const existing = stores.get(tileSize);
		if (existing) {
			return existing;
		}
		const store = createTextureArray(gl, tileSize, LAYERS);
		stores.set(tileSize, store);
		return store;
	}
	function push(slot: Slot): void {
		const at = batch.cursor();
		const data = batch.data;
		data[at] = rect.x1;
		data[at + 1] = rect.y1;
		data[at + 2] = rect.x2 - rect.x1;
		data[at + 3] = rect.y2 - rect.y1;
		data[at + 4] = uv.x1;
		data[at + 5] = uv.y1;
		data[at + 6] = uv.x2;
		data[at + 7] = uv.y2;
		data[at + 8] = slot.layer;
	}
	return {
		begin() {
			for (const store of stores.values()) {
				store.tick();
			}
			loader.begin();
		},
		draw(cam, map, alpha, deviceWidth, deviceHeight, dpr) {
			if (map.width < 1 || map.height < 1 || map.tileSize < 1) {
				return;
			}
			const store = storeFor(map.tileSize);
			const z = levelFor(cam.zoom, dpr, map.maxZoom);
			visibleRect(cam, { width: deviceWidth / dpr, height: deviceHeight / dpr }, view);
			visibleRange(map, z, view, range);
			const wanted = rangeCount(range);
			if (wanted === 0) {
				return;
			}
			batch.reserve(wanted);
			batch.begin();
			for (let y = range.y0; y <= range.y1; y++) {
				for (let x = range.x0; x <= range.x1; x++) {
					tileRect(map, z, x, y, rect);
					const key = tileKey(map, z, x, y);
					const slot = store.get(key);
					if (slot) {
						uvFor(map, z, x, y, rect, uv);
						push(slot);
						continue;
					}
					tileSizeByKey.set(key, map.tileSize);
					loader.want(
						key,
						tileURL(map, z, x, y),
						Math.hypot((rect.x1 + rect.x2) / 2 - cam.x, (rect.y1 + rect.y2) / 2 - cam.y),
					);
					for (let up = z + 1; up <= map.maxZoom; up++) {
						const shift = up - z;
						const ax = x >> shift;
						const ay = y >> shift;
						const ancestor = store.get(tileKey(map, up, ax, ay));
						if (ancestor) {
							uvFor(map, up, ax, ay, rect, uv);
							push(ancestor);
							break;
						}
					}
				}
			}
			if (batch.count === 0) {
				return;
			}
			batch.upload();
			program.use();
			gl.activeTexture(gl.TEXTURE0);
			gl.bindTexture(gl.TEXTURE_2D_ARRAY, store.texture);
			gl.uniform1i(program.at.u_tiles, 0);
			gl.uniform1f(program.at.u_alpha, alpha);
			gl.uniformMatrix3fv(program.at.u_clip, false, clipMatrix(cam, deviceWidth, deviceHeight, dpr, matrix));
			blended(gl, flush);
		},
		end() {
			loader.end();
			let uploaded = 0;
			const waiting = loader.drain((key, bitmap) => {
				uploaded++;
				const tileSize = tileSizeByKey.get(key);
				if (tileSize === undefined) {
					return;
				}
				tileSizeByKey.delete(key);
				const store = stores.get(tileSize);
				if (!store) {
					return;
				}
				store.upload(key, bitmap, bitmap.width, bitmap.height);
			});
			return uploaded > 0 || waiting > 0;
		},
		fetched: () => loader.fetched(),
		dispose() {
			loader.stop();
			program.dispose();
			batch.dispose();
			for (const store of stores.values()) {
				store.dispose();
			}
			stores.clear();
		},
	};
}
