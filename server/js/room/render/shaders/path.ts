export const vertexSource = `#version 300 es
layout(location = 0) in vec2 a_corner;
layout(location = 1) in vec4 a_rect;
layout(location = 2) in vec4 a_axis;
layout(location = 3) in vec4 a_color;
layout(location = 4) in vec4 a_uv;
uniform mat3 u_clip;
out vec2 v_uv;
flat out vec4 v_color;
flat out float v_textured;
void main() {
	vec2 world = a_rect.xy + a_corner.x * a_rect.zw + a_corner.y * a_axis.xy;
	v_uv = mix(a_uv.xy, a_uv.zw, a_corner);
	v_color = a_color;
	v_textured = a_axis.z;
	gl_Position = vec4((u_clip * vec3(world, 1.0)).xy, 0.0, 1.0);
}
`;
export const fragmentSource = `#version 300 es
precision highp float;
in vec2 v_uv;
flat in vec4 v_color;
flat in float v_textured;
uniform sampler2D u_atlas;
out vec4 outColor;
void main() {
	float alpha = v_color.a;
	if (v_textured > 0.5) {
		alpha *= texture(u_atlas, v_uv).a;
	}
	if (alpha <= 0.0) {
		discard;
	}
	outColor = vec4(v_color.rgb, alpha);
}
`;
export const uniforms = ["u_clip", "u_atlas"] as const;
