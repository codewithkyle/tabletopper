import db from "@codewithkyle/jsql";
import { publish, subscribe } from "@codewithkyle/pubsub";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import env from "~brixi/controllers/env";
import notifications from "~brixi/controllers/notifications";
import PlayerPawn from "~components/player-pawn/player-pawn";
import { setValueFromKeypath } from "~utils/object";

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

    constructor(){
        super();
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
            case "locate:pawn":
                const el = this.querySelector(`[data-uid="${data}"]`);
                if (el){
                    this.moving = false;
                    const bounds = el.getBoundingClientRect();
                    const diffX = window.innerWidth * 0.5 - bounds.x;
                    const diffY = window.innerHeight * 0.5 - bounds.y;
                    this.x = this.x + diffX;
                    this.y = this.y + diffY;
                    this.style.transform = `matrix(${this.zoom}, 0, 0, ${this.zoom}, ${this.x}, ${this.y})`;
                } else {
                    notifications.snackbar("Failed to locate pawn.");
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
            default:
                break;
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
                    this.set(updatedModel);
                    if (op.keypath === "map"){
                        this.moving = false;
                        this.x = window.innerWidth * 0.5;
                        this.y = (window.innerHeight - 28) * 0.5;
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

    private handleWindowDown:EventListener = (e:MouseEvent) =>{
        this.moving = true;
        this.lastX = e.clientX;
        this.lastY = e.clientY;
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
        let image = null;
        if (this.model.map){
            image = (await db.query("SELECT * FROM images WHERE uid = $uid", { uid : this.model.map }))[0];
        }
        const playerPawns = await db.query("SELECT * FROM pawns WHERE playerId != $value AND room = $room", {
            value: null,
            room: sessionStorage.getItem("room"),
        });
        const view = html`
            ${image ? html`<img class="center absolute" src="${image.data}" alt="${image.name}" draggable="false">` : ""}
            ${playerPawns.map(pawn => {
                return new PlayerPawn(pawn);
            })}
        `;
        render(view, this);
        this.style.transform = `matrix(${this.zoom}, 0, 0, ${this.zoom}, ${this.x}, ${this.y})`;
    }
}
env.bind("tabletop-component", TabeltopComponent);
