import SuperComponent from "@codewithkyle/supercomponent";
import env from "~brixi/controllers/env";
import { subscribe, unsubscribe } from "@codewithkyle/pubsub";
import type { Image } from "~types/app";
import room from "room";
import TabeltopComponent from "../tabletop-component";
import { send } from "~controllers/ws";

interface IDoodleCanvas { }
export default class DoodleCanvas extends SuperComponent<IDoodleCanvas>{
    private canvas: HTMLCanvasElement;
    private ctx: CanvasRenderingContext2D;
    private w: number;
    private h: number;
    private tabletop: TabeltopComponent;
    private isDrawing:boolean;
    private color:string;

    constructor() {
        super();
        this.w = 0;
        this.h = 0;
        this.canvas = document.createElement("canvas") as HTMLCanvasElement;
        this.ctx = this.canvas.getContext("2d");
        this.tabletop = document.querySelector("tabletop-component");
        this.isDrawing = false;
        this.color = "#ffffff";
        subscribe("socket", this.inbox.bind(this));
        subscribe("doodle", this.doodleInbox.bind(this));
    }

    override async connected() {
        await env.css(["doodle-canvas"]);
        this.appendChild(this.canvas);
    }

    public convertViewportToTabletopPosition(clientX:number, clientY:number):Array<number>{
        const canvas = this.getBoundingClientRect();
        const x = Math.round(clientX - canvas.left + this.scrollLeft) / this.tabletop.zoom;
        const y = Math.round(clientY - canvas.top + this.scrollTop) / this.tabletop.zoom;
        return [x, y];
    }

    private doodleInbox({ type, data }) {
        const [x, y] = this.convertViewportToTabletopPosition(data.x, data.y);
        switch (type) {
            case "color":
                this.color = data;
                break;
            case "stop":
                this.isDrawing = false;
                this.ctx.closePath();
                this.send();
                break;
            case "mouse":
                break;
            case "draw":
                this.color = data.color;
                this.draw(x, y);
                break;
            case "erase":
                this.erase(x, y);
                break;
            default:
                break;
        }
    }

    private inbox({ type, data }) {
        switch (type) {
            case "room:tabletop:doodle":
                const img = new Image();
                img.src = `data:image/png;base64,${data}`;
                img.onload = () => {
                    this.ctx.clearRect(0, 0, this.w, this.h);
                    this.ctx.drawImage(img, 0, 0);
                };
                break;
            case "room:tabletop:clear":
                this.renderDoodles();
                break;
            case "room:tabletop:map:update":
                this.renderDoodles();
                break;
            default:
                break;
        }
    }

    private send(){
        const snapshot = this.canvas.toDataURL("image/png").replace("data:image/png;base64,", "");
        send("room:tabletop:doodle", snapshot);
    }

    private draw(x, y){
        if (!this.isDrawing){
            this.isDrawing = true;
            this.ctx.strokeStyle = this.color;
            this.ctx.fillStyle = this.color;
            this.ctx.lineWidth = 2;
            this.ctx.moveTo(x, y);
            this.ctx.beginPath();
        }
        this.ctx.globalCompositeOperation = "source-over";
        this.ctx.lineTo(x, y);
        this.ctx.stroke();
    }

    private erase(x, y){
        if (!this.isDrawing){
            this.isDrawing = true;
            this.ctx.strokeStyle = this.color;
            this.ctx.fillStyle = this.color;
            this.ctx.lineWidth = 16;
            this.ctx.moveTo(x, y);
            this.ctx.beginPath();
        }
        this.ctx.globalCompositeOperation = "destination-out";
        this.ctx.lineTo(x, y);
        this.ctx.stroke();
    }

    private renderDoodles() {
        this.ctx.clearRect(0, 0, this.w, this.h);
        this.ctx.lineCap = "round";
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
        this.renderDoodles();
    }
}
env.bind("doodle-canvas", DoodleCanvas);
