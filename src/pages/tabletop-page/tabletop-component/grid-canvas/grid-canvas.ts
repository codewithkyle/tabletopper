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
    private ticket: string;
    private img:HTMLImageElement;

    constructor(img:HTMLImageElement){
        super();
        this.img = img;
        this.canvas = null;
        this.ctx = null;
        this.time = 0;
        this.gridSize = 32;
        this.renderGrid = false;
        this.ticket = subscribe("sync", this.inbox.bind(this));
    }

    override async connected(){
        await env.css(["grid-canvas"]);
        const result = (await db.query("SELECT * FROM games WHERE uid = $room", { room: sessionStorage.getItem("room") }))[0];
        this.gridSize = result?.["grid_size"] ?? 32;
        this.renderGrid = result?.["render_grid"] ?? false;
    }

    override disconnected(): void {
        unsubscribe(this.ticket);
    }

    private inbox(op){
        switch (op.op){
            case "SET":
                if (op.table === "games" && op.key === sessionStorage.getItem("room")){
                    if (op.keypath === "grid_size"){
                        this.gridSize = op.value;
                        this.render();
                    } else if (op.keypath === "render_grid"){
                        this.renderGrid = op.value;
                        this.render();
                    }
                }
                break;
            default:
                break;
        }
    }

    // @ts-ignore
    override render(): void {
        if (!this.canvas){
            this.canvas = document.createElement("canvas") as HTMLCanvasElement;
            this.appendChild(this.canvas);
        }
        this.canvas.width = this.img.width;
        this.canvas.height = this.img.height;
        this.canvas.style.width = `${this.img.width}px`;
        this.canvas.style.height = `${this.img.height}px`;
        this.ctx = this.canvas.getContext("2d");
        this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);

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
}
env.bind("grid-canvas", GridCanvas);
