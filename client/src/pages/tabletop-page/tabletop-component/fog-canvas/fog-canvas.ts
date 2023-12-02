import SuperComponent from "@codewithkyle/supercomponent";
import env from "~brixi/controllers/env";
import { subscribe, unsubscribe } from "@codewithkyle/pubsub";
import type { Image } from "~types/app";
import room from "room";
import TabeltopComponent from "../tabletop-component";
import { send } from "~controllers/ws";

interface IFogCanvas { }
export default class FogCanvas extends SuperComponent<IFogCanvas>{
    private canvas: HTMLCanvasElement;
    private ctx: CanvasRenderingContext2D;
    private time: number;
    private gridSize: number;
    private fogOfWar: boolean;
    private ticket: string;
    private w: number;
    private h: number;
    private clearedCells: {
        [key: string]: boolean,
    }
    private tabletop: TabeltopComponent;
    private brushSize: number;

    constructor() {
        super();
        this.w = 0;
        this.h = 0;
        this.canvas = document.createElement("canvas") as HTMLCanvasElement;
        this.ctx = this.canvas.getContext("2d");
        this.tabletop = document.querySelector("tabletop-component");
        this.time = 0;
        this.gridSize = 32;
        this.fogOfWar = false;
        this.brushSize = 1;
        this.clearedCells = {};
        subscribe("socket", this.inbox.bind(this));
        subscribe("fog", this.fogInbox.bind(this));
    }

    override async connected() {
        await env.css(["fog-canvas"]);
        this.appendChild(this.canvas);
    }

    public convertViewportToTabletopPosition(clientX:number, clientY:number):Array<number>{
        const canvas = this.getBoundingClientRect();
        const x = Math.round(clientX - canvas.left + this.scrollLeft) / this.tabletop.zoom;
        const y = Math.round(clientY - canvas.top + this.scrollTop) / this.tabletop.zoom;
        return [x, y];
    }

    private fogInbox({ type, data }){
        const [x, y] = this.convertViewportToTabletopPosition(data.x, data.y);
        switch(type){
            case "eraser":
                this.brushSize = data.brushSize;
                this.erase(x, y);
                break;
            case "fill":
                this.brushSize = data.brushSize;
                this.fill(x, y);
                break;
            default:
                break;
        }
    }

    private inbox({ type, data }) {
        switch (type) {
            case "room:tabletop:fog:sync":
                this.fogOfWar = data.fogOfWar;
                this.clearedCells = data.clearedCells;
                this.renderFogOfWar();
                break;
            case "room:tabletop:clear":
                this.fogOfWar = false;
                this.clearedCells = {};
                this.renderFogOfWar();
                break;
            case "room:tabletop:map:update":
                this.gridSize = data.cellSize;
                this.renderFogOfWar();
                break;
            default:
                break;
        }
    }

    private fill(x: number, y: number){
        let x1, y1, x2, y2;
        if (this.brushSize > 2){
            x1 = x - (this.gridSize * this.brushSize * 0.5) + (this.gridSize * 0.25);
            y1 = y - (this.gridSize * this.brushSize * 0.5) + (this.gridSize * 0.25);
            x2 = x + (this.gridSize * this.brushSize * 0.5) - (this.gridSize * 0.25);
            y2 = y + (this.gridSize * this.brushSize * 0.5) - (this.gridSize * 0.25);
        } else {
            x1 = x - (this.gridSize * this.brushSize * 0.5);
            y1 = y - (this.gridSize * this.brushSize * 0.5);
            x2 = x + (this.gridSize * this.brushSize * 0.5);
            y2 = y + (this.gridSize * this.brushSize * 0.5);
        }
        const cellsInRow = Math.ceil(this.w / this.gridSize);
        const rows = Math.ceil(this.h / this.gridSize);
        for (let i = 0; i < rows; i++) {
            for (let j = 0; j < cellsInRow; j++) {
                const cellX = j * this.gridSize;
                const cellY = i * this.gridSize;
                if (cellX >= x1 && cellX + this.gridSize < x2 && cellY >= y1 && cellY + this.gridSize < y2){
                    const key = `${cellX}-${cellY}`;
                    if (key in this.clearedCells){
                        this.clearedCells[key] = false;
                        this.debounceSyncClearCells();
                    }
                }
            }
        }
        this.renderFogOfWar();
    }

    private erase(x: number, y: number){
        let x1, y1, x2, y2;
        if (this.brushSize > 2){
            x1 = x - (this.gridSize * this.brushSize * 0.5) + (this.gridSize * 0.25);
            y1 = y - (this.gridSize * this.brushSize * 0.5) + (this.gridSize * 0.25);
            x2 = x + (this.gridSize * this.brushSize * 0.5) - (this.gridSize * 0.25);
            y2 = y + (this.gridSize * this.brushSize * 0.5) - (this.gridSize * 0.25);
        } else {
            x1 = x - (this.gridSize * this.brushSize * 0.5);
            y1 = y - (this.gridSize * this.brushSize * 0.5);
            x2 = x + (this.gridSize * this.brushSize * 0.5);
            y2 = y + (this.gridSize * this.brushSize * 0.5);
        }
        const cellsInRow = Math.ceil(this.w / this.gridSize);
        const rows = Math.ceil(this.h / this.gridSize);
        for (let i = 0; i < rows; i++) {
            for (let j = 0; j < cellsInRow; j++) {
                const cellX = j * this.gridSize;
                const cellY = i * this.gridSize;
                if (cellX >= x1 && cellX + this.gridSize < x2 && cellY >= y1 && cellY + this.gridSize < y2){
                    const key = `${cellX}-${cellY}`;
                    this.clearedCells[key] = true;
                    this.debounceSyncClearCells();
                }
            }
        }
        this.renderFogOfWar();
    }

    private syncClearCells(){
        send("room:tabletop:fog:sync", this.clearedCells);
    }

    private debounceSyncClearCells = this.debounce(this.syncClearCells.bind(this), 500);

    private renderFogOfWar() {
        this.ctx.clearRect(0, 0, this.w, this.h);
        if (room.isGM){
            this.ctx.globalAlpha = 0.6;
        }
        if (this.fogOfWar) {
            const cells = [];
            const cellsInRow = Math.ceil(this.w / this.gridSize);
            const rows = Math.ceil(this.h / this.gridSize);
            for (let i = 0; i < rows; i++) {
                for (let j = 0; j < cellsInRow; j++) {
                    const x = j * this.gridSize;
                    const y = i * this.gridSize;
                    const key = `${x}-${y}`;
                    if (this.clearedCells?.[key]){
                        continue;
                    }
                    cells.push({
                        x: x,
                        y: y,
                    });
                }
            }

            this.ctx.fillStyle = "rgba(24,24,24)";
            for (let i = 0; i < cells.length; i++) {
                const { x, y } = cells[i];
                this.ctx.fillRect(x, y, this.gridSize, this.gridSize);
            }
        }
    }

    // @ts-ignore
    override render(image: HTMLImageElement): void {
        if (!image) {
            this.w = 0;
            this.h = 0;
            this.canvas.width = this.w;
            this.canvas.height = this.h;
            this.canvas.style.width = `0px`;
            this.canvas.style.height = `0px`;
        } else {
            this.w = image.width;
            this.h = image.height;
            this.canvas.width = this.w;
            this.canvas.height = this.h;
            this.canvas.style.width = `${image.width}px`;
            this.canvas.style.height = `${image.height}px`;
        }
        this.renderFogOfWar();
    }
}
env.bind("fog-canvas", FogCanvas);
