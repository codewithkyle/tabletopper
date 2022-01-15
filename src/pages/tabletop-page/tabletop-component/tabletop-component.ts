import db from "@codewithkyle/jsql";
import { publish, subscribe } from "@codewithkyle/pubsub";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import env from "~brixi/controllers/env";
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

    private tabletopInbox({type, data}){
        switch(type){
            case "position:reset":
                    this.moving = false;
                    this.x = window.innerWidth * 0.5;
                    this.y = (window.innerHeight - 28) * 0.5;
                    this.style.transform = `translate(${this.x}px, ${this.y}px) scale(${this.zoom})`;
                break;
            case "zoom":
                this.moving = false;
                this.zoom = data;
                this.style.transform = `translate(${this.x}px, ${this.y}px) scale(${this.zoom})`;
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
        let zoom = this.zoom + -(e.deltaY * 0.001);
        if (zoom > 2){
            zoom = 2;
        } else if (zoom < 0.1){
            zoom = 0.1;
        }
        publish("tabletop", {
            type: "zoom",
            data: zoom,
        });
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
            if (e instanceof TouchEvent){
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
            this.style.transform = `translate(${this.x}px, ${this.y}px) scale(${this.zoom})`;
            this.lastX = x;
            this.lastY = y;
        }
    }

    override async render() {
        let image = null;
        if (this.model.map){
            image = (await db.query("SELECT * FROM images WHERE uid = $uid", { uid : this.model.map }))[0];
        }
        const players = [];
        if (this.model.players.length){
            for (const uid of this.model.players){
                const player = (await db.query("SELECT * FROM players WHERE uid = $uid AND active = $status", { uid: uid, status: true }))?.[0] ?? null;
                if (player){
                    players.push(player);
                }
            }
        }
        const view = html`
            ${image ? html`<img class="center absolute" src="${image.data}" alt="${image.name}" draggable="false">` : ""}
            ${players.map(player => {
                return new PlayerPawn(player);
            })}
        `;
        render(view, this);
        this.style.transform = `translate(${this.x}px, ${this.y}px) scale(${this.zoom})`;
    }
}
env.bind("tabletop-component", TabeltopComponent);