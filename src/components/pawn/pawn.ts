import SuperComponent from "@codewithkyle/supercomponent";
import env from "~brixi/controllers/env";
import { subscribe } from "@codewithkyle/pubsub";
import db from "@codewithkyle/jsql";
import { ConvertBase64ToBlob } from "utils/file";
import { setValueFromKeypath } from "utils/object";
import cc from "controllers/control-center";
import TabeltopComponent from "pages/tabletop-page/tabletop-component/tabletop-component";
import {html, render, TemplateResult} from "lit-html";
import Window from "~components/window/window";
import StatBlock from "components/window/windows/stat-block/stat-block";

interface IPawn{
    uid: string,
    x: number,
    y: number,
    image: string|null;
    hidden: boolean;
    token: string|null;
    name: string;
    playerId: string|null;
    monsterId: string|null;
    rings: {
        blue: boolean,
        green: boolean,
        orange: boolean,
        pink: boolean,
        purple: boolean,
        red: boolean,
        white: boolean,
        yellow: boolean,
    },
}
export default class Pawn extends SuperComponent<IPawn>{
    public dragging: boolean;
    public localX: number;
    public localY: number;

    constructor(pawn){
        super();
        this.dragging = false;
        this.localX = pawn.x;
        this.localY = pawn.y;
        this.model = {
            uid: pawn.uid,
            x: pawn.x,
            y: pawn.y,
            image: null,
            hidden: pawn?.hidden ?? false,
            name: pawn.name,
            token: pawn?.token ?? null,
            playerId: pawn?.playerId ?? null,
            monsterId: pawn?.monsterId ?? null,
            rings: pawn.rings,
        };
        subscribe("sync", this.syncInbox.bind(this));
    }
    
    override async connected() {
        await env.css(["pawn"]);
        window.addEventListener("mousemove", this.dragPawn, { passive: true, capture: true });
        this.addEventListener("mousedown", this.startDrag, { passive: false, capture: true });
        window.addEventListener("mouseup", this.stopDrag, { passive: true, capture: true });
        this.addEventListener("contextmenu", this.contextMenu, { passive: false, capture: true });
        if (this.model.playerId){
            const player = (await db.query("SELECT * FROM players WHERE uid = $uid", { uid: this.model.playerId }))[0];
            if (player.token){
                await this.loadImage(player.token);
            }
        }
        this.render();
    }

    public async loadImage(imageId:string){
        const image = (await db.query("SELECT * FROM images WHERE uid = $uid", { uid: imageId }))[0];
        const blob = ConvertBase64ToBlob(image.data);
        const url = URL.createObjectURL(blob);
        this.set({
            image: url,
        });
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
                if(op.table === "pawns" && op.key === this.model.uid && !this.dragging){
                    setValueFromKeypath(updatedModel, op.keypath, op.value);
                    this.set(updatedModel);
                }
                break;
            case "DELETE":
                if (op.table === "pawns" && op.key === this.model.uid){
                    this.remove();
                }
                break;
            default:
                break;
        }
    }

    private contextMenu:EventListener = (e:MouseEvent) => {
        e.preventDefault();
        e.stopImmediatePropagation();
        if (this.model?.playerId == null && sessionStorage.getItem("role") === "gm"){
            const x = e.clientX;
            const y = e.clientY;
            const windowEl = new Window({
                name: `${this.model.name} ${this.model?.monsterId == null ? "(npc)" : "(monster)"}`,
                width: 400,
                height: 200,
                view: new StatBlock(this.model.uid, this.model.monsterId),
                handle: "stat-block",
            });
            if (!windowEl.isConnected){
                document.body.appendChild(windowEl);
            }
        }
    }

    private stopDrag:EventListener = (e:DragEvent) => {
        this.classList.remove("no-anim");
        this.setAttribute("tooltip", this.model.name);
        this.setAttribute("sfx", "button");
        if (this.dragging){
            const op1 = cc.set("pawns", this.model.uid, "x", this.localX);
            const op2 = cc.set("pawns", this.model.uid, "y", this.localY);
            const ops = cc.batch("pawns", this.model.uid, [op1, op2]);
            cc.dispatch(ops);
        }
        this.dragging = false;
    }

    private startDrag:EventListener = (e:MouseEvent) => {
        e.preventDefault();
        e.stopImmediatePropagation();
        if (e.button === 0){
            this.dragging = true;
            const tooltip = document.body.querySelector(`tool-tip[uid="${this.dataset.tooltipUid}"]`);
            if (tooltip){
                tooltip.remove();
            }
            this.removeAttribute("sfx");
            this.removeAttribute("tooltip");
            this.classList.add("no-anim");
        }
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
        let out:TemplateResult;
        if (this.model.image){
            out = html`
                <img src="${this.model.image}" alt="${this.model.name} token" draggable="false">
            `;
        } else {
            let pawnType = "npc";
            if (this.model?.playerId != null){
                pawnType = "player";
            } else if (this.model?.monsterId != null){
                pawnType = "monster";
            }
            out = html`
                <div pawn="${pawnType}"></div>
            `;
        }
        return out;
    }

    private renderRings(){
        let x = 0;
        let y = 0;
        let w = 32;
        let h = 32;
        let delay = 0;
        return html`
            ${Object.keys(this.model.rings).map(key => {
                if (this.model.rings[key]){
                    x -= 3;
                    y -= 3;
                    w += 6;
                    h += 6;
                    delay += 0.25;
                    return html`
                        <div style="left:${x}px;top:${y}px;width:${w}px;height:${h}px;animation-delay:${delay}s;" class="ring" color="${key}"></div>
                    `;
                } else {
                    return "";
                }
            })}
        `;
    }

    override async render() {
        if (!this.model.hidden){
            this.style.visibility = "visible";
            this.style.opacity = "1";
            this.style.pointerEvents = "all";
        } else if (this.model.hidden && sessionStorage.getItem("role") === "gm") {
            this.style.visibility = "visible";
            this.style.opacity = "0.5";
            this.style.pointerEvents = "all";
        } else {
            this.style.visibility = "hidden";
            this.style.opacity = "0";
            this.style.pointerEvents = "none";
        }
        this.dataset.uid = this.model.uid;
        if (this.model?.playerId){
            this.dataset.playerUid = this.model.playerId;
        }
        if (!this.dragging){
            this.setAttribute("tooltip", this.model.name);
            this.setAttribute("sfx", "button");
        }
        this.style.transform = `translate(${this.model.x}px, ${this.model.y}px)`;
        this.localX = this.model.x;
        this.localY = this.model.y;
        this.className = "pawn";
        const view = html`
            ${this.renderRings()}
            ${this.renderPawn()}
        `;
        render(view, this);
    }
}
env.bind("pawn-component", Pawn);
