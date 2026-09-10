// The drawing on the table: every stroke on the viewed floor, as instanced
// segments with round caps computed in the fragment shader.
//
// A SEGMENT IS AN INSTANCE AND A STROKE IS A RUN OF THEM. The quad is grown by
// the line's own half-width at both ends and both sides, and the fragment
// shader keeps the pixels within that distance of the segment itself -- so a
// cap is round for free, and two segments that meet at an angle need no join
// geometry at all because each one's cap fills the corner. That is the whole
// reason this is not a triangle strip: a strip needs mitres, and a mitre on a
// hand-drawn line that doubles back on itself is a spike.
//
// THERE ARE TWO BUFFERS AND THE DIFFERENCE IS HOW OFTEN THEY CHANGE. A finished
// stroke never changes again, so the buffer holding all of them is rebuilt only
// when the floor's set of finished strokes does -- an end, an erase, a clear, a
// change of floor. A stroke still being drawn changes every frame, and there is
// at most one per person at the table, so that buffer is rebuilt per frame and
// is almost always empty. Rebuilding the first per frame would mean uploading
// an evening's drawing sixty times a second to add one point to it.
//
// THE WIDTH IS IN MAP PIXELS AND THE FLOOR IS IN DEVICE PIXELS. Ink is on the
// map: a line drawn across a corridor stays across that corridor at every zoom,
// which a screen-space width would not do. But a two-pixel line on a map zoomed
// right out is a sub-pixel line that flickers in and out of existence as the
// camera moves, so the shader clamps the half-width against a uniform the
// caller computes from the zoom. It is a uniform and not a field for exactly
// that reason: it changes with the camera, and the buffer must not.

import type { Camera } from "./camera.ts";
import type { Stroke } from "../protocol.ts";
import { clipMatrix } from "./camera.ts";
import { createProgram, uniforms } from "./gl.ts";
import { parseColor } from "./grid-pass.ts";
import { strokeSegments } from "../draw.ts";

// FLOATS_PER_INSTANCE: the segment's two endpoints, the colour, and the
// half-width. Two vec4s and a float, which is nine rather than the twelve a
// third vec4 would pad it to -- and at a few hundred thousand segments that
// quarter is megabytes.
const FLOATS_PER_INSTANCE = 9;

// MIN_HALF_DEVICE is the thinnest a line is allowed to be drawn, in device
// pixels. Three quarters of a pixel still lands as a visible grey line through
// the antialiasing; below that a stroke starts to disappear in patches as the
// camera moves, which reads as the drawing being corrupted rather than small.
const MIN_HALF_DEVICE = 0.75;

const vertexSource = `#version 300 es
layout(location = 0) in vec2 a_corner;
layout(location = 1) in vec4 a_seg;
layout(location = 2) in vec4 a_color;
layout(location = 3) in float a_half;

uniform mat3 u_clip;
uniform float u_minHalf;

out vec2 v_local;
flat out vec4 v_color;
flat out float v_half;
flat out float v_len;

void main() {
	float radius = max(a_half, u_minHalf);

	vec2 p0 = a_seg.xy;
	vec2 delta = a_seg.zw - p0;
	float len = length(delta);

	// A SEGMENT OF NO LENGTH IS A DOT, which is what a click with a pen is and
	// what a one-point stroke expands to. The direction is arbitrary then, and
	// the fragment shader's clamp collapses the whole quad onto the one point.
	vec2 dir = len > 1e-4 ? delta / len : vec2(1.0, 0.0);
	vec2 nor = vec2(-dir.y, dir.x);

	// Local space runs along the segment: x from -radius to len + radius so
	// there is room for both caps, y across it.
	float lx = mix(-radius, len + radius, a_corner.x);
	float ly = mix(-radius, radius, a_corner.y);

	v_local = vec2(lx, ly);
	v_color = a_color;
	v_half = radius;
	v_len = len;

	vec2 world = p0 + dir * lx + nor * ly;

	gl_Position = vec4((u_clip * vec3(world, 1.0)).xy, 0.0, 1.0);
}
`;

const fragmentSource = `#version 300 es
precision highp float;

in vec2 v_local;
flat in vec4 v_color;
flat in float v_half;
flat in float v_len;

out vec4 outColor;

void main() {
	// The distance from this pixel to the segment, in map pixels: clamp along
	// the line and measure to where that lands. Inside the run it is the
	// perpendicular distance and past either end it is the distance to the
	// endpoint, which is what makes the cap a circle rather than a square.
	float t = clamp(v_local.x, 0.0, v_len);
	float dist = length(v_local - vec2(t, 0.0));

	// fwidth answers in map pixels per device pixel here, because v_local is in
	// map pixels -- so the edge is one device pixel wide at every zoom without
	// the zoom being passed in.
	float aa = max(fwidth(dist), 1e-5);
	float alpha = v_color.a * (1.0 - smoothstep(v_half - aa, v_half + aa, dist));

	if (alpha <= 0.0) {
		discard;
	}

	outColor = vec4(v_color.rgb, alpha);
}
`;

const names = ["u_clip", "u_minHalf"] as const;

export interface StrokePass {
	// sync brings the finished buffer up to date with the floor. It walks the
	// strokes to decide whether anything changed and returns without touching
	// the GPU when nothing has, which is every frame but the few that follow an
	// event.
	sync(strokes: readonly Stroke[], layerID: string): void;

	// live rebuilds the small buffer of strokes still being drawn: everybody
	// else's from the store, and this viewer's own from `own` rather than from
	// the store's copy of it.
	//
	// THE VIEWER'S OWN ECHO IS SKIPPED, not merged. stroke.began and
	// stroke.extended come back to their author like everybody else's and the
	// reducer applies them, so the store holds the same line a round trip
	// behind the hand. Drawing `own` instead is what keeps the ink under the pen.
	live(strokes: readonly Stroke[], layerID: string, own: Stroke | null): void;

	draw(cam: Camera, deviceWidth: number, deviceHeight: number, dpr: number): void;
	dispose(): void;
}

// A batch is one buffer's worth: the array being filled, how many instances are
// in it, and the VAO that draws it.
interface Batch {
	vao: WebGLVertexArrayObject;
	buffer: WebGLBuffer;
	data: Float32Array;
	count: number;
}

export function createStrokePass(gl: WebGL2RenderingContext): StrokePass {
	const program = createProgram(gl, vertexSource, fragmentSource);
	const at = uniforms(gl, program, names);

	// ONE CORNER BUFFER FOR BOTH BATCHES. The quad is the same four vertices
	// whatever is being drawn with it; only the instances differ.
	const corners = gl.createBuffer();
	gl.bindBuffer(gl.ARRAY_BUFFER, corners);
	gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([0, 0, 1, 0, 0, 1, 1, 1]), gl.STATIC_DRAW);
	gl.bindBuffer(gl.ARRAY_BUFFER, null);

	const done = newBatch(gl, corners, 1024);
	const drawing = newBatch(gl, corners, 64);

	// key is what the finished buffer was built from: the floor, how many
	// finished strokes are on it, and the newest one's id. Ids are minted in
	// order and the store keeps the collection sorted by them, so that triple
	// moves for an end, an erase and a clear alike -- and for nothing else.
	let key = "";

	const color = new Float32Array(4);
	const segments: number[] = [];
	const matrix = new Float32Array(9);

	// build fills one batch from a list of strokes. It is the only place a
	// stroke becomes geometry, which is why the tessellation of a shape lives in
	// draw.ts rather than in here: the same expansion serves the eraser's hit
	// test, and a right answer computed twice is a right answer until it is not.
	function build(batch: Batch, strokes: readonly Stroke[]): void {
		batch.count = 0;

		for (const stroke of strokes) {
			segments.length = 0;
			strokeSegments(stroke, segments);
			if (segments.length === 0) {
				continue;
			}

			parseColor(stroke.color, color);
			const half = Math.max(stroke.width, 1) / 2;

			for (let i = 0; i + 3 < segments.length; i += 4) {
				push(batch, segments[i], segments[i + 1], segments[i + 2], segments[i + 3], half);
			}
		}
	}

	// push writes one instance, growing the array by doubling when it is full.
	// The colour comes off the scratch above rather than through the arguments,
	// because it is the same for every segment of a stroke.
	function push(batch: Batch, x0: number, y0: number, x1: number, y1: number, half: number): void {
		const floats = (batch.count + 1) * FLOATS_PER_INSTANCE;
		if (floats > batch.data.length) {
			let size = batch.data.length;
			while (size < floats) {
				size *= 2;
			}

			const grown = new Float32Array(size);
			grown.set(batch.data);
			batch.data = grown;
		}

		const i = batch.count * FLOATS_PER_INSTANCE;

		batch.data[i] = x0;
		batch.data[i + 1] = y0;
		batch.data[i + 2] = x1;
		batch.data[i + 3] = y1;

		batch.data[i + 4] = color[0];
		batch.data[i + 5] = color[1];
		batch.data[i + 6] = color[2];
		batch.data[i + 7] = color[3];

		batch.data[i + 8] = half;

		batch.count++;
	}

	function upload(batch: Batch): void {
		gl.bindBuffer(gl.ARRAY_BUFFER, batch.buffer);
		gl.bufferData(gl.ARRAY_BUFFER, batch.data.subarray(0, batch.count * FLOATS_PER_INSTANCE), gl.DYNAMIC_DRAW);
		gl.bindBuffer(gl.ARRAY_BUFFER, null);
	}

	function run(batch: Batch): void {
		if (batch.count === 0) {
			return;
		}

		gl.bindVertexArray(batch.vao);
		gl.drawArraysInstanced(gl.TRIANGLE_STRIP, 0, 4, batch.count);
	}

	// The two lists the batches are built from, reused so the frame path does
	// not allocate.
	const finished: Stroke[] = [];
	const live: Stroke[] = [];

	return {
		sync(strokes, layerID) {
			finished.length = 0;

			let newest = "";
			for (const stroke of strokes) {
				if (stroke.layerId !== layerID || !stroke.done) {
					continue;
				}

				finished.push(stroke);
				newest = stroke.id;
			}

			const next = layerID + "|" + finished.length + "|" + newest;
			if (next === key) {
				return;
			}
			key = next;

			build(done, finished);
			upload(done);
		},

		live(strokes, layerID, own) {
			live.length = 0;

			for (const stroke of strokes) {
				if (stroke.layerId !== layerID || stroke.done || stroke.id === own?.id) {
					continue;
				}

				live.push(stroke);
			}

			if (own && own.layerId === layerID) {
				live.push(own);
			}

			// AN EMPTY BATCH IS NOT UPLOADED, which is the ordinary case: it
			// only has anything in it while somebody at the table has a button
			// down.
			if (live.length === 0) {
				drawing.count = 0;

				return;
			}

			build(drawing, live);
			upload(drawing);
		},

		draw(cam, deviceWidth, deviceHeight, dpr) {
			if (done.count === 0 && drawing.count === 0) {
				return;
			}

			gl.useProgram(program);
			gl.uniformMatrix3fv(at.u_clip, false, clipMatrix(cam, deviceWidth, deviceHeight, dpr, matrix));
			gl.uniform1f(at.u_minHalf, MIN_HALF_DEVICE / Math.max(cam.zoom * dpr, 1e-4));

			// TWO SEGMENTS THAT OVERLAP BLEND TWICE, which is invisible at full
			// alpha and is why the colour picker has none: a translucent stroke
			// would show a darker dot at every joint. The protocol accepts
			// #RRGGBBAA because the grid needs it, and a stroke that arrives
			// with one draws honestly rather than being flattened here.
			gl.enable(gl.BLEND);
			gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);

			run(done);
			run(drawing);

			gl.disable(gl.BLEND);
			gl.bindVertexArray(null);
		},

		dispose() {
			gl.deleteProgram(program);
			gl.deleteVertexArray(done.vao);
			gl.deleteVertexArray(drawing.vao);
			gl.deleteBuffer(done.buffer);
			gl.deleteBuffer(drawing.buffer);
			gl.deleteBuffer(corners);
		},
	};
}

// newBatch wires one VAO: the shared corner quad on location 0, and the
// instance attributes on 1 to 3 with a divisor.
function newBatch(gl: WebGL2RenderingContext, corners: WebGLBuffer, instances: number): Batch {
	const vao = gl.createVertexArray();
	gl.bindVertexArray(vao);

	gl.bindBuffer(gl.ARRAY_BUFFER, corners);
	gl.enableVertexAttribArray(0);
	gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);

	const buffer = gl.createBuffer();
	gl.bindBuffer(gl.ARRAY_BUFFER, buffer);

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

	return { vao, buffer, data: new Float32Array(instances * FLOATS_PER_INSTANCE), count: 0 };
}
