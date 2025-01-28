export class Program {
    private gl: WebGLRenderingContext;
    private program: WebGLProgram;
    private uniforms: {
        [key:string]: WebGLUniformLocation;
    }
    private attributes: {
        [key:string]: number;
    }
    private vs:WebGLShader;
    private fs:WebGLShader;

    constructor(gl: WebGLRenderingContext){
        this.vs = undefined;
        this.fs = undefined;
        this.gl = gl;
        this.uniforms = {};
        this.attributes = {};
        this.program = undefined;
    }

    public add_vertex_shader(source:string) {
        this.vs = this.compile_shader(source, this.gl.VERTEX_SHADER);
        return this;
    }

    public add_fragment_shader(source:string) {
        this.fs = this.compile_shader(source, this.gl.FRAGMENT_SHADER);
        return this;
    }

    public build() {
        this.program = this.link_program();
        return this;
    }

    public get_uniform(uniform: string) {
        if (!(uniform in this.uniforms)) {
            throw new Error(`Failed to get uniform: ${uniform}.`);
        }
        return this.uniforms[uniform];
    }

    public get_attribute(attr: string) {
        if (!(attr in this.attributes)) {
            throw new Error(`Failed to get attribute: ${attr}.`);
        }
        return this.attributes[attr];
    }

    public get_program() {
        return this.program;
    }

    public build_uniforms(uniforms:string[]) {
        if (this.program === undefined) {
            throw new Error("Failed to get uniforms: missing program. Call build() before getting uniforms.");
        }
        uniforms.forEach(name => {
            this.uniforms[name] = this.gl.getUniformLocation(this.program, name);
        });
        return this;
    }

    public build_attributes(attrs:string[]) {
        if (this.program === undefined) {
            throw new Error("Failed to get attributes: missing program. Call build() before getting attributes.");
        }
        attrs.forEach(name => {
            this.attributes[name] = this.gl.getAttribLocation(this.program, name);
        });
        return this;
    }

    private compile_shader(source:string, type:number) {
        const shader = this.gl.createShader(type);
        this.gl.shaderSource(shader, source);
        this.gl.compileShader(shader);
        if (!this.gl.getShaderParameter(shader, this.gl.COMPILE_STATUS)) {
            console.error(this.gl.getShaderInfoLog(shader));
            const shaderType = this.shader_type_to_string(shader);
            this.gl.deleteShader(shader);
            throw new Error(`Failed to compile ${shaderType} shader.`);
        }
        return shader;
    }

    private link_program() {
        if (this.vs === undefined) {
            throw new Error("Failed to link program: missing vertex shader. Call add_vertex_shader() before build.");
        } else if (this.fs === undefined) {
            throw new Error("Failed to link program: missing fragment shader. Call add_fragment_shader() before build.");
        }
        const program = this.gl.createProgram();
        this.gl.attachShader(program, this.vs);
        this.gl.attachShader(program, this.fs);
        this.gl.linkProgram(program);
        if (!this.gl.getProgramParameter(program, this.gl.LINK_STATUS)) {
            console.error("Failed to link program:", this.gl.getProgramInfoLog(program));
            throw new Error("Linking program has failed");
        }
        return program;
    }

    private shader_type_to_string(type:WebGLShader) {
        switch(type) {
            case this.gl.FRAGMENT_SHADER:
                return "FRAGMENT_SHADER";
            case this.gl.VERTEX_SHADER:
                return "VERTEX_SHADER";
            default:
                return "UNKNOWN";
        }
    }
}
