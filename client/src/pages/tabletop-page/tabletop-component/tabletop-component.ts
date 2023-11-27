import { publish, subscribe } from "@codewithkyle/pubsub";
import SuperComponent from "@codewithkyle/supercomponent";
import env from "~brixi/controllers/env";
import Pawn from "~components/pawn/pawn";
import VFXCanvas from "./vfx-canvas/vfx-canvas";
import GridCanvas from "./grid-canvas/grid-canvas";
import type { Image } from "~types/app";
import PingComponent from "./ping-component/ping-component";

interface ITabletopComponent {
    map: string,
    players: Array<string>,
}
export default class TabeltopComponent extends SuperComponent<ITabletopComponent>{
    private moving: boolean;
    public x: number;
    public y: number;
    private lastX: number;
    private lastY: number;
    public zoom: number;
    private vfxCanvas: VFXCanvas;
    private gridCanvas: GridCanvas;
    private img: HTMLImageElement;

    constructor(){
        super(); 
        this.img = new Image();
        this.vfxCanvas = new VFXCanvas();
        this.gridCanvas = new GridCanvas();
        this.moving = false;
        this.x = window.innerWidth * 0.5;
        this.y = (window.innerHeight - 28) * 0.5;
        this.zoom = 1;
        this.model = {
            map: null,
            players: [],
        };
        subscribe("socket", this.inbox.bind(this));
    }

    private inbox({ type, data }){
        switch(type){
            case "room:tabletop:load":
                this.model.map = `https://tabletopper.nyc3.digitaloceanspaces.com/maps/${data}`;
                this.render();
                break;
            case "room:tabletop:clear":
                this.model.map = null;
                this.render();
                break;
        }
    }

    private locatePawn(el:HTMLElement){
        if (el != null && el instanceof HTMLElement && el.isConnected){
            this.moving = false;
            const bounds = el.getBoundingClientRect();
            const diffX = window.innerWidth * 0.5 - bounds.x;
            const diffY = window.innerHeight * 0.5 - bounds.y;
            this.x = this.x + diffX;
            this.y = this.y + diffY;
            this.style.transform = `matrix(${this.zoom}, 0, 0, ${this.zoom}, ${this.x}, ${this.y})`;
        }
    }

    override async connected(){
        await env.css(["tabletop-component"]);
        window.addEventListener("mousemove", this.handleMouseMove, { passive: true });
        window.addEventListener("mousedown", this.handleWindowDown, { passive: true });
        window.addEventListener("mouseup", this.handleMouseUp, { passive: true });
        window.addEventListener("touchmove", this.handleMouseMove, { passive: true });
        window.addEventListener("touchstart", this.handleWindowDown, { passive: true });
        window.addEventListener("touchend", this.handleMouseUp, { passive: true });
        window.addEventListener("wheel", this.handleScroll, { passive: true });
        document.body.addEventListener("drop", this.handleDrop, { passive: false, capture: true });
        this.render();
    }

    private handleDrop:EventListener = (e:DragEvent) => {
        e.preventDefault();
        e.stopImmediatePropagation();
        console.log(e);
    }

    private handleScroll:EventListener = (e:WheelEvent) => {
        const target = this.querySelector("tabletop-page");
        let delta = e.deltaY;
        let sign = Math.sign(delta);
        let deltaAdjustedSpeed = Math.min(0.25, Math.abs(0.25 * delta / 128));
        let multiplier = 1 - sign * deltaAdjustedSpeed;
        let zoom = this.zoom * multiplier;
        if (zoom > 2){
            zoom = 2;
        } else if (zoom < 0.1){
            zoom = 0.1;
        }
        const data = {
            zoom: zoom,
            x: e.clientX,
            y: e.clientY,
            delta: e.deltaY,
            ratio: multiplier,
        };
        this.moving = false;
        if (this.zoom !== data.zoom){
            if (data.x === null){
                data.x = window.innerWidth * .5;
            }
            if (data.y === null){
                data.y = window.innerHeight * .5;
            }
            if (!data?.ratio){
                if (data.zoom < this.zoom){
                    data.ratio = 0.8046875;
                }
                else if (data.zoom > this.zoom){
                    data.ratio = 1.1953125;
                }
                else {
                    data.ratio = 0;
                }
            }
            this.zoom = data.zoom;
            this.x = data.x - data.ratio * (data.x - this.x);
            this.y = data.y - data.ratio * (data.y - this.y);
            this.style.transform = `matrix(${this.zoom}, 0, 0, ${this.zoom}, ${this.x}, ${this.y})`;
            sessionStorage.setItem("zoom", this.zoom.toFixed(2).toString());
        }
    }

    private handleWindowDown:EventListener = (e:MouseEvent|TouchEvent) =>{
        this.moving = true;
        if (window.TouchEvent && e instanceof TouchEvent){
            this.lastX = e.touches[0].clientX;
            this.lastY = e.touches[0].clientY;
        } else {
            this.lastX = e.clientX;
            this.lastY = e.clientY;
        }
    }

    private handleMouseUp:EventListener = () => {
        this.moving = false;
    }

    private handleMouseMove:EventListener = (e:MouseEvent|TouchEvent) => {
        if (this.moving){
            let x = 0;
            let y = 0;
            if (window.TouchEvent && e instanceof TouchEvent){
                x = e.touches[0].clientX;
                y = e.touches[0].clientY;
            } else {
                // @ts-ignore
                x = e.clientX;
                // @ts-ignore
                y = e.clientY;
            }
            const deltaX = this.lastX - x;
            const deltaY = this.lastY - y;
            this.x -= deltaX;
            this.y -= deltaY;
            this.style.transform = `matrix(${this.zoom}, 0, 0, ${this.zoom}, ${this.x}, ${this.y})`;
            this.lastX = x;
            this.lastY = y;
        }
    }

    override async render() {
        if (this.model.map){
            this.img = new Image();
            this.img.src = this.model.map;
            this.img.className = "center absolute map";
            this.img.draggable = false;
        } else {
            this.img?.remove();
            this.img = null;
        }
        if (!this.img.isConnected){
            this.appendChild(this.img);
        }
        if (!this.gridCanvas.isConnected){
            this.appendChild(this.gridCanvas);
        }
        if (!this.vfxCanvas.isConnected){
            this.appendChild(this.vfxCanvas);
        }
        this.img.onload = () => {
            this.gridCanvas.render(this.img);
            this.vfxCanvas.render(this.img);
        }
        this.style.transform = `matrix(${this.zoom}, 0, 0, ${this.zoom}, ${this.x}, ${this.y})`;
    }
}
env.bind("tabletop-component", TabeltopComponent);
