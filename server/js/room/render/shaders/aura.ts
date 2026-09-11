const AURA_PAD = 4;
const AURA_NEAR = 4;
const AURA_FAR = 16;
const AURA_NEAR_A = 0.7;
const AURA_FAR_A = 0.3;
const AURA_REACH = AURA_PAD + AURA_FAR + 2;
const AURA_TAIL = 0.6;
export const vertexSource = `#version 300 es
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
	float reach = REACH / max(u_scale, 1e-4);
	v_style = vec2(reach, a_style.y);
	v_local = (a_rect.zw + reach) * corner;
	vec2 turned = vec2(
		v_local.x * a_spin.x - v_local.y * a_spin.y,
		v_local.x * a_spin.y + v_local.y * a_spin.x
	);
	gl_Position = vec4((u_clip * vec3(a_rect.xy + turned, 1.0)).xy, 0.0, 1.0);
}
`;
export const fragmentSource = `#version 300 es
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
float ramp(float q) {
	return clamp((q - (1.0 - TAIL)) / TAIL, 0.0, 1.0);
}
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
	float outside;
	if (v_style.y < 0.5) {
		outside = length(v_local) - v_half.x;
	} else {
		vec2 q = abs(v_local) - v_half;
		outside = length(max(q, 0.0)) + min(max(q.x, q.y), 0.0);
	}
	float aa = max(fwidth(outside), 1e-5);
	float scale = 1.0 / max(u_scale, 1e-4);
	float pad = PAD * scale;
	float near = NEAR * scale;
	float far = FAR * scale;
	float hole = smoothstep(-aa, 0.0, outside);
	if (hole <= 0.0 || outside > reach) {
		discard;
	}
	float turn = fract(atan(v_local.x, -v_local.y) / TAU - u_turn);
	float q = fract(turn * 2.0);
	float r = max(length(v_local), 1e-3);
	float softNear = clamp(near / (PI * r), 0.0, 0.5);
	float softFar = clamp(far / (PI * r), 0.0, 0.5);
	float band = hole * (1.0 - smoothstep(pad, pad + aa, outside));
	float crisp = band * dual(q, 0.0);
	float glowNear = hole * NEAR_A * (1.0 - smoothstep(pad - near, pad + near, outside)) * dual(q, softNear);
	float glowFar = hole * FAR_A * (1.0 - smoothstep(pad - far, pad + far, outside)) * dual(q, softFar);
	float alpha = 1.0 - (1.0 - crisp) * (1.0 - glowNear) * (1.0 - glowFar);
	alpha *= v_color.a;
	if (alpha <= 0.0) {
		discard;
	}
	outColor = vec4(v_color.rgb, alpha);
}
`;
export const uniforms = ["u_clip", "u_scale", "u_turn"] as const;
