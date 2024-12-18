import SuperComponent from "@codewithkyle/supercomponent";
import env from "~brixi/controllers/env";
import { subscribe, unsubscribe } from "@codewithkyle/pubsub";
import type { Image } from "~types/app";
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

interface IFogCanvas { }
export default class FogCanvas extends SuperComponent<IFogCanvas>{
    private canvas: HTMLCanvasElement;
    private fogCanvas: HTMLCanvasElement;
    private fogctx: CanvasRenderingContext2D;
    private imgctx: CanvasRenderingContext2D;
    private time: number;
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
        this.time = 0;
        this.gridSize = 32;
        this.fogOfWar = false;
        this.fogOfWarShapes = [];
        this.image = null;
        subscribe("socket", this.inbox.bind(this));
        subscribe("fog", this.fogInbox.bind(this));
    }

    override async connected() {
        await env.css(["fog-canvas"]);
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
                this.renderFogOfWar();
                break;
            case "room:tabletop:clear":
                this.fogOfWarShapes = [];
                this.fogOfWar = false;
                this.renderFogOfWar();
                break;
            case "room:tabletop:map:update":
                const prevGridSize = this.gridSize;
                this.gridSize = data.cellSize;
                this.fogOfWar = data.prefillFog;
                this.renderFogOfWar();
                if (prevGridSize != this.gridSize) this.load();
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
        this.fogctx.clearRect(0, 0, this.w, this.h);
        this.imgctx.clearRect(0, 0, this.w, this.h);
        if (!this.image) return;
        this.imgctx.drawImage(this.image, 0, 0, this.w, this.h);
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
        this.imgctx.drawImage(this.fogCanvas, 0, 0);
    }

    public load() {}

    // @ts-ignore
    override render(image: HTMLImageElement): void {
        if (!image) {
            this.w = 0;
            this.h = 0;
            this.canvas.width = this.w;
            this.canvas.height = this.h;
            this.canvas.style.width = `0px`;
            this.canvas.style.height = `0px`;
            this.fogCanvas.width = this.w;
            this.fogCanvas.height = this.h;
            this.fogCanvas.style.width = `0px`;
            this.fogCanvas.style.height = `0px`;
            this.image = null;
        } else {
            this.w = image.width;
            this.h = image.height;
            this.canvas.width = this.w;
            this.canvas.height = this.h;
            this.canvas.style.width = `${image.width}px`;
            this.canvas.style.height = `${image.height}px`;
            this.fogCanvas.width = this.w;
            this.fogCanvas.height = this.h;
            this.fogCanvas.style.width = `${image.width}px`;
            this.fogCanvas.style.height = `${image.height}px`;
            this.image = image;
        }
        this.renderFogOfWar();
    }
}
env.bind("fog-canvas", FogCanvas);
