export const map_vert_shader =
`#version 300 es
in vec2 a_position;
in vec2 a_texCoord;
out vec2 v_texCoord;

uniform vec2 u_resolution;
uniform vec2 u_translation;
uniform vec2 u_scale;

void main() {
    // apply translation
    vec2 translatedPosition = a_position * u_scale + u_translation;

    // convert pixel coord to normalized device coord
    vec2 zeroToOne = translatedPosition / u_resolution;
    vec2 zeroToTwo = zeroToOne * 2.0;
    vec2 clipSpace = zeroToTwo - 1.0;

    gl_Position = vec4(clipSpace * vec2(1, -1), 0, 1); // flip y-axis
    v_texCoord = a_texCoord; // pass tex coords
}
`;
export const map_frag_shader =
`#version 300 es
precision mediump float;

in vec2 v_texCoord;
uniform sampler2D u_texture;
out vec4 outColor;

void main() {
    outColor = texture(u_texture, v_texCoord);
}
`;

export const grid_vert_shader =
`#version 300 es

in vec2 a_position;

void main() {
    gl_Position = vec4(a_position, 0.0, 1.0);
}
`;

export const grid_frag_shader =
`#version 300 es
precision highp float;

uniform vec2 u_resolution;
uniform float u_scale;
uniform float u_spacing;
uniform vec2 u_translation;
uniform vec4 u_color;

out vec4 outColor;

void main() {
    vec2 pixelPos = gl_FragCoord.xy - 0.5;
    pixelPos.x -= u_translation.x;
    pixelPos.y += u_translation.y;

    vec2 worldPos = pixelPos / u_scale;

    vec2 modPos = mod(worldPos, u_spacing);
    vec2 cellCoord = modPos / u_spacing;
    float distX = min(cellCoord.x, 1.0 - cellCoord.x);
    float distY = min(cellCoord.y, 1.0 - cellCoord.y);

    float thickness = 0.02;
    float edgeSize = 0.02;  // tweak for sharper/softer edges
    float edge0 = thickness - edgeSize;
    float edge1 = thickness + edgeSize;

    float alphaX = 1.0 - smoothstep(edge0, edge1, distX);
    float alphaY = 1.0 - smoothstep(edge0, edge1, distY);
    float line = max(alphaX, alphaY);

    vec4 baseColor = vec4(0.0, 0.0, 0.0, 0.0);
    outColor = mix(baseColor, u_color, line);
}
`;
