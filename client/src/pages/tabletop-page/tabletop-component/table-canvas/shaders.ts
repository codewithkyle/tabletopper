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
out vec2 v_uv;

void main() {
    gl_Position = vec4(a_position, 0.0, 1.0);
    v_uv = a_position;
}
`;

export const grid_frag_shader =
`#version 300 es
precision highp float;

in vec2 v_uv;

uniform vec2 u_resolution;
uniform vec2 u_scale;
uniform float u_spacing;
uniform vec2 u_translation;
uniform vec4 u_color;

out vec4 outColor;

void main() {
    vec2 pixelPos = 0.5 * (v_uv + 1.0) * u_resolution;
    pixelPos.x -= u_translation.x;
    pixelPos.y += u_translation.y;

    float x = mod(pixelPos.x, u_spacing * u_scale.x);
    float y = mod(pixelPos.y, u_spacing * u_scale.y);

    bool onVerticalLine = (x <= 1.0);
    bool onHorizontalLine = (y <= 1.0);

    if (onVerticalLine || onHorizontalLine) {
        outColor = u_color;
    } else {
        outColor = vec4(0.0, 0.0, 0.0, 0.0);
    }
}
`;
