import SuperComponent from "@codewithkyle/supercomponent";
import env from "~brixi/controllers/env";
import { subscribe, unsubscribe } from "@codewithkyle/pubsub";
import type { Image } from "~types/app";

interface IGridCanvas { }
export default class GridCanvas extends SuperComponent<IGridCanvas>{
    private canvas: HTMLCanvasElement;
    private ctx: CanvasRenderingContext2D;
    private time: number;
    private gridSize: number;
    private renderGrid: boolean;
    private ticket: string;
    private w: number;
    private h: number;

    constructor() {
        super();
        this.w = 0;
        this.h = 0;
        this.canvas = document.createElement("canvas") as HTMLCanvasElement;
        this.ctx = this.canvas.getContext("2d");
        this.time = 0;
        this.gridSize = 32;
        this.renderGrid = false;
        this.ticket = subscribe("socket", this.inbox.bind(this));
    }

    override async connected() {
        await env.css(["grid-canvas"]);
        this.appendChild(this.canvas);
    }

    override disconnected() {
        unsubscribe(this.ticket);
    }

    private inbox({ type, data }) {
        switch (type) {
            case "room:tabletop:clear":
                this.renderGrid = false;
                this.renderGridLines();
                break;
            case "room:tabletop:map:update":
                this.renderGrid = data.renderGrid;
                this.gridSize = data.cellSize;
                this.renderGridLines();
                break;
            default:
                break;
        }
    }

    private renderGridLines() {
        this.ctx.clearRect(0, 0, this.w, this.h);
        if (this.renderGrid) {
            const cells = [];
            const cellsInRow = Math.ceil(this.w / this.gridSize);
            const rows = Math.ceil(this.h / this.gridSize);
            for (let i = 0; i < rows; i++) {
                for (let j = 0; j < cellsInRow; j++) {
                    cells.push({
                        x: j * this.gridSize,
                        y: i * this.gridSize,
                    });
                }
            }

            this.ctx.strokeStyle = "rgba(0,0,0,0.6)";
            for (let i = 0; i < cells.length; i++) {
                const { x, y } = cells[i];
                this.ctx.beginPath();
                this.ctx.moveTo(x, y);
                this.ctx.lineTo(x, y + this.gridSize);
                this.ctx.stroke();
                this.ctx.beginPath();
                this.ctx.moveTo(x, y);
                this.ctx.lineTo(x + this.gridSize, y);
                this.ctx.stroke();
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
        this.renderGridLines();
    }
}
env.bind("grid-canvas", GridCanvas);
