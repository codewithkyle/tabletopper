export const vertexSource = `#version 300 es
layout(location = 0) in vec2 a_corner;
layout(location = 1) in vec4 a_rect;
layout(location = 2) in vec4 a_color;
layout(location = 3) in vec4 a_sheet;
layout(location = 4) in vec2 a_spin;
uniform mat3 u_clip;
out vec2 v_local;
flat out vec4 v_color;
flat out vec4 v_sheet;
void main() {
	v_local = a_corner * 2.0 - 1.0;
	v_color = a_color;
	v_sheet = a_sheet;
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
flat in vec4 v_color;
flat in vec4 v_sheet;
uniform sampler2DArray u_sprites;
out vec4 outColor;
void main() {
	vec2 t = (v_local * 0.5 + 0.5) * v_sheet.yz;
	vec4 picture = texture(u_sprites, vec3(t, v_sheet.x));
	float cover = picture.a * v_color.a;
	if (cover <= 0.0) {
		discard;
	}
	outColor = vec4(v_color.rgb * picture.r, cover);
}
`;
export const uniforms = ["u_clip", "u_sprites"] as const;
