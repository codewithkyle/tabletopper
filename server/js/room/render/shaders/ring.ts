export const vertexSource = `#version 300 es
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
	v_local = a_corner * 2.0 - 1.0;
	v_color = a_color;
	v_half = a_rect.zw;
	v_style = vec2(a_style.x / max(u_scale, 1e-4), a_style.y);
	vec2 grown = a_rect.zw + v_style.x;
	vec2 offset = v_local * grown;
	vec2 turned = vec2(
		offset.x * a_spin.x - offset.y * a_spin.y,
		offset.x * a_spin.y + offset.y * a_spin.x
	);
	vec2 world = a_rect.xy + turned;
	gl_Position = vec4((u_clip * vec3(world, 1.0)).xy, 0.0, 1.0);
	v_local *= grown / max(a_rect.zw, vec2(1e-4));
}
`;
export const fragmentSource = `#version 300 es
precision highp float;
in vec2 v_local;
flat in vec4 v_color;
flat in vec2 v_half;
flat in vec2 v_style;
out vec4 outColor;
void main() {
	float thickness = v_style.x;
	float inside;
	if (v_style.y < 0.5) {
		float r = length(v_local);
		inside = (1.0 - r) * v_half.x;
	} else {
		vec2 edge = (vec2(1.0) - abs(v_local)) * v_half;
		inside = min(edge.x, edge.y);
	}
	float aa = max(fwidth(inside), 1e-5);
	float alpha = smoothstep(-thickness * 0.5 - aa, -thickness * 0.5, inside)
		* (1.0 - smoothstep(thickness * 0.5, thickness * 0.5 + aa, inside));
	if (alpha <= 0.0) {
		discard;
	}
	outColor = vec4(v_color.rgb, v_color.a * alpha);
}
`;
export const uniforms = ["u_clip", "u_scale"] as const;
