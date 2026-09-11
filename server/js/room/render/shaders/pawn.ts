const BORDER_PIXELS = 2;
const BLOOD_INNER = 0.25;
const PULSE_REACH = 0.24;
export const vertexSource = `#version 300 es
#define BORDER_PIXELS ${BORDER_PIXELS}.0
layout(location = 0) in vec2 a_corner;
layout(location = 1) in vec4 a_rect;
layout(location = 2) in vec4 a_border;
layout(location = 3) in vec4 a_style;
layout(location = 4) in vec4 a_fit;
layout(location = 5) in vec4 a_spin;
uniform mat3 u_clip;
uniform float u_scale;
out vec2 v_local;
flat out vec4 v_border;
flat out vec4 v_style;
flat out vec4 v_fit;
flat out vec2 v_wound;
flat out float v_edge;
void main() {
	v_local = a_corner * 2.0 - 1.0;
	v_border = a_border;
	v_style = a_style;
	v_fit = a_fit;
	v_wound = a_spin.zw;
	v_edge = BORDER_PIXELS / max(a_rect.z * u_scale, 1e-4);
	vec2 offset = v_local * a_rect.zw;
	vec2 turned = vec2(
		offset.x * a_spin.x - offset.y * a_spin.y,
		offset.x * a_spin.y + offset.y * a_spin.x
	);
	vec2 world = a_rect.xy + turned;
	gl_Position = vec4((u_clip * vec3(world, 1.0)).xy, 0.0, 1.0);
}
`;
export const fragmentSource = `#version 300 es
precision highp float;
precision highp sampler2DArray;
in vec2 v_local;
flat in vec4 v_border;
flat in vec4 v_style;
flat in vec4 v_fit;
flat in vec2 v_wound;
flat in float v_edge;
uniform sampler2DArray u_sprites;
uniform vec2 u_pulse;
const vec3 LUMA = vec3(0.299, 0.587, 0.114);
const vec3 BLOOD = vec3(0.42, 0.03, 0.03);
const vec3 PALLOR = vec3(0.82, 0.88, 1.0);
const vec3 PULSE = vec3(1.0, 0.24, 0.2);
out vec4 outColor;
void main() {
	float layer = v_style.x;
	float shape = v_style.y;
	float alpha = v_style.z;
	float grey  = v_style.w;
	vec2 t = (v_local / v_fit.xy) * 0.5 + 0.5;
	vec4 picture = vec4(0.0);
	if (layer >= 0.0 && t.x >= 0.0 && t.y >= 0.0 && t.x <= 1.0 && t.y <= 1.0) {
		picture = texture(u_sprites, vec3(t * v_fit.zw, layer));
	}
	vec3 rgb;
	float cover;
	if (shape < 0.5) {
		float r = length(v_local);
		float aa = max(fwidth(r), 1e-5);
		cover = 1.0 - smoothstep(1.0 - aa, 1.0, r);
		if (cover <= 0.0) {
			discard;
		}
		rgb = mix(v_border.rgb, picture.rgb, picture.a);
		float border = smoothstep(1.0 - v_edge - aa, 1.0 - v_edge, r);
		rgb = mix(rgb, v_border.rgb, border * v_border.a);
		float wounded = v_wound.x;
		if (wounded > 0.0) {
			float low = 0.55 + 0.45 * v_local.y;
			float rim = smoothstep(1.0 - (0.3 + 0.35 * wounded), 1.0, r) * low;
			rgb = mix(rgb, BLOOD, clamp(rim, 0.0, 1.0) * wounded);
			rgb = mix(rgb, PALLOR * dot(rgb, LUMA), 0.35 * wounded);
		}
		float beating = v_wound.y;
		if (beating > 0.5) {
			float amount = beating < 1.5 ? u_pulse.x : u_pulse.y;
			rgb = mix(rgb, PULSE, smoothstep(1.0 - ${PULSE_REACH}, 1.0, r) * amount);
		}
	} else if (shape < 1.5) {
		cover = picture.a;
		if (cover <= 0.0) {
			discard;
		}
		rgb = picture.rgb;
	} else {
		float r = length(v_local);
		float aa = max(fwidth(r), 1e-5);
		cover = picture.a
			* (1.0 - smoothstep(1.0 - aa, 1.0, r))
			* smoothstep(${BLOOD_INNER}, 1.0, r);
		if (cover <= 0.0) {
			discard;
		}
		rgb = v_border.rgb * picture.r;
	}
	rgb = mix(rgb, vec3(dot(rgb, LUMA)), grey);
	outColor = vec4(rgb, cover * alpha);
}
`;
export const uniforms = ["u_clip", "u_sprites", "u_scale", "u_pulse"] as const;
