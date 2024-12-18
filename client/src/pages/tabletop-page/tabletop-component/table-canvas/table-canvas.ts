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

interface ITableCanvas { }
export default class TableCanvas extends SuperComponent<ITableCanvas>{
    private canvas: HTMLCanvasElement;
    private fogCanvas: HTMLCanvasElement;
    private fogctx: CanvasRenderingContext2D;
    private imgctx: CanvasRenderingContext2D;
    private time: number;
    private renderGrid: boolean;
    private gridSize: number;
    private fogOfWar: boolean;
    private ticket: string;
    private w: number;
    private h: number;
    private fogOfWarShapes: Array<FogOfWarShape>;
    private tabletop: TabeltopComponent;
    private image: HTMLImageElement;

    constructor() {
        super();
        this.w = 0;
        this.h = 0;
        this.canvas = document.createElement("canvas") as HTMLCanvasElement;
        this.fogCanvas = document.createElement("canvas") as HTMLCanvasElement;
        this.fogctx = this.fogCanvas.getContext("2d");
        this.imgctx = this.canvas.getContext("2d");
        this.tabletop = document.querySelector("tabletop-component");
        this.time = performance.now();
        this.renderGrid = false;
        this.gridSize = 32;
        this.fogOfWar = false;
        this.fogOfWarShapes = [];
        this.image = null;
        subscribe("socket", this.inbox.bind(this));
        subscribe("fog", this.fogInbox.bind(this));
    }

    override async connected() {
        await env.css(["table-canvas"]);
        this.appendChild(this.canvas);
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
            case "room:tabletop:fog:sync":
                this.fogOfWar = data.fogOfWar;
                this.fogOfWarShapes = data.fogOfWarShapes;
                this.render();
                break;
            case "room:tabletop:clear":
                this.fogOfWarShapes = [];
                this.render();
                break;
            case "room:tabletop:map:update":
                this.renderGrid = data.renderGrid;
                this.gridSize = data.cellSize;
                this.fogOfWar = data.prefillFog;
                this.render();
                break;
            default:
                break;
        }
    }

    private sync(shape:FogOfWarShape) {
        send("room:tabletop:fog:sync", shape);
    }

    private revealShapes() {
        this.fogctx.globalCompositeOperation = "destination-out";
        this.fogctx.globalAlpha = 1.0;
        this.fogctx.fillStyle = "white";
        for (let i = 0; i < this.fogOfWarShapes.length; i++) {
            switch (this.fogOfWarShapes[i].type){
                case "poly":
                    this.fogctx.beginPath();
                    this.fogctx.moveTo(this.fogOfWarShapes[i].points[0].x, this.fogOfWarShapes[i].points[0].y);
                    for (let p = 1; p < this.fogOfWarShapes[i].points.length; p++){
                        this.fogctx.lineTo(this.fogOfWarShapes[i].points[p].x, this.fogOfWarShapes[i].points[p].y);
                    }
                    this.fogctx.closePath();
                    this.fogctx.fill();
                    break;
                case "rect":
                    const width = this.fogOfWarShapes[i].points[1].x - this.fogOfWarShapes[i].points[0].x;
                    const height = this.fogOfWarShapes[i].points[1].y - this.fogOfWarShapes[i].points[0].y
                    this.fogctx.rect(this.fogOfWarShapes[i].points[0].x, this.fogOfWarShapes[i].points[0].y, width, height);
                    this.fogctx.fill();
                    break;
            }
        }
        this.fogctx.globalCompositeOperation = "source-over";
    }

    private renderFogOfWar() {
        if (!this.fogOfWar) return;
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
    }

    private renderGridLines(){
        if (!this.renderGrid) return;
        const columns = Math.ceil(this.w / this.gridSize);
        const rows = Math.ceil(this.h / this.gridSize);

        this.imgctx.strokeStyle = "rgb(0,0,0)";
        for (let i = 0; i < columns; i++) {
            const x = i * this.gridSize;
            this.imgctx.beginPath();
            this.imgctx.moveTo(x, 0);
            this.imgctx.lineTo(x, this.h);
            this.imgctx.stroke();
        }
        for (let i = 0; i < rows; i++) {
            const y = i * this.gridSize;
            this.imgctx.beginPath();
            this.imgctx.moveTo(0, y);
            this.imgctx.lineTo(this.w, y);
            this.imgctx.stroke();
        }
    }

    public load(image: HTMLImageElement) {
        if (!image) {
            this.w = 0;
            this.h = 0;
            this.image = null;

            this.canvas.width = this.w;
            this.canvas.height = this.h;
            this.canvas.style.width = `0px`;
            this.canvas.style.height = `0px`;

            this.fogCanvas.width = this.w;
            this.fogCanvas.height = this.h;
            this.fogCanvas.style.width = `0px`;
            this.fogCanvas.style.height = `0px`;
        } else {
            this.w = image.width;
            this.h = image.height;
            this.image = image;

            this.canvas.width = this.w;
            this.canvas.height = this.h;
            this.canvas.style.width = `${image.width}px`;
            this.canvas.style.height = `${image.height}px`;

            this.fogCanvas.width = this.w;
            this.fogCanvas.height = this.h;
            this.fogCanvas.style.width = `${image.width}px`;
            this.fogCanvas.style.height = `${image.height}px`;
        }
        this.render();
    }

    override render(): void {
        this.fogctx.clearRect(0, 0, this.w, this.h);
        this.imgctx.clearRect(0, 0, this.w, this.h);

        if (!this.image) return;

        // Always draw map first
        this.imgctx.drawImage(this.image, 0, 0, this.w, this.h);

        // Other
        this.renderGridLines();

        // Always draw fog last
        this.renderFogOfWar();
        this.imgctx.drawImage(this.fogCanvas, 0, 0);
    }
}
env.bind("table-canvas", TableCanvas);
