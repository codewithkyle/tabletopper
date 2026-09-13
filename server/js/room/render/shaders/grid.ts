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
uniform float u_type;
out vec4 outColor;
const float DASH_PERIOD = 0.25;
const float DASH_HALF = 0.3;
const float SQRT3 = 1.7320508075688772;
float dash(float along, float perPixel) {
	float s = abs(fract(along / DASH_PERIOD + 0.5) - 0.5);
	float w = max(perPixel / DASH_PERIOD, 1e-8);
	return 1.0 - smoothstep(DASH_HALF - w, DASH_HALF + w, s);
}
vec2 axialOf(vec2 p) {
	if (u_type > 1.5) {
		return vec2(2.0 * p.x / (u_cell * SQRT3), p.y / u_cell - p.x / (u_cell * SQRT3));
	}
	return vec2(p.x / u_cell - p.y / (u_cell * SQRT3), 2.0 * p.y / (u_cell * SQRT3));
}
vec2 centreOf(vec2 axial) {
	if (u_type > 1.5) {
		return vec2(u_cell * (SQRT3 / 2.0) * axial.x, u_cell * (axial.x / 2.0 + axial.y));
	}
	return vec2(u_cell * (axial.x + axial.y / 2.0), u_cell * (SQRT3 / 2.0) * axial.y);
}
vec2 roundAxial(vec2 f) {
	float x = f.x;
	float z = f.y;
	float y = -x - z;
	float rx = floor(x + 0.5);
	float ry = floor(y + 0.5);
	float rz = floor(z + 0.5);
	float dx = abs(rx - x);
	float dy = abs(ry - y);
	float dz = abs(rz - z);
	if (dx > dy && dx > dz) {
		rx = -ry - rz;
	} else if (dy > dz) {
		ry = -rx - rz;
	} else {
		rz = -rx - ry;
	}
	return vec2(rx, rz);
}
void squareGrid(out float line, out float perPixel) {
	vec2 cells = (v_world - u_offset) / u_cell;
	vec2 px = fwidth(cells);
	vec2 toEdge = abs(fract(cells - 0.5) - 0.5) / max(px, vec2(1e-8));
	vec2 lit = 1.0 - clamp(toEdge, 0.0, 1.0);
	if (u_dash > 0.5) {
		lit.x *= dash(cells.y, px.y);
		lit.y *= dash(cells.x, px.x);
	}
	line = max(lit.x, lit.y);
	perPixel = max(px.x, px.y);
}
void hexGrid(out float line, out float perPixel) {
	vec2 p = v_world - u_offset;
	vec2 d = p - centreOf(roundAxial(axialOf(p)));
	vec2 n0 = u_type > 1.5 ? vec2(0.0, 1.0) : vec2(1.0, 0.0);
	vec2 n1 = u_type > 1.5 ? vec2(SQRT3 / 2.0, 0.5) : vec2(0.5, SQRT3 / 2.0);
	vec2 n2 = u_type > 1.5 ? vec2(-SQRT3 / 2.0, 0.5) : vec2(-0.5, SQRT3 / 2.0);
	float a0 = abs(dot(d, n0));
	float a1 = abs(dot(d, n1));
	float a2 = abs(dot(d, n2));
	vec2 won = n0;
	float reach = a0;
	if (a1 > reach) {
		reach = a1;
		won = n1;
	}
	if (a2 > reach) {
		reach = a2;
		won = n2;
	}
	float edge = u_cell * 0.5 - reach;
	float px = max(fwidth(v_world.x), fwidth(v_world.y));
	line = 1.0 - clamp(edge / max(px, 1e-8), 0.0, 1.0);
	if (u_dash > 0.5) {
		line *= dash(dot(d, vec2(-won.y, won.x)) / u_cell, px / u_cell);
	}
	perPixel = px / u_cell;
}
void main() {
	float line = 0.0;
	float perPixel = 1.0;
	if (u_type > 0.5) {
		hexGrid(line, perPixel);
	} else {
		squareGrid(line, perPixel);
	}
	float fade = smoothstep(2.0, 6.0, 1.0 / max(perPixel, 1e-8));
	float alpha = u_color.a * line * fade;
	if (alpha <= 0.0) {
		discard;
	}
	outColor = vec4(u_color.rgb, alpha);
}
`;
export const uniforms = ["u_clipToWorld", "u_offset", "u_cell", "u_color", "u_dash", "u_type"] as const;
