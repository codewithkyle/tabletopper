import SuperComponent from "@codewithkyle/supercomponent";
import env from "~brixi/controllers/env";
import { subscribe } from "@codewithkyle/pubsub";
import room from "room";
import TabeltopComponent from "../tabletop-component";
import { send } from "~controllers/ws";
import { Program } from "./program";
import { grid_frag_shader, grid_vert_shader, map_frag_shader, map_vert_shader } from "./shaders";

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
    private time: number;
    private pos: {
        x: number,
        y: number,
    };
    private doMove: boolean;

    constructor() {
        super();
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
        const canvas = this.getBoundingClientRect();
        const x = Math.round(clientX - canvas.left) / this.tabletop.zoom;
        const y = Math.round(clientY - canvas.top) / this.tabletop.zoom;
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
                this.updateFog = true;
                break;
            case "room:tabletop:clear":
                this.fogOfWarShapes = [];
                this.updateFog = true;
                break;
            case "room:tabletop:map:update":
                this.renderGrid = data.renderGrid;
                this.gridSize = data.cellSize;
                this.fogOfWar = data.prefillFog;
                this.updateGrid = true;
                this.updateFog = true;
                break;
            default:
                break;
        }
    }

    private sync(shape: FogOfWarShape) {
        send("room:tabletop:fog:add", shape);
    }

    private revealShapes() {
        try {
            this.fogctx.globalCompositeOperation = "destination-out";
            this.fogctx.globalAlpha = 1.0;
            this.fogctx.fillStyle = "white";
            for (let i = 0; i < this.fogOfWarShapes.length; i++) {
                switch (this.fogOfWarShapes[i].type) {
                    case "poly":
                        try {
                            this.fogctx.beginPath();
                            this.fogctx.moveTo(this.fogOfWarShapes[i].points[0].x, this.fogOfWarShapes[i].points[0].y);
                            for (let p = 1; p < this.fogOfWarShapes[i].points.length; p++) {
                                this.fogctx.lineTo(this.fogOfWarShapes[i].points[p].x, this.fogOfWarShapes[i].points[p].y);
                            }
                            this.fogctx.closePath();
                            this.fogctx.fill();
                        } catch (e) {
                            console.error("Reveal poly error:", e);
                        }
                        break;
                    case "rect":
                        try {
                            const width = this.fogOfWarShapes[i].points[1].x - this.fogOfWarShapes[i].points[0].x;
                            const height = this.fogOfWarShapes[i].points[1].y - this.fogOfWarShapes[i].points[0].y
                            this.fogctx.rect(this.fogOfWarShapes[i].points[0].x, this.fogOfWarShapes[i].points[0].y, width, height);
                            this.fogctx.fill();
                        } catch (e) {
                            console.error("Reveal rect error:", e);
                        }
                        break;
                    default:
                        console.error("How did you get here?", this.fogOfWarShapes[i]);
                        break;
                }
            }
            this.fogctx.globalCompositeOperation = "source-over";
        } catch (e) {
            console.error("Reveal shapes render error:", e);
        }
    }

    private buildGridLinesProgram() {
        this.gridProgram = new Program(this.gl)
            .add_vertex_shader(grid_vert_shader)
            .add_fragment_shader(grid_frag_shader)
            .build()
            .build_uniforms(["u_resolution", "u_spacing", "u_origin", "u_color", "u_scale"])
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
            if (imageSrc == null) {
                this.imgProgram = undefined;
                return resolve([0, 0]);
            }

            this.image = new Image();
            this.image.crossOrigin = "anonymous";
            this.image.src = imageSrc;
            this.image.onload = () => {
                this.pos.x = (this.w * 0.5) - (this.image.width * 0.5);
                this.pos.y = ((this.h - 28) * 0.5) - (this.image.height * 0.5);

                this.imgProgram = new Program(this.gl)
                    .add_vertex_shader(map_vert_shader)
                    .add_fragment_shader(map_frag_shader)
                    .build()
                    .build_uniforms(["u_resolution", "u_scale", "u_translation"])
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
                    .create_texture()
                    .create_vao();

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

                this.gl.bindTexture(this.gl.TEXTURE_2D, this.imgProgram.get_texture());
                this.gl.texImage2D(this.gl.TEXTURE_2D, 0, this.gl.RGBA, this.gl.RGBA, this.gl.UNSIGNED_BYTE, this.image);
                this.gl.texParameteri(this.gl.TEXTURE_2D, this.gl.TEXTURE_MIN_FILTER, this.gl.NEAREST);
                this.gl.texParameteri(this.gl.TEXTURE_2D, this.gl.TEXTURE_MAG_FILTER, this.gl.NEAREST);
                this.gl.generateMipmap(this.gl.TEXTURE_2D);

                this.doMove = true;

                this.gl.bindVertexArray(null);
                window.requestAnimationFrame(this.firstFrame.bind(this));
                return resolve([this.image.width, this.image.height]);
            };
        });
    }

    private firstFrame(ts) {
        this.time = ts;
        window.requestAnimationFrame(this.nextFrame.bind(this));
    }

    private nextFrame(ts) {
        const dt = (ts - this.time) * 0.001;
        this.time = ts;

        if (!this.updateFog && !this.updateGrid && !this.doMove) {
            return window.requestAnimationFrame(this.nextFrame.bind(this));
        }

        this.gl.clearColor(0, 0, 0, 0);
        this.gl.clearDepth(1.0);

        this.gl.enable(this.gl.DEPTH_TEST);
        this.gl.enable(this.gl.BLEND);

        this.gl.depthFunc(this.gl.LEQUAL);
        this.gl.blendFunc(this.gl.SRC_ALPHA, this.gl.ONE_MINUS_SRC_ALPHA);

        this.gl.clear(this.gl.COLOR_BUFFER_BIT | this.gl.DEPTH_BUFFER_BIT);

        if (!this.image) return;

        this.drawImage();

        if (this.renderGrid) {
            this.drawGrid();
        }

        this.updateFog = false;
        this.updateGrid = false;
        this.doMove = false;
        window.requestAnimationFrame(this.nextFrame.bind(this));
    }

    private drawGrid() {
        if (this.gridProgram === undefined) {
            throw new Error("Render error: missing grid program.");
        }
        this.gl.useProgram(this.gridProgram.get_program());
        this.gl.bindVertexArray(this.gridProgram.get_vao());

        this.gl.uniform2f(this.gridProgram.get_uniform("u_resolution"), this.w, this.h);
        this.gl.uniform2f(this.gridProgram.get_uniform("u_origin"), this.pos.x, this.pos.y);
        this.gl.uniform1f(this.gridProgram.get_uniform("u_spacing"), this.gridSize);
        this.gl.uniform1f(this.gridProgram.get_uniform("u_scale"), this.tabletop.zoom);
        const [r,g,b,a] = this.hex_to_rgbaf("#FFFFFFFF"); // temp
        this.gl.uniform4f(this.gridProgram.get_uniform("u_color"), r,g,b,a);

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
}
env.bind("table-canvas", TableCanvas);
