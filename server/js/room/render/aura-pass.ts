// The mark on whoever is acting: a warm ring round the pawn with two comet
// tails sweeping round it, for as long as the turn lasts.
//
// IT IS DaisyUI'S `aura`, REBUILT IN A SHADER, because a pawn is not an element
// and a component cannot be put on one. Everything below is that component's
// own CSS, read off the vendored bundle and turned into arithmetic; the
// constants say which of its numbers they are, and where they deliberately are
// not.
//
// WHAT THE COMPONENT ACTUALLY IS, in four parts:
//
//   .aura            a box padded by --aura-padding, filled with a conic
//                    gradient that rotates once every six seconds, with the
//                    content sitting on top of it -- so what shows is the
//                    padding.
//   .aura-dual       that gradient, repeated twice round the circle: empty for
//                    most of each half turn and then a linear ramp up to full
//                    colour, cut off dead at the end. The ramp is the TAIL and
//                    the cut is the head. See AURA_TAIL.
//   .aura:before     the same gradient again, blurred by 0.25rem at 70%.
//   .aura:after      and again, blurred by 1rem at 30%.
//
// THE STRIP WEARS THE SAME COMPONENT AND NOT THE SAME VARIANT. Over the table,
// the acting line of the turn order is `aura aura-silver` -- a fixed metallic
// band all the way round, carrying its own greys. This is `aura-dual`, in gold,
// with a longer tail. Same mechanic, same six seconds, same question answered;
// they are allowed to look different because nobody ever sees them at one
// scale. One is a 72-pixel card at the top of the screen and the other is a
// creature standing on the floor.
//
// The three copies are composited one over another in the same colour, which is
// the whole reason this fits in one pass: source-over of three layers that
// share an RGB is one alpha, 1 - (1-a)(1-b)(1-c), and the fragment shader can
// work it out rather than the framebuffer.
//
// THE PAWN'S OWN CIRCLE IS A HOLE AND NOT A FILL. In the DOM the gradient runs
// under the content and the content hides it; here the pass is drawn before the
// pawns, so the disc would be covered anyway -- except that an object's picture
// has transparent corners and would show the fill through them. So the inside is
// masked out, which is what is visible in the DOM version in either case.
//
// THE BLUR IS RADIAL AND ANGULAR AND IT IS AN APPROXIMATION. A real Gaussian is
// a second pass over a second framebuffer for an effect that is at most nine
// creatures wide. What a blurred ring actually looks like is an edge softened
// across the blur's radius and a gradient smeared along the arc by the same
// distance, and both of those are one line each: the edge is a smoothstep and
// the smear is a five-tap average of the gradient at that fragment's own
// radius. Nobody looking at a token can tell them apart.
//
// EVERY SIZE IS IN DEVICE PIXELS, which is the rule the pawn's border and the
// condition rings already follow: an aura that scaled with the camera would be
// a hairline at the zoom a GM runs a fight from and a band of paint zoomed in.

import type { Camera } from "./camera.ts";
import type { HPBand } from "../protocol.ts";
import { BLOOD_FRESH } from "./wounds.ts";
import { clipMatrix } from "./camera.ts";
import { createProgram, uniforms } from "./gl.ts";
import { radians } from "./path.ts";

// FLOATS_PER_INSTANCE: the rectangle, the colour, the style, and the angle's
// cosine and sine, exactly as the ring pass carries them. Three vec4s and a
// vec2.
const FLOATS_PER_INSTANCE = 14;

// AURA_DISC and AURA_RECT are what the hole in the middle is shaped like, and
// they are the pawn's own two shapes: a creature is a disc and an object is a
// turned rectangle. See pawn-pass.ts, which draws the thing this frames.
export const AURA_DISC = 0;
export const AURA_RECT = 1;

// AURA_PERIOD is how long one revolution takes. It is DaisyUI's default
// --tw-duration for the component, unchanged, so the ring round a pawn and the
// ring round its line in the strip turn at the same rate.
const AURA_PERIOD = 6000;

// AURA_PAD is the width of the crisp band, in device pixels. It is twice the
// component's --aura-padding, whose default 0.125rem was tried first and is too
// thin to find on a token at the zoom a fight is run from: what reads as a fine
// metal edge round a card at arm's length is a hairline round a goblin, and out
// here there is no card behind it to be an edge OF.
//
// DEVICE PIXELS AND NOT CSS ONES, unlike the strip's, because this band sits
// immediately outside the pawn's own two-device-pixel border. Two lines that
// touch have to be measured in one unit or they come apart the moment somebody
// opens the table on a retina screen.
const AURA_PAD = 4;

// AURA_NEAR and AURA_FAR are the two blurs, in device pixels, and AURA_NEAR_A
// and AURA_FAR_A are what each is worth. 0.25rem at 70 per cent and 1rem at 30
// per cent: the component's :before and :after.
const AURA_NEAR = 4;
const AURA_FAR = 16;
const AURA_NEAR_A = 0.7;
const AURA_FAR_A = 0.3;

// AURA_REACH is how far outside the pawn the quad has to extend for the widest
// of those to be rasterised, plus a pixel of slack for the antialiasing at the
// end of it. A quad cut to the exact reach clips the last of the far glow into
// a straight edge, which is the one artefact that would give the whole thing
// away as a rectangle.
const AURA_REACH = AURA_PAD + AURA_FAR + 2;

// AURA_GOLD is the colour of a turn, and it is the one number here that is not
// DaisyUI's -- the component takes currentColor and this is the currentColor
// chosen for it. Warm and light rather than yellow: a gold that a red map, a
// green field and a grey dungeon floor all sit under.
//
// IT IS THIS FILE'S ALONE. The strip's aura is aura-silver, which carries its
// own greys and takes nothing from outside, so there is no second copy of this
// number in a stylesheet to keep in step with.
export const AURA_GOLD: readonly [number, number, number] = [1, 0.78, 0.35];

// AURA_TAIL is how much of each half turn the tail takes: the arc from where
// it starts to glow to the head where it is cut off.
//
// IT IS LONGER THAN THE COMPONENT'S, which is the one place this departs from
// aura-dual on purpose. DaisyUI ramps over the last fifth of its period -- 36
// degrees of arc -- which is a bright dash on a card held at reading distance
// and a tick mark on a token across the table. Three fifths is a comet: a head,
// a trail long enough behind it to read as motion rather than as a mark, and
// still a dark gap between the two of them, which is the thing that stops it
// becoming a plain ring with a bright spot on it.
const AURA_TAIL = 0.6;

const vertexSource = `#version 300 es
#define REACH ${AURA_REACH}.0

layout(location = 0) in vec2 a_corner;
layout(location = 1) in vec4 a_rect;
layout(location = 2) in vec4 a_color;
layout(location = 3) in vec4 a_style;
layout(location = 4) in vec2 a_spin;

uniform mat3 u_clip;
uniform float u_scale;

out vec2 v_local;
flat out vec4 v_color;
flat out vec2 v_half;
flat out vec2 v_style;

void main() {
	vec2 corner = a_corner * 2.0 - 1.0;

	v_color = a_color;
	v_half = a_rect.zw;

	// The reach arrives as device pixels and leaves as map pixels, which is the
	// one conversion this pass exists to get right -- see the note at the top.
	float reach = REACH / max(u_scale, 1e-4);
	v_style = vec2(reach, a_style.y);

	// v_local is the fragment's offset from the pawn's centre in MAP PIXELS and
	// in the pawn's OWN frame, which is what makes the fragment shader's
	// distances real distances and lets a turned object be framed by a turned
	// rectangle without the shader ever learning the angle.
	v_local = (a_rect.zw + reach) * corner;

	vec2 turned = vec2(
		v_local.x * a_spin.x - v_local.y * a_spin.y,
		v_local.x * a_spin.y + v_local.y * a_spin.x
	);

	gl_Position = vec4((u_clip * vec3(a_rect.xy + turned, 1.0)).xy, 0.0, 1.0);
}
`;

const fragmentSource = `#version 300 es
precision highp float;

#define PI 3.141592653589793
#define TAU 6.283185307179586
#define PAD ${AURA_PAD}.0
#define NEAR ${AURA_NEAR}.0
#define FAR ${AURA_FAR}.0
#define NEAR_A ${AURA_NEAR_A}
#define FAR_A ${AURA_FAR_A}
#define TAIL ${AURA_TAIL}

in vec2 v_local;
flat in vec4 v_color;
flat in vec2 v_half;
flat in vec2 v_style;

uniform float u_scale;
uniform float u_turn;

out vec4 outColor;

// ramp is one period of aura-dual, as a fraction of that period: empty until
// the tail begins, then a straight climb to full colour, then a cut where the
// next period starts. The cut is the head of the comet and the climb behind it
// is the tail. DaisyUI writes the same shape as three colour stops -- clear at
// 0 per cent, clear at 40, currentColor at 50 -- over a gradient repeating
// every half turn; TAIL is where the 40 has been moved to.
float ramp(float q) {
	return clamp((q - (1.0 - TAIL)) / TAIL, 0.0, 1.0);
}

// dual is that ramp smeared along the arc, which is what the blurred copies of
// it look like. soft is the blur's radius expressed in periods AT THIS
// FRAGMENT'S OWN RADIUS, so the smear is the same distance on the table
// wherever it is measured rather than the same angle -- a fixed angle would
// smear the outside of the glow further than the inside and bend the tail.
//
// FIVE TAPS OVER A BOX RATHER THAN A GAUSSIAN. The thing being blurred is a
// linear ramp and a step; a box of the right width takes both to within a
// shade of what a real Gaussian would, and the ramp is most of what is on
// screen. fract() is what carries a tap across the seam between two periods,
// which is exactly where the head is and the one place an error would show.
float dual(float q, float soft) {
	if (soft < 1e-4) {
		return ramp(q);
	}

	float sum = 0.0;
	for (int i = -2; i <= 2; i++) {
		sum += ramp(fract(q + float(i) * soft * 0.6));
	}

	return sum * 0.2;
}

void main() {
	float reach = v_style.x;

	// outside is how far this fragment is beyond the pawn's own edge, in map
	// pixels, and it is the exact distance in both shapes: a circle's is its
	// radius less the pawn's, and a box's is the standard rounded-corner form,
	// which is what keeps the band an even width round a corner instead of
	// bulging at forty-five degrees.
	float outside;
	if (v_style.y < 0.5) {
		outside = length(v_local) - v_half.x;
	} else {
		vec2 q = abs(v_local) - v_half;
		outside = length(max(q, 0.0)) + min(max(q.x, q.y), 0.0);
	}

	// THE DERIVATIVE IS TAKEN BEFORE ANYTHING IS THROWN AWAY. fwidth reads the
	// fragments beside this one, and a neighbour that has already discarded is
	// not there to be read -- so the width of an antialiased edge has to be
	// measured while the whole quad is still being shaded.
	float aa = max(fwidth(outside), 1e-5);

	// The three sizes back in map pixels, together, because every one of them
	// is a screen size and this is the only place that knows the zoom.
	float scale = 1.0 / max(u_scale, 1e-4);
	float pad = PAD * scale;
	float near = NEAR * scale;
	float far = FAR * scale;

	// THE INSIDE IS A HOLE. In the DOM the content is what hides the fill; here
	// it is this, for the object with transparent corners -- see the note at
	// the top. Past the far end of the widest blur there is nothing left.
	float hole = smoothstep(-aa, 0.0, outside);
	if (hole <= 0.0 || outside > reach) {
		discard;
	}

	// Where round the pawn this fragment is, as a fraction of a turn, counted
	// from twelve o'clock and going clockwise -- which is where a CSS conic
	// gradient starts and which way it runs. Map y grows downward, so up is -y.
	// Subtracting the phase is what "from var(--aura-angle)" means: the pattern
	// turns clockwise as the angle grows.
	float turn = fract(atan(v_local.x, -v_local.y) / TAU - u_turn);

	// And the same thing in aura-dual's periods, of which there are two.
	float q = fract(turn * 2.0);

	// A blur of radius s at radius r smears an arc of s/r radians, which is
	// s/(PI*r) of a period. The radius is the fragment's own distance from the
	// centre and not the pawn's, so the far glow -- which is most of the way to
	// twice the pawn's width at the zoom a fight is run from -- is smeared by
	// the smaller angle it actually subtends out there.
	float r = max(length(v_local), 1e-3);
	float softNear = clamp(near / (PI * r), 0.0, 0.5);
	float softFar = clamp(far / (PI * r), 0.0, 0.5);

	// The three copies. The crisp one is the padding band itself, the other two
	// are that same band's edge softened across its blur -- a blurred step is a
	// smoothstep of the blur's own width, and past the far end of it there is
	// nothing left to draw.
	float band = hole * (1.0 - smoothstep(pad, pad + aa, outside));
	float crisp = band * dual(q, 0.0);
	float glowNear = hole * NEAR_A * (1.0 - smoothstep(pad - near, pad + near, outside)) * dual(q, softNear);
	float glowFar = hole * FAR_A * (1.0 - smoothstep(pad - far, pad + far, outside)) * dual(q, softFar);

	// Three layers of one colour, composited. Source-over of a stack that
	// shares an RGB is one subtraction per layer, so the framebuffer never
	// sees more than the answer.
	float alpha = 1.0 - (1.0 - crisp) * (1.0 - glowNear) * (1.0 - glowFar);
	alpha *= v_color.a;

	if (alpha <= 0.0) {
		discard;
	}

	outColor = vec4(v_color.rgb, alpha);
}
`;

const names = ["u_clip", "u_scale", "u_turn"] as const;

export interface AuraPass {
	// begin empties the buffer. There is no cache here for the reason the ring
	// pass has none: what is in it is one line of the turn order, and it turns
	// over at the rate a fight does.
	begin(): void;

	// add is one creature's aura: its centre and its OWN half extents -- the
	// hole, not the glow -- a colour, how solid the whole thing is, which shape
	// the hole is, and how far the pawn is turned.
	add(
		x: number, y: number, halfW: number, halfH: number,
		color: readonly [number, number, number], alpha: number,
		shape: number, rotation?: number,
	): void;

	// draw takes the phase as well as the camera, because where the tails have
	// got to is a property of the clock and not of the table. It is a uniform
	// for the pawn pulse's reason: one number for every instance at once, so
	// nothing per-instance has to be rewritten to move it.
	draw(cam: Camera, deviceWidth: number, deviceHeight: number, dpr: number, turn: number): void;
	dispose(): void;
}

// auraTurn is where the tails have got to, as a fraction of one revolution.
//
// IT IS A FUNCTION OF THE WALL CLOCK AND OF NOTHING ELSE, so it looks the same
// an hour into a session as in the first minute, and two creatures acting on
// one count -- nine goblins on a grouped line -- turn in step rather than each
// from whenever it happened to be added. That is the pulse's argument as well:
// together reads as one thing happening, out of phase reads as noise.
export function auraTurn(now: number): number {
	return (((now % AURA_PERIOD) + AURA_PERIOD) % AURA_PERIOD) / AURA_PERIOD;
}

// auraColor is what colour a creature's turn is drawn in, and the answer is
// gold until it is bleeding.
//
// THE RED IS THE BLOOD'S OWN, imported rather than copied, so a goblin's ring
// and the pool it is standing in are the one colour. The band behind it comes
// from healthOf, which is the number the whole table already reads.
//
// THE STRIP DOES NOT DO THIS AND DOES NOT NEED TO. Its aura is aura-silver,
// which carries its own colours and takes none from outside -- and a line of
// the turn order already says how hurt a creature is five ways over: blood at
// the rim, the colour draining out, a pulse, a splatter, a skull. Out here
// there is none of that on the token's outside, so the ring is where it goes.
//
// IT IS hurt()'s TABLE AND NOT A NEW ONE, which settles both ends of it.
// NOTHING ABOVE THE HALFWAY LINE IS MARKED: healthy, bruised and a line with no
// creature behind it are all gold, because a table where every aura is red is a
// table with no warning left in it. AND A CORPSE IS NOT MARKED EITHER: it is
// already grey and already wearing a skull, which says more than a colour
// could, and the dried red it would take is invisible on a dark map -- so what
// a dead line's turn gets is the gold that means nothing but "you are up".
export function auraColor(band: HPBand | null): readonly [number, number, number] {
	switch (band) {
		case "bloody":
		case "veryBloody":
		case "nearDeath":
			return BLOOD_FRESH;
		default:
			return AURA_GOLD;
	}
}

export function createAuraPass(gl: WebGL2RenderingContext): AuraPass {
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

	gl.enableVertexAttribArray(4);
	gl.vertexAttribPointer(4, 2, gl.FLOAT, false, stride, 48);
	gl.vertexAttribDivisor(4, 1);

	gl.bindVertexArray(null);
	gl.bindBuffer(gl.ARRAY_BUFFER, null);

	// Sixteen is the protocol's cap on a grouped line, so the buffer is built
	// once at the largest turn the server will hand over and never grown.
	let data = new Float32Array(16 * FLOATS_PER_INSTANCE);
	let count = 0;

	const matrix = new Float32Array(9);

	return {
		begin() {
			count = 0;
		},

		add(x, y, halfW, halfH, color, alpha, shape, rotation = 0) {
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

			data[at + 8] = 0;
			data[at + 9] = shape;
			data[at + 10] = 0;
			data[at + 11] = 0;

			const angle = radians(rotation);
			data[at + 12] = rotation === 0 ? 1 : Math.cos(angle);
			data[at + 13] = rotation === 0 ? 0 : Math.sin(angle);

			count++;
		},

		draw(cam, deviceWidth, deviceHeight, dpr, turn) {
			if (count === 0) {
				return;
			}

			gl.useProgram(program);
			gl.bindVertexArray(vao);
			gl.bindBuffer(gl.ARRAY_BUFFER, instances);
			gl.bufferData(gl.ARRAY_BUFFER, data.subarray(0, count * FLOATS_PER_INSTANCE), gl.DYNAMIC_DRAW);

			gl.uniform1f(at.u_scale, cam.zoom * dpr);
			gl.uniform1f(at.u_turn, turn);
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
