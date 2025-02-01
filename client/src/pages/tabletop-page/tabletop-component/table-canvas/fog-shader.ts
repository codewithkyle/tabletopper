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
    outColor = vec4(1.0, 1.0, 1.0, 1.0);
}
`;

export const fog_composite_vert_shader = 
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

export const fog_composite_frag_shader = 
`#version 300 es
precision mediump float;

// Varying texture coordinate from the vertex shader
in vec2 v_texCoord;

// The two texture uniforms: one for the custom image and one for the mask.
uniform sampler2D u_image;
uniform sampler2D u_mask;
uniform vec4 u_color;
uniform int u_isGM;

// Output color of the fragment.
out vec4 fragColor;

void main() {
    // Sample the custom image texture at the given UV coordinates.
    vec4 imageColor = texture(u_image, v_texCoord);
    
    // Sample the mask texture at the same UV coordinates.
    // Assuming the mask is in grayscale, we only need one channel.
    float maskValue = texture(u_mask, v_texCoord).r;
    
    // Use a threshold to decide if the mask is "white" (revealed).
    // You can adjust the threshold (here 0.5) as needed.
    if(maskValue > 0.5) {
        fragColor = imageColor; // Reveal the image where the mask is white.
    } else {
        if (u_isGM == 1) {
            fragColor = mix(imageColor, u_color, 0.5); // Otherwise, output black.
        } else {
            fragColor = u_color;
        }
    }
}

`;
