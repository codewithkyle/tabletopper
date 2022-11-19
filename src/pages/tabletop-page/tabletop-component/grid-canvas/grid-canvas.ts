import SuperComponent from "@codewithkyle/supercomponent";
import env from "~brixi/controllers/env";
import db from "@codewithkyle/jsql";
import {subscribe, unsubscribe} from "@codewithkyle/pubsub";

interface IGridCanvas {}
export default class GridCanvas extends SuperComponent<IGridCanvas>{
    private canvas:HTMLCanvasElement;
    private ctx:CanvasRenderingContext2D;
    private time:number;
    private gridSize: number;
    private renderGrid: boolean;

    constructor(){
        super();
        this.gridSize = 32;
        this.renderGrid = false;
        this.ticket = subscribe("sync", this.inbox.bind(this));
    }

    override async connected(){
        await env.css(["grid-canvas"]);
        const result = (await db.query("SELECT * FROM games WHERE uid = $room", { room: sessionStorage.getItem("room") }))[0];
        this.gridSize = result["grid_size"];
        this.renderGrid = result["render_grid"];
        this.render();
    }

    override disconnected(): void {
        unsubscribe(this.ticket);
    }

    private inbox(op){
        switch (op.op){
            case "SET":
                if (op.table === "games" && op.key === sessionStorage.getItem("room") && (op.keypath === "grid_size" || op.keypath === "render_grid")){
                    this.renderCanvas();
                }
                break;
            default:
                break;
        }
    }

    private renderCanvas(){
        this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
        const newTime = performance.now();
        const deltaTime = (newTime - this.time) / 1000;
        this.time = newTime;

        if (this.renderGrid){
            const cells = [];
            const cellsInRow = Math.ceil(this.canvas.width / this.gridSize);
            const rows = Math.ceil(this.canvas.height / this.gridSize);
            for (let i = 0; i < rows; i++){
                for (let j = 0; j < cellsInRow; j++){
                    cells.push({
                        x: j * this.gridSize,
                        y: i * this.gridSize,
                    });
                }
            }

            for (let i = 0; i < cells.length; i++){
                const { x, y } = cells[i];
                this.ctx.strokeStyle = "rgba(0,0,0,0.6)";
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

    override render(): void {
        this.innerHTML = `<canvas></canvas>`;
        const bgImg = this.parentElement.querySelector(":scope > img");
        this.canvas = this.querySelector("canvas");
        const bgImgBounds = bgImg.getBoundingClientRect();
        this.canvas.width = bgImgBounds.width;
        this.canvas.height = bgImgBounds.height;
        this.ctx = this.canvas.getContext("2d");
        this.time = performance.now();
        this.renderCanvas();
    }
}
env.bind("grid-canvas", GridCanvas);
