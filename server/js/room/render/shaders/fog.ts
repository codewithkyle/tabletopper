export const shapeVertexSource = `#version 300 es
layout(location = 0) in vec2 a_map;
uniform vec4 u_rect;
void main() {
	vec2 t = (a_map - u_rect.xy) / u_rect.zw;
	gl_Position = vec4(t * 2.0 - 1.0, 0.0, 1.0);
}
`;
export const shapeFragmentSource = `#version 300 es
precision highp float;
uniform float u_open;
out vec4 outColor;
void main() {
	outColor = vec4(u_open, 0.0, 0.0, 1.0);
}
`;
export const coverVertexSource = `#version 300 es
layout(location = 0) in vec2 a_clip;
uniform mat3 u_clipToWorld;
out vec2 v_world;
void main() {
	v_world = (u_clipToWorld * vec3(a_clip, 1.0)).xy;
	gl_Position = vec4(a_clip, 0.0, 1.0);
}
`;
export const coverFragmentSource = `#version 300 es
precision highp float;
in vec2 v_world;
uniform vec4 u_rect;
uniform sampler2D u_openness;
uniform float u_prefill;
uniform vec4 u_color;
out vec4 outColor;
void main() {
	float open = 1.0 - u_prefill;
	if (u_rect.z > 0.0 && u_rect.w > 0.0) {
		vec2 t = (v_world - u_rect.xy) / u_rect.zw;
		if (t.x >= 0.0 && t.x <= 1.0 && t.y >= 0.0 && t.y <= 1.0) {
			open = texture(u_openness, t).r;
		}
	}
	float alpha = u_color.a * (1.0 - smoothstep(0.4, 0.6, open));
	if (alpha <= 0.0) {
		discard;
	}
	outColor = vec4(u_color.rgb, alpha);
}
`;
export const shapeUniforms = ["u_rect", "u_open"] as const;
export const coverUniforms = ["u_clipToWorld", "u_rect", "u_openness", "u_prefill", "u_color"] as const;
