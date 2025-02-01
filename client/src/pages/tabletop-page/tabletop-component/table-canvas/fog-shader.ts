export const fog_mask_vert_shader =
`#version 300 es

in vec2 a_position;

void main() {
    gl_Position = vec4(a_position, 0.0, 1.0);
}
`;

export const fog_mask_frag_shader =
`#version 300 es
precision mediump float;

out vec4 outColor;

void main() {
    outColor = vec4(0.0, 0.0, 1.0, 1.0);
}
`;
