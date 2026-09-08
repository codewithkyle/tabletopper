// Outlines: the condition rings round a creature, the ring or rectangle round a
// selected pawn, and the outline a drag ghost is drawn as.
//
// THEY ARE ONE PASS BECAUSE THEY ARE ONE SHAPE. Every one of them is a hollow
// ellipse or a hollow rectangle at a position, in a colour, with a thickness --
// so they are one instanced draw over one buffer, in the order they were added,
// and a fight with sixteen conditions on eight goblins is still one call.
//
// THE THICKNESS IS IN DEVICE PIXELS AND THE RADIUS IS IN MAP PIXELS. A ring
// whose line scaled with the camera would be a hairline zoomed out and a band
// zoomed in; a ring whose RADIUS did not scale would come away from the pawn it
// belongs to. So the instance carries a world rectangle and the shader converts
// the pixel thickness against the current zoom.

import type { Camera } from "./camera.ts";
import { clipMatrix } from "./camera.ts";
import { createProgram, uniforms } from "./gl.ts";

// FLOATS_PER_INSTANCE: the rectangle, the colour, and the style. Three vec4s.
const FLOATS_PER_INSTANCE = 12;

export const RING_ELLIPSE = 0;
export const RING_RECT = 1;

const vertexSource = `#version 300 es
layout(location = 0) in vec2 a_corner;
layout(location = 1) in vec4 a_rect;
layout(location = 2) in vec4 a_color;
layout(location = 3) in vec4 a_style;

uniform mat3 u_clip;
uniform float u_scale;

out vec2 v_local;
flat out vec4 v_color;
flat out vec2 v_half;
flat out vec2 v_style;

void main() {
	v_local = a_corner * 2.0 - 1.0;
	v_color = a_color;
	v_half = a_rect.zw;

	// The thickness arrives in device pixels and leaves in map pixels, which is
	// the one conversion this pass exists to get right.
	v_style = vec2(a_style.x / max(u_scale, 1e-4), a_style.y);

	// The quad is grown by the line's own width so a ring drawn exactly at the
	// pawn's radius has its outer half somewhere to be rasterised. Without it
	// the outer edge is clipped by the quad and every ring reads as thinner
	// than the one before it.
	vec2 grown = a_rect.zw + v_style.x;
	vec2 world = a_rect.xy + v_local * grown;

	gl_Position = vec4((u_clip * vec3(world, 1.0)).xy, 0.0, 1.0);
	v_local *= grown / max(a_rect.zw, vec2(1e-4));
}
`;

const fragmentSource = `#version 300 es
precision highp float;

in vec2 v_local;
flat in vec4 v_color;
flat in vec2 v_half;
flat in vec2 v_style;

out vec4 outColor;

void main() {
	float thickness = v_style.x;

	// inside is how far this fragment is INSIDE the shape's edge, in map
	// pixels. Negative is outside it.
	float inside;
	if (v_style.y < 0.5) {
		// An ellipse's edge distance is only exact for a circle, which every
		// ring in this app is -- a creature's quad is square. The approximation
		// is measured along the radius, which for a circle IS the distance.
		float r = length(v_local);
		inside = (1.0 - r) * v_half.x;
	} else {
		vec2 edge = (vec2(1.0) - abs(v_local)) * v_half;
		inside = min(edge.x, edge.y);
	}

	// The line straddles the edge, half in and half out, so a ring at a pawn's
	// radius touches the pawn rather than sitting a line's width inside it.
	float aa = max(fwidth(inside), 1e-5);
	float alpha = smoothstep(-thickness * 0.5 - aa, -thickness * 0.5, inside)
		* (1.0 - smoothstep(thickness * 0.5, thickness * 0.5 + aa, inside));

	if (alpha <= 0.0) {
		discard;
	}

	outColor = vec4(v_color.rgb, v_color.a * alpha);
}
`;

const names = ["u_clip", "u_scale"] as const;

export interface RingPass {
	// begin empties the buffer. Rings are per frame rather than cached, because
	// what is in them -- the selection, the ghosts, a condition somebody just
	// added -- changes at the rate a hand moves.
	begin(): void;

	// add is one outline: a centre, half extents in map pixels, a colour, a
	// thickness in device pixels, and which shape.
	add(
		x: number, y: number, halfW: number, halfH: number,
		color: readonly [number, number, number], alpha: number,
		thickness: number, shape: number,
	): void;

	draw(cam: Camera, deviceWidth: number, deviceHeight: number, dpr: number): void;
	dispose(): void;
}

export function createRingPass(gl: WebGL2RenderingContext): RingPass {
	const program = createProgram(gl, vertexSource, fragmentSource);
	const at = uniforms(gl, program, names);

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
	for (let i = 0; i < 3; i++) {
		gl.enableVertexAttribArray(1 + i);
		gl.vertexAttribPointer(1 + i, 4, gl.FLOAT, false, stride, i * 16);
		gl.vertexAttribDivisor(1 + i, 1);
	}

	gl.bindVertexArray(null);
	gl.bindBuffer(gl.ARRAY_BUFFER, null);

	let data = new Float32Array(64 * FLOATS_PER_INSTANCE);
	let count = 0;

	const matrix = new Float32Array(9);

	return {
		begin() {
			count = 0;
		},

		add(x, y, halfW, halfH, color, alpha, thickness, shape) {
			const floats = (count + 1) * FLOATS_PER_INSTANCE;
			if (floats > data.length) {
				let size = data.length;
				while (size < floats) {
					size *= 2;
				}

				const grown = new Float32Array(size);
				grown.set(data);
				data = grown;
			}

			const at = count * FLOATS_PER_INSTANCE;

			data[at] = x;
			data[at + 1] = y;
			data[at + 2] = halfW;
			data[at + 3] = halfH;

			data[at + 4] = color[0];
			data[at + 5] = color[1];
			data[at + 6] = color[2];
			data[at + 7] = alpha;

			data[at + 8] = thickness;
			data[at + 9] = shape;
			data[at + 10] = 0;
			data[at + 11] = 0;

			count++;
		},

		draw(cam, deviceWidth, deviceHeight, dpr) {
			if (count === 0) {
				return;
			}

			gl.useProgram(program);
			gl.bindVertexArray(vao);
			gl.bindBuffer(gl.ARRAY_BUFFER, instances);
			gl.bufferData(gl.ARRAY_BUFFER, data.subarray(0, count * FLOATS_PER_INSTANCE), gl.DYNAMIC_DRAW);

			gl.uniform1f(at.u_scale, cam.zoom * dpr);
			gl.uniformMatrix3fv(at.u_clip, false, clipMatrix(cam, deviceWidth, deviceHeight, dpr, matrix));

			gl.enable(gl.BLEND);
			gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
			gl.drawArraysInstanced(gl.TRIANGLE_STRIP, 0, 4, count);
			gl.disable(gl.BLEND);

			gl.bindVertexArray(null);
			gl.bindBuffer(gl.ARRAY_BUFFER, null);
		},

		dispose() {
			gl.deleteProgram(program);
			gl.deleteVertexArray(vao);
			gl.deleteBuffer(corners);
			gl.deleteBuffer(instances);
		},
	};
}
