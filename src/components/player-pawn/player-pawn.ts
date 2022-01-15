import db from "@codewithkyle/jsql";
import { subscribe } from "@codewithkyle/pubsub";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render, TemplateResult } from "lit-html";
import TabeltopComponent from "pages/tabletop-page/tabletop-component/tabletop-component";
import env from "~brixi/controllers/env";
import cc from "~controllers/control-center";
import { ConvertBase64ToBlob } from "~utils/file";
import { setValueFromKeypath } from "~utils/object";

export interface IPlayerPawn {
    name: string,
    uid: string,
    token: string,
    image: string,
    x: number,
    y: number,
    active: boolean,
};
export default class PlayerPawn extends SuperComponent<IPlayerPawn>{
    private dragging: boolean;
    private localX: number;
    private localY: number;

    constructor(player){
        super();
        this.dragging = false;
        this.localX = 0;
        this.localY = 0;
        this.model = {
            name: player.name,
            uid: player.uid,
            token: player.token,
            image: null,
            x: player.x,
            y: player.y,
            active: player.active,
        };
        subscribe("pawn", this.pawnInbox.bind(this));
        subscribe("sync", this.syncInbox.bind(this));
    }
    
    override async connected(){
        await env.css(["player-pawn"]);
        if (this.model.token){
            const image = (await db.query("SELECT * FROM images WHERE uid = $uid", { uid: this.model.token }))[0];
            const blob = ConvertBase64ToBlob(image.data);
            const url = URL.createObjectURL(blob);
            this.set({
                image: url,
            });
        }
        window.addEventListener("mousemove", this.dragPawn, { passive: true, capture: true });
        this.addEventListener("mousedown", this.startDrag, { passive: false, capture: true });
        window.addEventListener("mouseup", this.stopDrag, { passive: true, capture: true });
        this.render();
    }

    private pawnInbox({ type, data }){
        switch(type){
            case "drop":
                if (this.dragging){
                    // TODO: move pawn to location
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
                if(op.table === "players" && op.key === this.model.uid && !this.dragging){
                    setValueFromKeypath(updatedModel, op.keypath, op.value);
                    this.set(updatedModel);
                }
                break;
            default:
                break;
        }
    }

    private stopDrag:EventListener = (e:DragEvent) => {
        this.classList.remove("no-anim");
        this.setAttribute("tooltip", this.model.name);
        this.setAttribute("sfx", "button");
        const wasDraggnig = this.dragging;
        this.dragging = false;
        if (wasDraggnig){
            const op1 = cc.set("players", this.model.uid, "x", this.localX);
            const op2 = cc.set("players", this.model.uid, "y", this.localY);
            const ops = cc.batch("players", this.model.uid, [op1, op2]);
            cc.perform(ops);
            cc.dispatch(ops);
        }
    }

    private startDrag:EventListener = (e:DragEvent) => {
        e.preventDefault();
        e.stopImmediatePropagation();
        this.dragging = true;
        const tooltip = document.body.querySelector(`tool-tip[uid="${this.dataset.tooltipUid}"]`);
        if (tooltip){
            tooltip.remove();
        }
        this.removeAttribute("sfx");
        this.removeAttribute("tooltip");
        this.classList.add("no-anim");
    }

    private dragPawn:EventListener = (e:MouseEvent) => {
        if (this.dragging){
            const tabletop = this.parentElement as TabeltopComponent;
            const x = e.clientX;
            const y = e.clientY;
            const bounds = this.getBoundingClientRect();
            let diffX = (tabletop.x - x);
            let diffY = (tabletop.y - y);
            diffX += bounds.width * 0.5;
            diffY += bounds.height * 0.5;
            diffX /= tabletop.zoom;
            diffY /= tabletop.zoom;
            this.localX = -diffX;
            this.localY = -diffY;
            this.style.transform = `translate(${this.localX}px, ${this.localY}px)`;
        }
    }

    private renderPawn():TemplateResult{
        let out;
        if (this.model.image){
            out = html`
                <img src="${this.model.image}" alt="${this.model.name} token" draggable="false">
            `;
        } else {
            out = html`
                <div class="generic-pawn"></div>
            `;
        }
        return out;
    }

    override async render() {
        if (!this.model.active){
            this.style.visibility = "hidden";
            this.style.opacity = "0";
            this.style.pointerEvents = "none";
        } else {
            this.style.visibility = "visible";
            this.style.opacity = "1";
            this.style.pointerEvents = "all";
        }
        this.dataset.uid = this.model.uid;
        if (!this.dragging){
            this.setAttribute("tooltip", this.model.name);
            this.setAttribute("sfx", "button");
        }
        this.style.transform = `translate(${this.model.x}px, ${this.model.y}px)`;
        this.localX = this.model.x;
        this.localY = this.model.y;
        const view = html`
            ${this.renderPawn()}
        `;
        render(view, this);
    }
}
env.bind("player-pawn", PlayerPawn);