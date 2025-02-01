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
