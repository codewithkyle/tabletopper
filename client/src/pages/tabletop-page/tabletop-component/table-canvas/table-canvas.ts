import SuperComponent from "@codewithkyle/supercomponent";
import env from "~brixi/controllers/env";
import { subscribe } from "@codewithkyle/pubsub";
import TabeltopComponent from "../tabletop-component";
import { send } from "~controllers/ws";
import { Program } from "./program";
import { map_frag_shader, map_vert_shader } from "./map-shader";
import { grid_frag_shader, grid_vert_shader } from "./grid-shader";
import { fog_composite_frag_shader, fog_composite_vert_shader, fog_mask_frag_shader, fog_mask_vert_shader } from "./fog-shader";
import room from "room";
import earcut from 'https://cdn.jsdelivr.net/npm/earcut/+esm';

type Point = {
    x: number,
    y: number,
}
type FogOfWarShape = {
    type: "poly" | "rect",
    points: Array<Point>,
}

interface ITableCanvas { }
export default class TableCanvas extends SuperComponent<ITableCanvas> {
    private canvas: HTMLCanvasElement;
    private fogctx: CanvasRenderingContext2D;
    private gl: WebGL2RenderingContext;
    private renderGrid: boolean;
    private gridSize: number;
    private gridOffset: Array<number>;
    private gridColor: Array<number>;
    private fogOfWar: boolean;
    private w: number;
    private h: number;
    private fogOfWarShapes: Array<FogOfWarShape>;
    private tabletop: TabeltopComponent;
    private image: HTMLImageElement;
    private updateGrid: boolean;
    private updateFog: boolean;
    private imgProgram: Program;
    private gridProgram: Program;
    private maskProgram: Program;
    private fogProgram: Program;
    private time: number;
    private pos: {
        x: number,
        y: number,

    };
    private doMove: boolean;
    private buildingFog: boolean;
    private loadingImage: boolean;
    private lastFogCount: number;
    private animationId: any;

    constructor() {
        super();
        this.animationId = null;
        this.lastFogCount = 0;
        this.buildingFog = false;
        this.loadingImage = false;
        this.w = window.innerWidth;
        this.h = window.innerHeight;
        this.pos = {
            x: 0,
            y: 0,
        };
        this.doMove = false;
        this.canvas = document.createElement("canvas") as HTMLCanvasElement;
        this.canvas.width = window.innerWidth;
        this.canvas.height = window.innerHeight;
        this.gl = this.canvas.getContext("webgl2");
        this.imgProgram = undefined;
        this.gridProgram = undefined;
        this.maskProgram = undefined;
        this.tabletop = document.querySelector("tabletop-component");
        this.renderGrid = false;
        this.gridSize = 32;
        this.fogOfWar = false;
        this.fogOfWarShapes = [];
        this.image = null;
        this.updateGrid = false;
        this.updateFog = false;

        subscribe("socket", this.inbox.bind(this));
        subscribe("fog", this.fogInbox.bind(this));
        subscribe("tabletop", this.tableInbox.bind(this));

        this.buildGridLinesProgram();
    }

    override async connected() {
        await env.css(["table-canvas"]);
        this.appendChild(this.canvas);
        window.addEventListener("resize", this.debounce(() => {
            this.canvas.width = window.innerWidth;
            this.canvas.height = window.innerHeight;
            this.w = window.innerWidth;
            this.h = window.innerHeight;
            this.gl.viewport(0, 0, this.canvas.width, this.canvas.height);
        }, 150));
    }

    public convertViewportToTabletopPosition(clientX: number, clientY: number): Array<number> {
        const x = Math.round(clientX - this.pos.x) / this.tabletop.zoom;
        const y = Math.round(clientY - this.pos.y) / this.tabletop.zoom;
        return [x, y];
    }

    private tableInbox(data) {
        const type = data?.type ?? "";
        switch (type) {
            case "zoom":
                this.pos.x = data.x;
                this.pos.y = data.y;
                this.doMove = true;
                break;
            case "move":
                this.pos.x = data.x;
                this.pos.y = data.y;
                this.doMove = true;
                break;
            default:
                break;
        }
    }

    private fogInbox({ type, points }) {
        const convertedPoints = [];
        for (let i = 0; i < points.length; i++) {
            const [x, y] = this.convertViewportToTabletopPosition(points[i].x, points[i].y);
            convertedPoints.push({ x, y });
        }
        switch (type) {
            case "rect":
                const newRect: FogOfWarShape = {
                    type: "rect",
                    points: convertedPoints,
                };
                this.fogOfWarShapes.push(newRect);
                this.sync(newRect);
                break;
            case "poly":
                const newPoly: FogOfWarShape = {
                    type: "poly",
                    points: convertedPoints,
                };
                this.fogOfWarShapes.push(newPoly);
                this.sync(newPoly);
                break;
            default:
                break;
        }
    }

    private inbox({ type, data }) {
        switch (type) {
            case "room:tabletop:fog:add":
                this.fogOfWar = data.fogOfWar;
                this.fogOfWarShapes.push(data.fogOfWarShapes);
                this.updateFog = true;
                break;
            case "room:tabletop:fog:sync":
                this.fogOfWar = data.fogOfWar;
                this.fogOfWarShapes = data.fogOfWarShapes;
                this.lastFogCount = -1;
                this.updateFog = true;
                if (this.fogOfWar) {
                    this.buildFogProgram();
                } else {
                    this.fogProgram = undefined;
                    this.maskProgram = undefined;
                }
                break;
            case "room:tabletop:clear":
                this.fogOfWarShapes = [];
                this.lastFogCount = 0;
                this.imgProgram = undefined;
                this.image = null;
                this.maskProgram = undefined;
                this.fogProgram = undefined;
                this.updateFog = true;
                break;
            case "room:tabletop:map:update":
                this.renderGrid = data.renderGrid;
                this.gridSize = data.cellSize;
                this.fogOfWar = data.prefillFog;
                this.gridColor = this.hex_to_rgbaf(data.gridColor);
                this.gridOffset = data.gridOffset;
                this.updateGrid = true;
                this.updateFog = true;
                if (this.fogOfWar) {
                    this.buildFogProgram();
                } else {
                    this.fogProgram = undefined;
                    this.maskProgram = undefined;
                }
                break;
            default:
                break;
        }
    }

    private sync(shape: FogOfWarShape) {
        send("room:tabletop:fog:add", shape);
    }

    private buildFogTexture() {
        if (this.loadingImage || !this.image) {
            console.error("Cannot build fog texture without map image.");
            return;
        } else if (this.maskProgram === undefined) {
            console.error("Cannot build fog texture without fog of war shader programs.");
            return;
        }

        this.gl.useProgram(this.maskProgram.get_program());
        this.gl.bindFramebuffer(this.gl.FRAMEBUFFER, this.maskProgram.get_fbo());
        this.gl.bindTexture(this.gl.TEXTURE_2D, this.maskProgram.get_texture());

        this.gl.texImage2D(
            this.gl.TEXTURE_2D,
            0, // level
            this.gl.RGBA,
            this.image.width,
            this.image.height,
            0, // border
            this.gl.RGBA,
            this.gl.UNSIGNED_BYTE,
            null
        );
        this.gl.texParameteri(this.gl.TEXTURE_2D, this.gl.TEXTURE_MIN_FILTER, this.gl.LINEAR);
        this.gl.texParameteri(this.gl.TEXTURE_2D, this.gl.TEXTURE_MAG_FILTER, this.gl.LINEAR);
        this.gl.texParameteri(this.gl.TEXTURE_2D, this.gl.TEXTURE_WRAP_S, this.gl.CLAMP_TO_EDGE);
        this.gl.texParameteri(this.gl.TEXTURE_2D, this.gl.TEXTURE_WRAP_T, this.gl.CLAMP_TO_EDGE);

        this.gl.framebufferTexture2D(
            this.gl.FRAMEBUFFER,
            this.gl.COLOR_ATTACHMENT0,
            this.gl.TEXTURE_2D,
            this.maskProgram.get_texture(),
            0
        );

        this.gl.bindTexture(this.gl.TEXTURE_2D, null);
        this.gl.bindFramebuffer(this.gl.FRAMEBUFFER, null);
    }

    private buildFogProgram() {
        if (!this.image || this.loadingImage || this.buildingFog) return;
        this.buildingFog = true;

        if (this.maskProgram === undefined) {
            this.maskProgram = new Program(this.gl)
                .add_vertex_shader(fog_mask_vert_shader)
                .add_fragment_shader(fog_mask_frag_shader)
                .build()
                .create_buffer("verticies")
                .build_attributes(["a_position"])
                .create_texture()
                .create_fbo();

            this.buildFogTexture();
        }

        if (this.fogProgram === undefined) {
            this.fogProgram = new Program(this.gl)
                    .add_vertex_shader(fog_composite_vert_shader)
                    .add_fragment_shader(fog_composite_frag_shader)
                    .build()
                    .build_uniforms(["u_image", "u_mask", "u_resolution", "u_scale", "u_translation", "u_color", "u_isGM"])
                    .build_attributes(["a_position", "a_texCoord"])
                    .set_verticies(new Float32Array([
                        0, 0, 0.0, 0.0, // top-left
                        this.image.width, 0, 1.0, 0.0, // top-right
                        0, this.image.height, 0.0, 1.0, // bottom-left
                        this.image.width, this.image.height, 1.0, 1.0 // bottom-right
                    ]))
                    .set_indices(new Uint16Array([
                        0, 1, 2,  // First triangle
                        2, 1, 3   // Second triangle
                    ]))
                    .create_buffer("verticies")
                    .create_buffer("indices")
                    .create_vao();

            this.gl.useProgram(this.fogProgram.get_program());

            this.gl.bindVertexArray(this.fogProgram.get_vao());

            this.gl.bindBuffer(this.gl.ARRAY_BUFFER, this.fogProgram.get_buffer("verticies"));
            this.gl.bufferData(this.gl.ARRAY_BUFFER, this.fogProgram.get_verticies(), this.gl.STATIC_DRAW);

            const stride = 4 * Float32Array.BYTES_PER_ELEMENT;
            this.gl.vertexAttribPointer(this.fogProgram.get_attribute("a_position"), 2, this.gl.FLOAT, false, stride, 0);
            this.gl.enableVertexAttribArray(this.fogProgram.get_attribute("a_position"));
            this.gl.vertexAttribPointer(this.fogProgram.get_attribute("a_texCoord"), 2, this.gl.FLOAT, false, stride, 2 * 4);
            this.gl.enableVertexAttribArray(this.fogProgram.get_attribute("a_texCoord"));

            this.gl.bindBuffer(this.gl.ELEMENT_ARRAY_BUFFER, this.fogProgram.get_buffer("indices"));
            this.gl.bufferData(this.gl.ELEMENT_ARRAY_BUFFER, this.fogProgram.get_indices(), this.gl.STATIC_DRAW);

            this.gl.bindVertexArray(null);
            this.gl.bindBuffer(this.gl.ARRAY_BUFFER, null);
        }
        this.buildingFog = false;
    }

    private buildGridLinesProgram() {
        this.gridProgram = new Program(this.gl)
            .add_vertex_shader(grid_vert_shader)
            .add_fragment_shader(grid_frag_shader)
            .build()
            .build_uniforms(["u_resolution", "u_spacing", "u_origin", "u_color", "u_scale", "u_offset"])
            .build_attributes(["a_position"])
            .set_verticies(new Float32Array([
                -1, -1,
                +1, -1,
                -1, +1,
                +1, +1
            ]))
            .create_buffer("verticies")
            .create_vao();
        this.gl.useProgram(this.gridProgram.get_program());
        this.gl.bindVertexArray(this.gridProgram.get_vao());

        this.gl.bindBuffer(this.gl.ARRAY_BUFFER, this.gridProgram.get_buffer("verticies"));
        this.gl.bufferData(this.gl.ARRAY_BUFFER, this.gridProgram.get_verticies(), this.gl.STATIC_DRAW);

        this.gl.enableVertexAttribArray(this.gridProgram.get_attribute("a_position"));
        this.gl.vertexAttribPointer(this.gridProgram.get_attribute("a_position"), 2, this.gl.FLOAT, false, 0, 0);

        this.gl.bindVertexArray(null);
    }

    public load(imageSrc: string): Promise<Array<number>> {
        return new Promise((resolve) => {
            this.loadingImage = true;
            this.imgProgram = undefined;
            this.maskProgram = undefined;
            this.fogProgram = undefined;
            this.image = null;
            if (this.animationId) window.cancelAnimationFrame(this.animationId);
            this.forceClear();
            this.gl = this.canvas.getContext("webgl2");

            if (imageSrc == null) {
                this.loadingImage = false;
                return resolve([0, 0]);
            }

            this.image = new Image();
            this.image.crossOrigin = "anonymous";
            this.image.src = imageSrc;
            this.image.onload = () => {

                let canvas = null;
                let width = this.image.width;
                let height = this.image.height;
                if (this.image.height > 8000 || this.image.width > 8000) {
                    const scaleFactor = 8000 / Math.max(this.image.width, this.image.height);
                    width = Math.floor(this.image.width * scaleFactor);
                    height = Math.floor(this.image.height * scaleFactor);

                    canvas = document.createElement('canvas');
                    canvas.width = width;
                    canvas.height = height;

                    const ctx = canvas.getContext('2d');
                    ctx.drawImage(this.image, 0, 0, width, height);

                    this.image.width = width;
                    this.image.height = height;
                }

                this.pos.x = (this.w * 0.5) - (width * 0.5);
                this.pos.y = ((this.h - 28) * 0.5) - (height * 0.5);

                this.imgProgram = new Program(this.gl)
                    .add_vertex_shader(map_vert_shader)
                    .add_fragment_shader(map_frag_shader)
                    .build()
                    .build_uniforms(["u_resolution", "u_scale", "u_translation"])
                    .build_attributes(["a_position", "a_texCoord"])
                    .set_verticies(new Float32Array([
                        0, 0, 0.0, 0.0, // top-left
                        width, 0, 1.0, 0.0, // top-right
                        0, height, 0.0, 1.0, // bottom-left
                        width, height, 1.0, 1.0 // bottom-right
                    ]))
                    .set_indices(new Uint16Array([
                        0, 1, 2,  // First triangle
                        2, 1, 3   // Second triangle
                    ]))
                    .create_buffer("verticies")
                    .create_buffer("indices")
                    .create_texture()
                    .create_vao()
                    .create_fbo();

                this.gl.useProgram(this.imgProgram.get_program());

                this.gl.bindVertexArray(this.imgProgram.get_vao());

                this.gl.bindBuffer(this.gl.ARRAY_BUFFER, this.imgProgram.get_buffer("verticies"));
                this.gl.bufferData(this.gl.ARRAY_BUFFER, this.imgProgram.get_verticies(), this.gl.STATIC_DRAW);

                const stride = 4 * Float32Array.BYTES_PER_ELEMENT;
                this.gl.vertexAttribPointer(this.imgProgram.get_attribute("a_position"), 2, this.gl.FLOAT, false, stride, 0);
                this.gl.enableVertexAttribArray(this.imgProgram.get_attribute("a_position"));
                this.gl.vertexAttribPointer(this.imgProgram.get_attribute("a_texCoord"), 2, this.gl.FLOAT, false, stride, 2 * 4);
                this.gl.enableVertexAttribArray(this.imgProgram.get_attribute("a_texCoord"));

                this.gl.bindBuffer(this.gl.ELEMENT_ARRAY_BUFFER, this.imgProgram.get_buffer("indices"));
                this.gl.bufferData(this.gl.ELEMENT_ARRAY_BUFFER, this.imgProgram.get_indices(), this.gl.STATIC_DRAW);

                this.gl.activeTexture(this.gl.TEXTURE0);
                this.gl.bindTexture(this.gl.TEXTURE_2D, this.imgProgram.get_texture());
                this.gl.texImage2D(this.gl.TEXTURE_2D, 0, this.gl.RGBA, this.gl.RGBA, this.gl.UNSIGNED_BYTE, canvas || this.image);

                if (this.isPowerOfTwo(width) && this.isPowerOfTwo(height)) {
                    this.gl.texParameteri(this.gl.TEXTURE_2D, this.gl.TEXTURE_MIN_FILTER, this.gl.LINEAR_MIPMAP_NEAREST);
                    this.gl.texParameteri(this.gl.TEXTURE_2D, this.gl.TEXTURE_MAG_FILTER, this.gl.NEAREST);
                    this.gl.generateMipmap(this.gl.TEXTURE_2D);

                    const error = this.gl.getError();
                    if (error !== this.gl.NO_ERROR) {
                        this.gl.texParameteri(this.gl.TEXTURE_2D, this.gl.TEXTURE_WRAP_S, this.gl.CLAMP_TO_EDGE);
                        this.gl.texParameteri(this.gl.TEXTURE_2D, this.gl.TEXTURE_WRAP_T, this.gl.CLAMP_TO_EDGE);
                        this.gl.texParameteri(this.gl.TEXTURE_2D, this.gl.TEXTURE_MIN_FILTER, this.gl.NEAREST);
                        this.gl.texParameteri(this.gl.TEXTURE_2D, this.gl.TEXTURE_MAG_FILTER, this.gl.NEAREST);
                    }
                } else {
                    this.gl.texParameteri(this.gl.TEXTURE_2D, this.gl.TEXTURE_WRAP_S, this.gl.CLAMP_TO_EDGE);
                    this.gl.texParameteri(this.gl.TEXTURE_2D, this.gl.TEXTURE_WRAP_T, this.gl.CLAMP_TO_EDGE);
                    this.gl.texParameteri(this.gl.TEXTURE_2D, this.gl.TEXTURE_MIN_FILTER, this.gl.NEAREST);
                    this.gl.texParameteri(this.gl.TEXTURE_2D, this.gl.TEXTURE_MAG_FILTER, this.gl.NEAREST);
                }

                this.gl.bindVertexArray(null);
                this.gl.bindBuffer(this.gl.ARRAY_BUFFER, null);
                this.gl.bindTexture(this.gl.TEXTURE_2D, null);

                this.loadingImage = false;

                this.buildFogProgram();

                this.doMove = true;
                this.animationId = window.requestAnimationFrame(this.firstFrame.bind(this));
                return resolve([width, height]);
            };
        });
    }

    private forceClear() {
        this.gl.clearColor(0, 0, 0, 0);
        this.gl.clear(this.gl.COLOR_BUFFER_BIT);
        this.gl = null;
    }

    private firstFrame(ts) {
        this.time = ts;
        this.animationId = window.requestAnimationFrame(this.nextFrame.bind(this));
    }

    private nextFrame(ts) {
        const dt = (ts - this.time) * 0.001;
        this.time = ts;

        if (!this.updateFog && !this.updateGrid && !this.doMove) {
            this.animationId = window.requestAnimationFrame(this.nextFrame.bind(this));
            return;
        }

        this.gl.clearColor(0, 0, 0, 0);
        this.gl.clearDepth(1.0);

        this.gl.enable(this.gl.DEPTH_TEST);
        this.gl.enable(this.gl.BLEND);

        this.gl.depthFunc(this.gl.LEQUAL);
        this.gl.blendFunc(this.gl.SRC_ALPHA, this.gl.ONE_MINUS_SRC_ALPHA);

        this.gl.clear(this.gl.COLOR_BUFFER_BIT | this.gl.DEPTH_BUFFER_BIT);

        if (!this.image || this.loadingImage) return;

        this.drawImage();

        if (this.fogOfWar) {
            this.drawFog();
        }
        if (this.renderGrid) {
            this.drawGrid();
        }

        this.updateFog = false;
        this.updateGrid = false;
        this.doMove = false;
        this.animationId = window.requestAnimationFrame(this.nextFrame.bind(this));
    }

    private buildFogMask() {
        if (this.lastFogCount === this.fogOfWarShapes.length) return;

        this.gl.useProgram(this.maskProgram.get_program());
        this.gl.bindFramebuffer(this.gl.FRAMEBUFFER, this.maskProgram.get_fbo());
        this.gl.viewport(0, 0, this.image.width, this.image.height);
        this.gl.clearColor(0.0, 0.0, 0.0, 0.0);
        this.gl.clear(this.gl.COLOR_BUFFER_BIT);
        this.gl.bindTexture(this.gl.TEXTURE_2D, this.maskProgram.get_texture());
        this.gl.bindBuffer(this.gl.ARRAY_BUFFER, this.maskProgram.get_buffer("verticies"));
        // Mask
        let allVertices = [];
        for (let shape of this.fogOfWarShapes) {
            let vertices = [];

            switch(shape.type){
                case "rect":
                    {
                        const p0 = shape.points[0];
                        const p1 = shape.points[1];

                        // Compute min and max for x and y so we cover the rectangle regardless of order.
                        const xMin = Math.min(p0.x, p1.x);
                        const xMax = Math.max(p0.x, p1.x);
                        const yMin = Math.min(p0.y, p1.y);
                        const yMax = Math.max(p0.y, p1.y);

                        // Convert the four corners to clip space:
                        // We create a quad using two triangles (or a TRIANGLE_STRIP)
                        const bl = this.world_to_clip(xMin, yMin, this.image.width, this.image.height); // bottom-left
                        const tl = this.world_to_clip(xMin, yMax, this.image.width, this.image.height); // top-left
                        const br = this.world_to_clip(xMax, yMin, this.image.width, this.image.height); // bottom-right
                        const tr = this.world_to_clip(xMax, yMax, this.image.width, this.image.height); // top-right

                        vertices.push(
                            bl[0], bl[1], br[0], br[1], tr[0], tr[1],  // Triangle 1
                            bl[0], bl[1], tr[0], tr[1], tl[0], tl[1]   // Triangle 2
                        );
                    }
                    break;
                case "poly":
                    {
                        const flatVertices = [];
                        shape.points.forEach(p => {
                        const clip = this.world_to_clip(p.x, p.y, this.image.width, this.image.height);
                            flatVertices.push(clip[0], clip[1]);
                        });

                        const indices = earcut(flatVertices);
                        for (let i = 0; i < indices.length; i++) {
                            const idx = indices[i];
                            vertices.push(flatVertices[2 * idx], flatVertices[2 * idx + 1]);
                        }
                    }
                    break;
            }

            allVertices.push(...vertices);
        }

        this.gl.bufferData(this.gl.ARRAY_BUFFER, new Float32Array(allVertices), this.gl.STATIC_DRAW);

        this.gl.enableVertexAttribArray(this.maskProgram.get_attribute("a_position"));
        this.gl.vertexAttribPointer(
            this.maskProgram.get_attribute("a_position"),
            2,
            this.gl.FLOAT,
            false,
            0,
            0
        );
        this.gl.drawArrays(this.gl.TRIANGLES, 0, allVertices.length / 2);
        this.lastFogCount = this.fogOfWarShapes.length;

        // reset
        this.gl.bindTexture(this.gl.TEXTURE_2D, null);
        this.gl.bindBuffer(this.gl.ARRAY_BUFFER, null);
        this.gl.bindFramebuffer(this.gl.FRAMEBUFFER, null);
        this.gl.viewport(0, 0, this.w, this.h);
    }

    private drawFog() {
        if (this.maskProgram === undefined || this.fogProgram === undefined) {
            throw new Error("Render error: missing fog or mask program.");
        }

        this.buildFogMask();

        this.gl.useProgram(this.fogProgram.get_program());
        this.gl.uniform2f(this.fogProgram.get_uniform("u_resolution"), this.w, this.h);
        this.gl.uniform2f(this.fogProgram.get_uniform("u_translation"), this.pos.x, this.pos.y);
        this.gl.uniform2f(this.fogProgram.get_uniform("u_scale"), this.tabletop.zoom, this.tabletop.zoom);
        let color = "#fafafaFF"
        if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
            color = "#09090bFF"
        }
        const [r,g,b,a] = this.hex_to_rgbaf(color);
        this.gl.uniform4f(this.fogProgram.get_uniform("u_color"), r, g, b, a);
        this.gl.uniform1i(this.fogProgram.get_uniform("u_isGM"), room.isGM ? 1 : 0);

        this.gl.activeTexture(this.gl.TEXTURE0);
        this.gl.bindTexture(this.gl.TEXTURE_2D, this.imgProgram.get_texture());
        this.gl.uniform1i(this.fogProgram.get_uniform("u_image"), 0);

        this.gl.activeTexture(this.gl.TEXTURE1);
        this.gl.bindTexture(this.gl.TEXTURE_2D, this.maskProgram.get_texture());
        this.gl.uniform1i(this.fogProgram.get_uniform("u_mask"), 1);

        this.gl.bindVertexArray(this.fogProgram.get_vao());
        this.gl.drawElements(this.gl.TRIANGLES, this.fogProgram.get_indices().length, this.gl.UNSIGNED_SHORT, 0);

        this.gl.bindVertexArray(null);
        this.gl.bindTexture(this.gl.TEXTURE_2D, null);
    }

    private drawGrid() {
        if (this.gridProgram === undefined) {
            throw new Error("Render error: missing grid program.");
        }
        this.gl.useProgram(this.gridProgram.get_program());
        this.gl.bindVertexArray(this.gridProgram.get_vao());

        this.gl.uniform2f(this.gridProgram.get_uniform("u_resolution"), this.w, this.h);
        this.gl.uniform2f(this.gridProgram.get_uniform("u_origin"), this.pos.x, this.pos.y);
        this.gl.uniform2f(this.gridProgram.get_uniform("u_offset"), this.gridOffset[0], this.gridOffset[1]);
        this.gl.uniform1f(this.gridProgram.get_uniform("u_spacing"), this.gridSize);
        this.gl.uniform1f(this.gridProgram.get_uniform("u_scale"), this.tabletop.zoom);
        this.gl.uniform4f(this.gridProgram.get_uniform("u_color"), this.gridColor[0], this.gridColor[1], this.gridColor[2], this.gridColor[3]);

        this.gl.drawArrays(this.gl.TRIANGLE_STRIP, 0, 4);

        this.gl.bindVertexArray(null);
    }

    private drawImage() {
        if (this.imgProgram === undefined) {
            throw new Error("Render error: missing image program.");
        }
        this.gl.useProgram(this.imgProgram.get_program());
        this.gl.bindVertexArray(this.imgProgram.get_vao());

        this.gl.bindTexture(this.gl.TEXTURE_2D, this.imgProgram.get_texture());
        this.gl.uniform2f(this.imgProgram.get_uniform("u_resolution"), this.w, this.h);
        this.gl.uniform2f(this.imgProgram.get_uniform("u_translation"), this.pos.x, this.pos.y);
        this.gl.uniform2f(this.imgProgram.get_uniform("u_scale"), this.tabletop.zoom, this.tabletop.zoom);

        this.gl.drawElements(this.gl.TRIANGLES, this.imgProgram.get_indices().length, this.gl.UNSIGNED_SHORT, 0);

        this.gl.bindVertexArray(null);
        this.gl.bindTexture(this.gl.TEXTURE_2D, null);
    }

    private hex_to_rgbaf(hex: string): Array<number> {
        if (hex.indexOf("#") == 0) {
            hex = hex.substring(1, 9);
        }
        if (hex.length < 8) {
            throw new Error("Malformed HEX color provided.");
        }
        return [
            +Math.max(0, Math.min(1, parseInt(hex.substring(0, 2), 16) / 255)).toFixed(1),
            +Math.max(0, Math.min(1, parseInt(hex.substring(2, 4), 16) / 255)).toFixed(1),
            +Math.max(0, Math.min(1, parseInt(hex.substring(4, 6), 16) / 255)).toFixed(1),
            +Math.max(0, Math.min(1, parseInt(hex.substring(6, 8), 16) / 255)).toFixed(1),
        ];
    }

    private hex_to_rgbai(hex: string): Array<number> {
        if (hex.indexOf("#") == 0) {
            hex = hex.substring(1, 9);
        }
        if (hex.length < 8) {
            throw new Error("Malformed HEX color provided.");
        }
        return [
            Math.max(0, Math.min(255, parseInt(hex.substring(0, 2), 16))),
            Math.max(0, Math.min(255, parseInt(hex.substring(2, 4), 16))),
            Math.max(0, Math.min(255, parseInt(hex.substring(4, 6), 16))),
            Math.max(0, Math.min(255, parseInt(hex.substring(6, 8), 16))),
        ];
    }

    private world_to_clip(x, y, w, h) {
        const clipX = (x / w) * 2 - 1;
        const clipY = (y / h) * 2 - 1;
        return [clipX, clipY];
    }
    
    private isPowerOfTwo(value) {
        return (value & (value - 1)) === 0;  // Check if the value is a power of 2
    }
}
env.bind("table-canvas", TableCanvas);
