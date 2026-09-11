export const vertexSource = `#version 300 es
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
	vec2 dir = len > 1e-4 ? delta / len : vec2(1.0, 0.0);
	vec2 nor = vec2(-dir.y, dir.x);
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
export const fragmentSource = `#version 300 es
precision highp float;
in vec2 v_local;
flat in vec4 v_color;
flat in float v_half;
flat in float v_len;
out vec4 outColor;
void main() {
	float t = clamp(v_local.x, 0.0, v_len);
	float dist = length(v_local - vec2(t, 0.0));
	float aa = max(fwidth(dist), 1e-5);
	float alpha = v_color.a * (1.0 - smoothstep(v_half - aa, v_half + aa, dist));
	if (alpha <= 0.0) {
		discard;
	}
	outColor = vec4(v_color.rgb, alpha);
}
`;
export const uniforms = ["u_clip", "u_minHalf"] as const;
export const layout = [{ size: 4 }, { size: 4 }, { size: 1 }] as const;
