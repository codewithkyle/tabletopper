// The map itself: one instanced draw call for every tile on screen, at every
// level, in one go.
//
// THE WHOLE VIEWPORT IS ONE DRAW CALL because the cache is a texture array. A
// draw call can only sample the textures bound to it, so ninety-six separate
// tile textures would be ninety-six calls; one array with ninety-six layers is
// one bind, one buffer upload, and one drawArraysInstanced whose per-instance
// attributes carry the rectangle, the texture coordinates and the layer index.
// Tiles at different levels ride in the same call for the same reason -- the
// level is not a property of the texture, only of the coordinates written into
// the instance.
//
// A TILE THAT IS NOT HERE YET IS DRAWN FROM ITS ANCESTOR. Walking up the
// pyramid until something is resident, and drawing the sub-rectangle of it that
// covers this tile, is what makes a map appear coarse and sharpen in place
// rather than filling in square by square out of an empty rectangle.

import type { Camera, Rect } from "./camera.ts";
import type { Loader, Slot, TileRange } from "./tiles.ts";
import type { MapRef } from "../protocol.ts";
import { clipMatrix, visibleRect } from "./camera.ts";
import { createProgram, uniforms } from "./gl.ts";
import {
	LAYERS,
	Slots,
	levelFor,
	mapPrefix,
	newLoader,
	newRange,
	rangeCount,
	tileKey,
	tileRect,
	tileURL,
	uvFor,
	visibleRange,
} from "./tiles.ts";

// FLOATS_PER_INSTANCE: the map-space rectangle, the texture rectangle, and the
// layer. Nine, packed as vec4 + vec4 + float so the attribute count stays at
// three and the whole instance is one contiguous run.
const FLOATS_PER_INSTANCE = 9;

const vertexSource = `#version 300 es
layout(location = 0) in vec2 a_corner;
layout(location = 1) in vec4 a_rect;
layout(location = 2) in vec4 a_uv;
layout(location = 3) in float a_layer;

uniform mat3 u_clip;

out vec2 v_uv;
flat out float v_layer;

void main() {
	vec2 world = a_rect.xy + a_corner * a_rect.zw;
	v_uv = mix(a_uv.xy, a_uv.zw, a_corner);
	v_layer = a_layer;
	gl_Position = vec4((u_clip * vec3(world, 1.0)).xy, 0.0, 1.0);
}
`;

const fragmentSource = `#version 300 es
precision highp float;
precision highp sampler2DArray;

in vec2 v_uv;
flat in float v_layer;

uniform sampler2DArray u_tiles;
uniform float u_alpha;

out vec4 outColor;

void main() {
	vec4 texel = texture(u_tiles, vec3(v_uv, v_layer));
	outColor = vec4(texel.rgb, texel.a * u_alpha);
}
`;

const names = ["u_clip", "u_tiles", "u_alpha"] as const;

export interface TilePass {
	// begin and end bracket one frame. Everything between them is demand: what
	// was asked for is fetched, and what was not is abandoned.
	begin(): void;
	end(): boolean;

	// draw paints one map at one opacity. It is called once normally and twice
	// during a crossfade, oldest first.
	draw(cam: Camera, map: MapRef, alpha: number, deviceWidth: number, deviceHeight: number, dpr: number): void;

	// abandon drops the fetch queue for a map that has been replaced or
	// re-tiled: those URLs are gone and nothing will answer them.
	abandon(map: MapRef): void;

	fetched(): number;
	dispose(): void;
}

// A store is one texture array, and there is one per distinct tile size.
//
// assets.tile_size IS A PER-ROW COLUMN, stored so a map tiled under an older
// constant goes on working. A texture array has one fixed layer size, so two
// maps tiled differently cannot share one -- and in practice there is exactly
// one of these.
interface Store {
	texture: WebGLTexture;
	slots: Slots;
	tileSize: number;
}

export function createTilePass(gl: WebGL2RenderingContext, invalidate: () => void): TilePass {
	const program = createProgram(gl, vertexSource, fragmentSource);
	const at = uniforms(gl, program, names);

	// MAX_ARRAY_TEXTURE_LAYERS is 256 on every desktop and as low as 256 on
	// mobile, so the cache size is what limits this rather than the driver --
	// but a driver that said less would be believed rather than crashed into.
	const maxLayers = Math.min(LAYERS, gl.getParameter(gl.MAX_ARRAY_TEXTURE_LAYERS) as number);

	const stores = new Map<number, Store>();
	const tileSizeByKey = new Map<string, number>();
	const loader = newLoader(invalidate);

	const vao = gl.createVertexArray();
	gl.bindVertexArray(vao);

	const corners = gl.createBuffer();
	gl.bindBuffer(gl.ARRAY_BUFFER, corners);
	gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([0, 0, 1, 0, 0, 1, 1, 1]), gl.STATIC_DRAW);
	gl.enableVertexAttribArray(0);
	gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);

	const instances = gl.createBuffer();
	gl.bindBuffer(gl.ARRAY_BUFFER, instances);
	const stride = FLOATS_PER_INSTANCE * 4;
	gl.enableVertexAttribArray(1);
	gl.vertexAttribPointer(1, 4, gl.FLOAT, false, stride, 0);
	gl.vertexAttribDivisor(1, 1);
	gl.enableVertexAttribArray(2);
	gl.vertexAttribPointer(2, 4, gl.FLOAT, false, stride, 16);
	gl.vertexAttribDivisor(2, 1);
	gl.enableVertexAttribArray(3);
	gl.vertexAttribPointer(3, 1, gl.FLOAT, false, stride, 32);
	gl.vertexAttribDivisor(3, 1);

	gl.bindVertexArray(null);
	gl.bindBuffer(gl.ARRAY_BUFFER, null);

	// The instance array is grown by doubling and never shrunk, which is the
	// second performance rule: a typed array reallocated per frame is a garbage
	// collection pause per second, and pauses are the jank that gets worse the
	// longer a session runs.
	let data = new Float32Array(256 * FLOATS_PER_INSTANCE);
	let count = 0;

	const matrix = new Float32Array(9);
	const view: Rect = { x1: 0, y1: 0, x2: 0, y2: 0 };
	const rect: Rect = { x1: 0, y1: 0, x2: 0, y2: 0 };
	const uv: Rect = { x1: 0, y1: 0, x2: 0, y2: 0 };
	const range: TileRange = newRange();

	function storeFor(tileSize: number): Store {
		const existing = stores.get(tileSize);
		if (existing) {
			return existing;
		}

		const texture = gl.createTexture();
		gl.bindTexture(gl.TEXTURE_2D_ARRAY, texture);
		gl.texStorage3D(gl.TEXTURE_2D_ARRAY, 1, gl.RGBA8, tileSize, tileSize, maxLayers);

		// LINEAR WITH NO MIPMAPS, because the pyramid IS the mip chain and it
		// is a better one: generateMipmap would build a second, redundant chain
		// per tile, and a minified tile would sample across a tile boundary
		// that the level below does not have.
		gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
		gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
		gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
		gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
		gl.bindTexture(gl.TEXTURE_2D_ARRAY, null);

		const store: Store = { texture, slots: new Slots(maxLayers), tileSize };
		stores.set(tileSize, store);

		return store;
	}

	function reserve(needed: number): void {
		const floats = needed * FLOATS_PER_INSTANCE;
		if (floats <= data.length) {
			return;
		}

		let size = data.length;
		while (size < floats) {
			size *= 2;
		}
		data = new Float32Array(size);
	}

	function push(slot: Slot): void {
		const at = count * FLOATS_PER_INSTANCE;

		data[at] = rect.x1;
		data[at + 1] = rect.y1;
		data[at + 2] = rect.x2 - rect.x1;
		data[at + 3] = rect.y2 - rect.y1;
		data[at + 4] = uv.x1;
		data[at + 5] = uv.y1;
		data[at + 6] = uv.x2;
		data[at + 7] = uv.y2;
		data[at + 8] = slot.layer;

		count++;
	}

	return {
		begin() {
			for (const store of stores.values()) {
				store.slots.tick();
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

			reserve(wanted);
			count = 0;

			for (let y = range.y0; y <= range.y1; y++) {
				for (let x = range.x0; x <= range.x1; x++) {
					tileRect(map, z, x, y, rect);

					const key = tileKey(map, z, x, y);
					const slot = store.slots.get(key);

					if (slot) {
						uvFor(map, z, x, y, rect, uv);
						push(slot);

						continue;
					}

					// PRIORITY IS DISTANCE FROM THE MIDDLE OF THE SCREEN.
					// Somebody who has just panned is looking at the centre,
					// and filling in from the edges delivers the same tiles in
					// the least useful order.
					tileSizeByKey.set(key, map.tileSize);
					loader.want(
						key,
						tileURL(map, z, x, y),
						Math.hypot((rect.x1 + rect.x2) / 2 - cam.x, (rect.y1 + rect.y2) / 2 - cam.y),
					);

					// Walk up until something is resident. The parent of
					// (z, x, y) is (z+1, x>>1, y>>1) and covers twice the
					// ground at half the detail; drawing the part of it that
					// covers this tile is what a coarse map is made of.
					for (let up = z + 1; up <= map.maxZoom; up++) {
						const shift = up - z;
						const ax = x >> shift;
						const ay = y >> shift;

						const ancestor = store.slots.get(tileKey(map, up, ax, ay));
						if (ancestor) {
							uvFor(map, up, ax, ay, rect, uv);
							push(ancestor);

							break;
						}
					}
				}
			}

			if (count === 0) {
				return;
			}

			gl.useProgram(program);
			gl.bindVertexArray(vao);
			gl.bindBuffer(gl.ARRAY_BUFFER, instances);
			gl.bufferData(gl.ARRAY_BUFFER, data.subarray(0, count * FLOATS_PER_INSTANCE), gl.DYNAMIC_DRAW);

			gl.activeTexture(gl.TEXTURE0);
			gl.bindTexture(gl.TEXTURE_2D_ARRAY, store.texture);
			gl.uniform1i(at.u_tiles, 0);
			gl.uniform1f(at.u_alpha, alpha);
			gl.uniformMatrix3fv(at.u_clip, false, clipMatrix(cam, deviceWidth, deviceHeight, dpr, matrix));

			// Blending is on for the crossfade's sake and costs nothing when
			// alpha is 1. A map with transparent corners -- a hand-drawn island
			// on a white page saved as PNG -- composites over the table for
			// free, which is the right answer anyway.
			gl.enable(gl.BLEND);
			gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
			gl.drawArraysInstanced(gl.TRIANGLE_STRIP, 0, 4, count);
			gl.disable(gl.BLEND);

			gl.bindVertexArray(null);
			gl.bindBuffer(gl.ARRAY_BUFFER, null);
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

				const slot = store.slots.claim(key, bitmap.width, bitmap.height);
				if (!slot) {
					// Every layer is spoken for by this frame, which means the
					// viewport needs more tiles than the cache holds. The tile
					// is dropped rather than evicting one that is on screen
					// right now; it will be asked for again next frame.
					return;
				}

				gl.bindTexture(gl.TEXTURE_2D_ARRAY, store.texture);
				gl.texSubImage3D(
					gl.TEXTURE_2D_ARRAY, 0,
					0, 0, slot.layer,
					bitmap.width, bitmap.height, 1,
					gl.RGBA, gl.UNSIGNED_BYTE, bitmap,
				);
				gl.bindTexture(gl.TEXTURE_2D_ARRAY, null);
			});

			// ANYTHING UPLOADED MEANS ANOTHER FRAME, not just anything still
			// queued. A tile lands in the texture array AFTER this frame's
			// draw call has already gone out, so the frame that uploads it is
			// never the frame that shows it -- and a loop that stopped here
			// would leave the map one batch of tiles short until something
			// else happened to ask for a frame. Waiting matters too: those are
			// uploads held back to keep this frame inside its budget.
			return uploaded > 0 || waiting > 0;
		},

		abandon(map) {
			loader.abandon(mapPrefix(map));
		},

		fetched: () => loader.fetched(),

		dispose() {
			loader.stop();
			gl.deleteProgram(program);
			gl.deleteVertexArray(vao);
			gl.deleteBuffer(corners);
			gl.deleteBuffer(instances);
			for (const store of stores.values()) {
				gl.deleteTexture(store.texture);
			}
			stores.clear();
		},
	};
}
