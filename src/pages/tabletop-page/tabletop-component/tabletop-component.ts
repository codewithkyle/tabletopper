import db from "@codewithkyle/jsql";
import { subscribe } from "@codewithkyle/pubsub";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import env from "~brixi/controllers/env";
import { setValueFromKeypath } from "~utils/object";

interface ITabletopComponent {
    map: string,
}
export default class TabeltopComponent extends SuperComponent<ITabletopComponent>{
    private moving: boolean;
    private x: number;
    private y: number;
    private lastX: number;
    private lastY: number;

    constructor(){
        super();
        this.moving = false;
        this.x = window.innerWidth * 0.5;
        this.y = (window.innerHeight - 28) * 0.5;
        this.model = {
            map: null,
        };
        subscribe("sync", this.syncInbox.bind(this));
    }

    private syncInbox(op){
        let updatedModel = this.get();
        switch (op.op){
            case "SET":
                if(op.table === "games"){
                    setValueFromKeypath(updatedModel, op.keypath, op.value);
                    this.set(updatedModel);
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
        this.render();
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
            this.style.transform = `translate(${this.x}px, ${this.y}px)`;
            this.lastX = x;
            this.lastY = y;
        }
    }

    override async render() {
        let image = null;
        if (this.model.map){
            image = (await db.query("SELECT * FROM images WHERE uid = $uid", { uid : this.model.map }))[0];
        }
        const view = html`
            ${image ? html`<img class="center absolute" src="${image.data}" alt="${image.name}" draggable="false">` : ""}
        `;
        render(view, this);
        this.style.transform = `translate(${this.x}px, ${this.y}px)`;
    }
}
env.bind("tabletop-component", TabeltopComponent);