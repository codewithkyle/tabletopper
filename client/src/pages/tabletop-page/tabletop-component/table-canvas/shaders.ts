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
in vec4 a_position;
out vec2 v_uv;

void main() {
    gl_Position = a_position;

    // normalize device coords [-1, 1] to UV coords
    v_uv = a_position.xy * 0.5 + 0.5;
}
`;

export const grid_frag_shader =
`#version 300 es
precision highp float;

uniform float u_spacing;
uniform vec2 u_resolution;
uniform vec2 u_origin;
uniform vec4 u_color;

in vec2 v_uv;
out vec4 fragColor;

void main() {
    // convert UV to screen space
    vec2 screenPos = v_uv * u_resolution;
    screenPos.x -= + u_origin.x;
    screenPos.y += + u_origin.y;

    float gridX = mod(screenPos.x, u_spacing);
    float gridY = mod(screenPos.y, u_spacing);

    float lineWidth = 1.0;
    float horizontalLine = step(gridX, lineWidth);
    float verticalLine = step(gridY, lineWidth);

    float grid = max(horizontalLine, verticalLine);

    if (grid > 0.0) {
        fragColor = u_color; // line
    } else {
        fragColor = vec4(0.0, 0.0, 0.0, 0.0); // bg
    }
}
`;
