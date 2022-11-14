import SuperComponent from "@codewithkyle/supercomponent";
import env from "~brixi/controllers/env";
import { subscribe } from "@codewithkyle/pubsub";
import db from "@codewithkyle/jsql";
import { ConvertBase64ToBlob } from "utils/file";
import { setValueFromKeypath } from "utils/object";
import cc from "controllers/control-center";
import TabeltopComponent from "pages/tabletop-page/tabletop-component/tabletop-component";

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
}
export default class Pawn extends SuperComponent<IPawn>{
    public dragging: boolean;
    public localX: number;
    public localY: number;

    constructor(pawn){
        super();
        this.dragging = false;
        this.localX = 0;
        this.localY = 0;
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
        };
        subscribe("sync", this.syncInbox.bind(this));
        this.init();
    }
    
    async init() {
        await env.css(["pawn"]);
        window.addEventListener("mousemove", this.dragPawn, { passive: true, capture: true });
        this.addEventListener("mousedown", this.startDrag, { passive: false, capture: true });
        window.addEventListener("mouseup", this.stopDrag, { passive: true, capture: true });
        if (this.model.playerId){
            const player = (await db.query("SELECT * FROM players WHERE uid = $uid", { uid: this.model.playerId }))[0];
            if (player.token){
                await this.loadImage(player.token);
            }
        }
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
            const op1 = cc.set("pawns", this.model.uid, "x", this.localX);
            const op2 = cc.set("pawns", this.model.uid, "y", this.localY);
            const ops = cc.batch("pawns", this.model.uid, [op1, op2]);
            cc.perform(ops);
            cc.dispatch(ops);
        }
    }

    private startDrag:EventListener = (e:MouseEvent) => {
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
}
