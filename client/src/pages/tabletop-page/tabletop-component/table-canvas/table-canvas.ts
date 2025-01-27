import SuperComponent from "@codewithkyle/supercomponent";
import env from "~brixi/controllers/env";
import { subscribe } from "@codewithkyle/pubsub";
import room from "room";
import TabeltopComponent from "../tabletop-component";
import { send } from "~controllers/ws";

type Point = {
    x: number,
    y: number,
}
type FogOfWarShape = {
    type: "poly" | "rect",
    points: Array<Point>,
}

const vsSource = `#version 300 es
    in vec2 a_position;
    in vec2 a_texCoord;
    out vec2 v_texCoord;

    uniform vec2 u_resolution;
    uniform vec2 u_translation;

    void main() {
        // apply translation
        vec2 translatedPosition = a_position + u_translation;

        // convert pixel coord to normalized device coord
        vec2 zeroToOne = translatedPosition / u_resolution;
        vec2 zeroToTwo = zeroToOne * 2.0;
        vec2 clipSpace = zeroToTwo - 1.0;

        gl_Position = vec4(clipSpace * vec2(1, -1), 0, 1); // flip y-axis
        v_texCoord = a_texCoord; // pass tex coords
    }
`;

const fsSource = `#version 300 es
    precision mediump float;

    in vec2 v_texCoord;
    uniform sampler2D u_texture;
    out vec4 outColor;

    void main() {
        outColor = texture(u_texture, v_texCoord);
    }
`;

interface ITableCanvas { }
export default class TableCanvas extends SuperComponent<ITableCanvas>{
    private canvas: HTMLCanvasElement;
    private fogCanvas: HTMLCanvasElement;
    private gridCanvas: HTMLCanvasElement;
    private fogctx: CanvasRenderingContext2D;
    private imgctx: WebGL2RenderingContext;
    private gridctx: CanvasRenderingContext2D;
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
    private indices: Uint16Array;
    private program: WebGLProgram;
    private time: number;
    private pos: {
        x: number,
        y: number,
    };
    private lastMousePos: {
        x: number,
        y: number,
    };
    private doMove: boolean;

    constructor() {
        super();
        this.w = 0;
        this.h = 0;
        this.pos = {
            x: 0,
            y: 0,
        };
        this.lastMousePos = undefined;
        this.doMove = false;
        this.canvas = document.createElement("canvas") as HTMLCanvasElement;
        this.canvas.width = window.innerWidth;
        this.canvas.height = window.innerHeight;
        //this.fogCanvas = document.createElement("canvas") as HTMLCanvasElement;
        //this.gridCanvas = document.createElement("canvas") as HTMLCanvasElement;

        //this.fogctx = this.fogCanvas.getContext("2d");
        //this.fogctx.imageSmoothingEnabled = false;

        this.imgctx = this.canvas.getContext("webgl2");

        //this.gridctx = this.gridCanvas.getContext("2d");
        //this.gridctx.imageSmoothingEnabled = false;

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
    }

    override async connected() {
        await env.css(["table-canvas"]);
        this.appendChild(this.canvas);
        window.addEventListener("resize", this.debounce(()=>{
            this.canvas.width = window.innerWidth;
            this.canvas.height = window.innerHeight;
            this.imgctx.viewport(0, 0, this.canvas.width, this.canvas.height);
        }, 150));
        window.addEventListener("mousemove", this.onMove, { passive: true });
        window.addEventListener("mousedown", () => {
            this.doMove = true;
        });
        window.addEventListener("mouseup", () => {
            this.doMove = false;
        });
    }

    private onMove:EventListener = (e:MouseEvent) => {
        const   x = e.clientX,
                y = e.clientY;
        if (this.lastMousePos === undefined) {
            this.lastMousePos = {
                x,
                y,
            };
            return;
        }
        const deltaX = this.lastMousePos.x - x;
        const deltaY = this.lastMousePos.y - y;
        this.lastMousePos.x = x;
        this.lastMousePos.y = y;
        if (this.doMove) {
            this.pos.x -= deltaX;
            this.pos.y -= deltaY;
        }
    }

    public convertViewportToTabletopPosition(clientX: number, clientY: number): Array<number> {
        const canvas = this.getBoundingClientRect();
        const x = Math.round(clientX - canvas.left) / this.tabletop.zoom;
        const y = Math.round(clientY - canvas.top) / this.tabletop.zoom;
        return [x, y];
    }

    private fogInbox({ type, points }) {
        const convertedPoints = [];
        for (let i = 0; i < points.length; i++){
            const [x, y] = this.convertViewportToTabletopPosition(points[i].x, points[i].y);
            convertedPoints.push({ x, y });
        }
        switch (type) {
            case "rect":
                const newRect:FogOfWarShape = {
                    type: "rect",
                    points: convertedPoints,
                };
                this.fogOfWarShapes.push(newRect);
                this.sync(newRect);
                break;
            case "poly":
                const newPoly:FogOfWarShape = {
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

    private sync(shape:FogOfWarShape) {
        send("room:tabletop:fog:add", shape);
    }

    private revealShapes() {
        try {
            this.fogctx.globalCompositeOperation = "destination-out";
            this.fogctx.globalAlpha = 1.0;
            this.fogctx.fillStyle = "white";
            for (let i = 0; i < this.fogOfWarShapes.length; i++) {
                switch (this.fogOfWarShapes[i].type){
                    case "poly":
                        try {
                            this.fogctx.beginPath();
                            this.fogctx.moveTo(this.fogOfWarShapes[i].points[0].x, this.fogOfWarShapes[i].points[0].y);
                            for (let p = 1; p < this.fogOfWarShapes[i].points.length; p++){
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

    private renderFogOfWar() {
        try {
            if (!this.fogOfWar || !this.updateFog) return;
            if (room.isGM) {
                this.fogctx.globalAlpha = 0.6;
            }
            let color = "#fafafa"
            if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
                color = "#09090b"
            }
            this.fogctx.fillStyle = color;
            this.fogctx.fillRect(0, 0, this.w, this.h);
            this.revealShapes();
        } catch (e) {
            console.error("Fog of war render error:", e);
        }
        this.updateFog = false;
    }

    private renderGridLines(){
        try {
            if (!this.renderGrid || !this.updateGrid) return;
            const columns = Math.ceil(this.w / this.gridSize);
            const rows = Math.ceil(this.h / this.gridSize);

            this.gridctx.strokeStyle = "rgb(0,0,0)";
            for (let i = 0; i < columns; i++) {
                const x = i * this.gridSize;
                this.gridctx.beginPath();
                this.gridctx.moveTo(x, 0);
                this.gridctx.lineTo(x, this.h);
                this.gridctx.stroke();
            }
            for (let i = 0; i < rows; i++) {
                const y = i * this.gridSize;
                this.gridctx.beginPath();
                this.gridctx.moveTo(0, y);
                this.gridctx.lineTo(this.w, y);
                this.gridctx.stroke();
            }
        } catch (e) {
            console.error("Grid line render error:", e);
        }
        this.updateGrid = false;
    }

    private compileShader(gl, source, type) {
        const shader = gl.createShader(type);
        gl.shaderSource(shader, source);
        gl.compileShader(shader);

        if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
            console.error("Error compiling shader:", gl.getShaderInfoLog(shader));
            gl.deleteShader(shader);
            return null;
        }

        return shader;
    }

    private createProgram(gl: WebGL2RenderingContext, vertexShader, fragmentShader) {
        const program = gl.createProgram();
        gl.attachShader(program, vertexShader);
        gl.attachShader(program, fragmentShader);
        gl.linkProgram(program);

        if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
            console.error("Error linking program:", gl.getProgramInfoLog(program));
            gl.deleteProgram(program);
            return null;
        }

        return program;
    }

    public load(imageSrc: string) {
        this.w = window.innerWidth;
        this.h = window.innerHeight;

        const vertexShader = this.compileShader(this.imgctx, vsSource, this.imgctx.VERTEX_SHADER);
        const fragmentShader = this.compileShader(this.imgctx, fsSource, this.imgctx.FRAGMENT_SHADER);
        this.program = this.createProgram(this.imgctx, vertexShader, fragmentShader);

        this.imgctx.useProgram(this.program);

        this.image = new Image();
        this.image.crossOrigin = "anonymous";
        this.image.src = imageSrc;
        this.image.onload = () => {
            const positions = new Float32Array([
                0, 0, 0.0, 0.0, // top-left
                this.image.width, 0, 1.0, 0.0, // top-right
                0, this.image.height, 0.0, 1.0, // bottom-left
                this.image.width, this.image.height, 1.0, 1.0 // bottom-right
            ]);
            const posBuffer = this.imgctx.createBuffer();
            this.imgctx.bindBuffer(this.imgctx.ARRAY_BUFFER, posBuffer);
            this.imgctx.bufferData(this.imgctx.ARRAY_BUFFER, positions, this.imgctx.STATIC_DRAW);

            this.indices = new Uint16Array([
                0, 1, 2,  // First triangle
                2, 1, 3   // Second triangle
            ]);
            const indexBuffer = this.imgctx.createBuffer();
            this.imgctx.bindBuffer(this.imgctx.ELEMENT_ARRAY_BUFFER, indexBuffer);
            this.imgctx.bufferData(this.imgctx.ELEMENT_ARRAY_BUFFER, this.indices, this.imgctx.STATIC_DRAW);

            // Define how to read the `positions` buffer for each attribute
            const stride = 4 * Float32Array.BYTES_PER_ELEMENT; // 4 values per vertex (x, y, u, v)
            const aPositionLoc = this.imgctx.getAttribLocation(this.program, "a_position");
            const aTexCoordLoc = this.imgctx.getAttribLocation(this.program, "a_texCoord");
            this.imgctx.vertexAttribPointer(aPositionLoc, 2, this.imgctx.FLOAT, false, stride, 0);        // x, y
            this.imgctx.enableVertexAttribArray(aPositionLoc);
            this.imgctx.vertexAttribPointer(aTexCoordLoc, 2, this.imgctx.FLOAT, false, stride, 2 * 4);   // u, v
            this.imgctx.enableVertexAttribArray(aTexCoordLoc);

            const texture = this.imgctx.createTexture();
            this.imgctx.bindTexture(this.imgctx.TEXTURE_2D, texture);
            this.imgctx.texImage2D(this.imgctx.TEXTURE_2D, 0, this.imgctx.RGBA, this.imgctx.RGBA, this.imgctx.UNSIGNED_BYTE, this.image);
            this.imgctx.texParameteri(this.imgctx.TEXTURE_2D, this.imgctx.TEXTURE_WRAP_S, this.imgctx.CLAMP_TO_EDGE);
            this.imgctx.texParameteri(this.imgctx.TEXTURE_2D, this.imgctx.TEXTURE_WRAP_T, this.imgctx.CLAMP_TO_EDGE);
            this.imgctx.texParameteri(this.imgctx.TEXTURE_2D, this.imgctx.TEXTURE_MIN_FILTER, this.imgctx.LINEAR);
            this.imgctx.texParameteri(this.imgctx.TEXTURE_2D, this.imgctx.TEXTURE_MAG_FILTER, this.imgctx.LINEAR);

            window.requestAnimationFrame(this.firstFrame.bind(this));
        };
    }

    private firstFrame(ts) {
        this.time = ts;
        window.requestAnimationFrame(this.nextFrame.bind(this));
    }

    private nextFrame(ts) {
        const dt = (ts - this.time) * 0.001;
        this.time = ts;

        this.imgctx.clearColor(0,0,0,0);
        this.imgctx.clear(this.imgctx.COLOR_BUFFER_BIT);

        if (!this.image) return;

        const uResolutionLoc = this.imgctx.getUniformLocation(this.program, "u_resolution");
        this.imgctx.uniform2f(uResolutionLoc, this.canvas.width, this.canvas.height);

        const uTranslationLoc = this.imgctx.getUniformLocation(this.program, "u_translation");
        this.imgctx.uniform2f(uTranslationLoc, this.pos.x, this.pos.y);

        this.imgctx.drawElements(this.imgctx.TRIANGLES, this.indices.length, this.imgctx.UNSIGNED_SHORT, 0);

        window.requestAnimationFrame(this.nextFrame.bind(this));
    }
}
env.bind("table-canvas", TableCanvas);
