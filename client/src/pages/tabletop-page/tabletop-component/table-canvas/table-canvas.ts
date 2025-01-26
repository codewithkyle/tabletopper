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
    private gridCanvas: HTMLCanvasElement;
    private fogctx: CanvasRenderingContext2D;
    private imgctx: CanvasRenderingContext2D;
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

    constructor() {
        super();
        this.w = 0;
        this.h = 0;
        this.canvas = document.createElement("canvas") as HTMLCanvasElement;
        this.fogCanvas = document.createElement("canvas") as HTMLCanvasElement;
        this.gridCanvas = document.createElement("canvas") as HTMLCanvasElement;
        this.fogctx = this.fogCanvas.getContext("2d");
        this.fogctx.imageSmoothingEnabled = false;
        this.imgctx = this.canvas.getContext("2d");
        this.imgctx.imageSmoothingEnabled = false;
        this.gridctx = this.gridCanvas.getContext("2d");
        this.gridctx.imageSmoothingEnabled = false;
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
                this.render();
                break;
            case "room:tabletop:fog:sync":
                this.fogOfWar = data.fogOfWar;
                this.fogOfWarShapes = data.fogOfWarShapes;
                this.updateFog = true;
                this.render();
                break;
            case "room:tabletop:clear":
                this.fogOfWarShapes = [];
                this.updateFog = true;
                this.render();
                break;
            case "room:tabletop:map:update":
                this.renderGrid = data.renderGrid;
                this.gridSize = data.cellSize;
                this.fogOfWar = data.prefillFog;
                this.updateGrid = true;
                this.updateFog = true;
                this.render();
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

    public load(image: HTMLImageElement) {
        if (!image) {
            this.w = 0;
            this.h = 0;
            this.image = null;
            this.updateGrid = true;
            this.updateFog = true;

            this.canvas.width = this.w;
            this.canvas.height = this.h;
            this.canvas.style.width = `0px`;
            this.canvas.style.height = `0px`;

            this.fogCanvas.width = this.w;
            this.fogCanvas.height = this.h;
            this.fogCanvas.style.width = `0px`;
            this.fogCanvas.style.height = `0px`;

            this.gridCanvas.width = this.w;
            this.gridCanvas.height = this.h;
            this.gridCanvas.style.width = `0px`;
            this.gridCanvas.style.height = `0px`;
        } else {
            this.w = image.width;
            this.h = image.height;
            this.image = image;
            this.updateGrid = true;
            this.updateFog = true;

            this.canvas.width = this.w;
            this.canvas.height = this.h;
            this.canvas.style.width = `${image.width}px`;
            this.canvas.style.height = `${image.height}px`;

            this.fogCanvas.width = this.w;
            this.fogCanvas.height = this.h;
            this.fogCanvas.style.width = `${image.width}px`;
            this.fogCanvas.style.height = `${image.height}px`;

            this.gridCanvas.width = this.w;
            this.gridCanvas.height = this.h;
            this.gridCanvas.style.width = `${image.width}px`;
            this.gridCanvas.style.height = `${image.height}px`;
        }
        this.render();
    }

    override render(): void {
        try {
            this.imgctx.clearRect(0, 0, this.w, this.h);
            if (this.updateFog) this.fogctx.clearRect(0, 0, this.w, this.h);
            if (this.updateGrid) this.gridctx.clearRect(0, 0, this.w, this.h);
            
            if (!this.image) return;

            // Other
            this.renderGridLines();
            this.renderFogOfWar();

            // Always draw map first
            this.imgctx.drawImage(
                this.image,
                0, 0, this.w, this.h
            );
           
            if (this.renderGridLines) {
                this.imgctx.drawImage(
                    this.gridCanvas,
                    0, 0,
                    this.w, this.h
                );
            }

            // Always draw fog last
            if (this.renderFogOfWar) {
                this.imgctx.drawImage(
                    this.fogCanvas,
                    0, 0,
                    this.w, this.h
                );
            }
        } catch (e) {
            console.error("Render error:", e);
            console.groupCollapsed();
            console.log("Update fog", this.updateFog);
            console.log("Update grid", this.updateGrid);
            console.log("Image", this.image);
            console.log("Fog shapes", this.fogOfWarShapes);
            console.log("Fog ctx", this.fogctx);
            console.log("Grid ctx", this.gridctx);
            console.groupEnd();
        }
    }
}
env.bind("table-canvas", TableCanvas);
