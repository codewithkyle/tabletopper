export const vertexSource = `#version 300 es
layout(location = 0) in vec2 a_clip;
uniform mat3 u_clipToWorld;
out vec2 v_world;
void main() {
	v_world = (u_clipToWorld * vec3(a_clip, 1.0)).xy;
	gl_Position = vec4(a_clip, 0.0, 1.0);
}
`;
export const fragmentSource = `#version 300 es
precision highp float;
in vec2 v_world;
uniform vec2 u_offset;
uniform float u_cell;
uniform vec4 u_color;
uniform float u_dash;
out vec4 outColor;
const float DASH_PERIOD = 0.25;
const float DASH_HALF = 0.3;
float dash(float along, float perPixel) {
	float s = abs(fract(along / DASH_PERIOD + 0.5) - 0.5);
	float w = max(perPixel / DASH_PERIOD, 1e-8);
	return 1.0 - smoothstep(DASH_HALF - w, DASH_HALF + w, s);
}
void main() {
	vec2 cells = (v_world - u_offset) / u_cell;
	vec2 perPixel = fwidth(cells);
	vec2 toEdge = abs(fract(cells - 0.5) - 0.5) / max(perPixel, vec2(1e-8));
	vec2 line = 1.0 - clamp(toEdge, 0.0, 1.0);
	if (u_dash > 0.5) {
		line.x *= dash(cells.y, perPixel.y);
		line.y *= dash(cells.x, perPixel.x);
	}
	float fade = smoothstep(2.0, 6.0, 1.0 / max(max(perPixel.x, perPixel.y), 1e-8));
	float alpha = u_color.a * max(line.x, line.y) * fade;
	if (alpha <= 0.0) {
		discard;
	}
	outColor = vec4(u_color.rgb, alpha);
}
`;
export const uniforms = ["u_clipToWorld", "u_offset", "u_cell", "u_color", "u_dash"] as const;
