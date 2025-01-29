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
out vec2 v_clipPos;

void main() {
    gl_Position = vec4(a_position, 0.0, 1.0);
    v_clipPos = a_position;
}
`;

export const grid_frag_shader =
`#version 300 es
precision highp float;

uniform vec2 u_origin;      // The line's origin in top‐left coords
uniform vec2 u_resolution;  // (width, height) of the screen
uniform float u_spacing;    // grid spacing
uniform vec2 u_offset;
uniform float u_scale;
uniform vec4 u_color;

out vec4 outColor;

void main() {
    // gl_FragCoord is the window coordinate:
    //    gl_FragCoord.x in [0, W], with 0 at LEFT, W at RIGHT
    //    gl_FragCoord.y in [0, H], with 0 at BOTTOM, H at TOP
    
    // But your "top‐right origin" system wants:
    //    (0,0) in the TOP‐RIGHT
    //    x increasing leftward => x = W - gl_FragCoord.x
    //    y increasing downward => y = H - gl_FragCoord.y

    float worldX = gl_FragCoord.x;
    float worldY = u_resolution.y - gl_FragCoord.y;

    float cellSize = u_spacing * u_scale;

    // Compare the fragment's coordinate to the origin
    float distX = mod((worldX - u_origin.x - u_offset.x), cellSize);
    float distY = mod((worldY - u_origin.y - u_offset.y), cellSize);

    bool onVertical = distX < 1.0;
    bool onHorizontal = distY < 1.0;

    // If we're within the line width of either axis, draw it
    if (onVertical || onHorizontal) {
        outColor = u_color;
    } else {
        outColor = vec4(0.0, 0.0, 0.0, 0.0);  // transparent background
    }
}
`;
