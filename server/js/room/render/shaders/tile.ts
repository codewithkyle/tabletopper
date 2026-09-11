export const vertexSource = `#version 300 es
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
export const fragmentSource = `#version 300 es
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
export const uniforms = ["u_clip", "u_tiles", "u_alpha"] as const;
