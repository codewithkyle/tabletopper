import db from "@codewithkyle/jsql";
import { publish, subscribe } from "@codewithkyle/pubsub";
import SuperComponent from "@codewithkyle/supercomponent";
import env from "~brixi/controllers/env";
import { setValueFromKeypath } from "~utils/object";
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
        subscribe("sync", this.syncInbox.bind(this));
        subscribe("tabletop", this.tabletopInbox.bind(this));
    }

     tabletopInbox({type, data}){
        switch(type){
            case "clear":
                this.querySelectorAll("pawn-component").forEach((el:HTMLElement) => {
                    el.remove();
                });
                break;
            case "locate:player":
                if (data != null){
                    const playerPawn = this.querySelector(`[data-player-uid="${data}"]`) as HTMLElement;
                    this.locatePawn(playerPawn);
                }
                break;
            case "locate:pawn":
                if (data != null){
                    const pawn = this.querySelector(`[data-uid="${data}"]`) as HTMLElement;
                    this.locatePawn(pawn);
                }
                break;
            case "position:reset":
                this.moving = false;
                this.x = window.innerWidth * 0.5;
                this.y = (window.innerHeight - 28) * 0.5;
                this.style.transform = `matrix(${this.zoom}, 0, 0, ${this.zoom}, ${this.x}, ${this.y})`;
                break;
            case "zoom":
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
                            data.raito = 0;
                        }
                    }
                    this.zoom = data.zoom;
                    this.x = data.x - data.ratio * (data.x - this.x);
                    this.y = data.y - data.ratio * (data.y - this.y);
                    this.style.transform = `matrix(${this.zoom}, 0, 0, ${this.zoom}, ${this.x}, ${this.y})`;
                    sessionStorage.setItem("zoom", this.zoom.toFixed(2).toString());
                }
                break;
            case "ping":
                const { x, y, color } = data;
                const pingEl = new PingComponent(x, y, color, this.zoom);
                this.appendChild(pingEl);
                break;
            default:
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

    private syncInbox(op){
        let updatedModel = this.get();
        switch (op.op){
            case "BATCH":
                for (let i = 0; i < op.ops.length; i++){
                    this.syncInbox(op.ops[i]);
                }
                break;
            case "SET":
                if(op.table === "games"){
                    setValueFromKeypath(updatedModel, op.keypath, op.value);
                    if (op.keypath === "map"){
                        this.moving = false;
                        this.x = window.innerWidth * 0.5;
                        this.y = (window.innerHeight - 28) * 0.5;
                        this.set(updatedModel);
                    } else {
                        this.set(updatedModel, true);
                    }
                }
                break;
            case "INSERT":
                if (op.table === "pawns"){
                    if (op.value?.playerId != null){
                        const el = this.querySelector(`pawn-component[data-player-uid="${op.value.playerId}"]`) || new Pawn(op.value);
                        if (!el.isConnected){
                            this.appendChild(el);
                        }
                    } else if (op.value?.monsterId != null){
                        const el = this.querySelector(`pawn-component[data-uid="${op.value.uid}"]`) || new Pawn(op.value);
                        if (!el.isConnected){
                            this.appendChild(el);
                        }
                    } else {
                        const el = this.querySelector(`pawn-component[data-uid="${op.value.uid}"]`) || new Pawn(op.value);
                        if (!el.isConnected){
                            this.appendChild(el);
                        }
                    }
                }
                break;
            default:
                break;
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
        const target = e.target as HTMLElement;
        if (target.closest("tabletop-page")){
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
            publish("tabletop", {
                type: "zoom",
                data: {
                    zoom: zoom,
                    x: e.clientX,
                    y: e.clientY,
                    delta: e.deltaY,
                    ratio: multiplier,
                },
            });
        }
    }

    private handleWindowDown:EventListener = (e:MouseEvent|TouchEvent) =>{
        if (e.target.classList.contains("map")){
            this.moving = true;
            if (window.TouchEvent && e instanceof TouchEvent){
                this.lastX = e.touches[0].clientX;
                this.lastY = e.touches[0].clientY;
            } else {
                this.lastX = e.clientX;
                this.lastY = e.clientY;
            }
        }
    }

    private handleMouseUp:EventListener = () => {
        this.moving = false;
    }

    private handleMouseMove:EventListener = (e:MouseEvent|TouchEvent) => {
        if (this.moving){
            let x;
            let y;
            if (window.TouchEvent && e instanceof TouchEvent){
                x = e.touches[0].clientX;
                y = e.touches[0].clientY;
            } else {
                x = e.clientX;
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
        let image:Image = null;
        if (this.model.map){
            // Wait and retry query due to insert race condition
            // TODO: Find a better way to do this
            while (image === null){
                await new Promise((resolve) => setTimeout(resolve, 1000));
                image = (await db.query<Image>("SELECT * FROM images WHERE uid = $uid", { uid : this.model.map }))?.[0] ?? null;
            }
        }
        if (image !== null){
            this.img.src = image.data;
            this.img.alt = image.name;
        } else {
            this.img.src = "";
            this.img.alt = "";
        }
        if (!this.img.isConnected){
            this.img.className = "center absolute map";
            this.img.draggable = false;
            this.appendChild(this.img);
        }
        if (!this.gridCanvas.isConnected){
            this.appendChild(this.gridCanvas);
        }
        if (!this.vfxCanvas.isConnected){
            this.appendChild(this.vfxCanvas);
        }
        this.gridCanvas.render(image);
        this.vfxCanvas.render(image);
        this.style.transform = `matrix(${this.zoom}, 0, 0, ${this.zoom}, ${this.x}, ${this.y})`;
    }
}
env.bind("tabletop-component", TabeltopComponent);
