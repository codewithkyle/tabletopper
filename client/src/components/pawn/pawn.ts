import SuperComponent from "@codewithkyle/supercomponent";
import env from "~brixi/controllers/env";
import { subscribe, unsubscribe } from "@codewithkyle/pubsub";
import TabeltopComponent from "pages/tabletop-page/tabletop-component/tabletop-component";
import {html, render, TemplateResult} from "lit-html";
import Window from "~components/window/window";
import StatBlock from "components/window/windows/stat-block/stat-block";
import {Size} from "~types/app";
import room from "room";
import { send } from "~controllers/ws";

interface IPawn{
    uid: string,
    x: number,
    y: number,
    image: string|null;
    hidden: boolean;
    token: string|null;
    name: string;
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
    hp?: number,
    fullHP?: number,
    size: Size,
    type: "player"|"monster"|"npc",
}
export default class Pawn extends SuperComponent<IPawn>{
    public dragging: boolean;
    public localX: number;
    public localY: number;
    public timeToSplatter: number;
    private ticket: string;
    private gridSize: number;

    constructor(pawn:IPawn){
        super();
        this.timeToSplatter = 0;
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
            rings: pawn.rings,
            hp: pawn?.hp ?? null,
            fullHP: pawn?.fullHP ?? null,
            size: pawn?.size ?? "medium",
            type: pawn.type,
        };
        this.ticket = subscribe("socket", this.inbox.bind(this));
    }
    
    override async connected() {
        await env.css(["pawn"]);
        window.addEventListener("mousemove", this.dragPawn, { passive: true, capture: true });
        this.addEventListener("mousedown", this.startDrag, { passive: false, capture: true });
        window.addEventListener("mouseup", this.stopDrag, { passive: true, capture: true });
        window.addEventListener("touchmove", this.dragPawn, { passive: true, capture: true });
        this.addEventListener("touchstart", this.startDrag, { passive: false, capture: true });
        window.addEventListener("touchend", this.stopDrag, { passive: true, capture: true });
        this.addEventListener("contextmenu", this.contextMenu, { passive: false, capture: true });
        this.gridSize = 32;
        this.render();
    }

    override disconnected(){
        unsubscribe(this.ticket);
    }

    private inbox({ type, data }){
        switch(type){
            case "room:tabletop:map:update":
                this.gridSize = data.cellSize;
                this.render();
                break;
            case "room:tabletop:clear":
                this.remove();
                break;
            case "room:tabletop:pawn:move":
                if (data.uid === this.model.uid){
                    this.model.x = data.x;
                    this.model.y = data.y;
                    this.render();
                }
                break;
            default:
                break;
        }
    }

    private contextMenu:EventListener = (e:MouseEvent) => {
        e.preventDefault();
        e.stopImmediatePropagation();
        if (this.model.type === "monster" && room.isGM){
            const x = e.clientX;
            const y = e.clientY;
            const windowEl = new Window({
                name: `${this.model.name} (${this.model.type})`,
                width: 400,
                height: 200,
                view: new StatBlock(this.model.uid, this.model.uid, this.model.type),
                handle: "stat-block",
            });
            if (!windowEl.isConnected){
                document.body.appendChild(windowEl);
            }
        } else if (this.model.type === "player"){
            const x = e.clientX;
            const y = e.clientY;
            const windowEl = new Window({
                name: `${this.model.name}`,
                width: 400,
                height: 200,
                view: new StatBlock(this.model.uid, "player"),
                handle: "stat-block",
            });
            if (!windowEl.isConnected){
                document.body.appendChild(windowEl);
            }
        }
    }

    private resetTooltip(){
        if (this.model.hp !== null && this.model.fullHP !== null){
            if (this.model.hp === 0){
                this.setAttribute("bleeding", "false");
                this.setAttribute("tooltip", `${this.model.name} (dead)`);
            } else if (this.model.hp <= this.model.fullHP * 0.5){
                this.setAttribute("bleeding", "true");
                this.setAttribute("tooltip", `${this.model.name} (bloody)`);
            } else {
                this.setAttribute("bleeding", "false");
                this.setAttribute("tooltip", this.model.name);
            }
        } else {
            this.setAttribute("tooltip", this.model.name);
        }
    }

    private stopDrag:EventListener = (e:DragEvent) => {
        this.classList.remove("no-anim");
        this.resetTooltip();
        this.setAttribute("sfx", "button");
        if (this.dragging){
            console.log("sending move", this.model.uid, this.localX, this.localY);
            send("room:tabletop:pawn:move", {
                uid: this.model.uid,
                x: this.localX,
                y: this.localY,
            });
        }
        this.dragging = false;
    }

    private startDrag:EventListener = (e:MouseEvent|TouchEvent) => {
        e.preventDefault();
        e.stopImmediatePropagation();
        if (e instanceof MouseEvent && e.button === 0 || window.TouchEvent && e instanceof TouchEvent){
            this.dragging = true;
            const tooltip = document.body.querySelector(`tool-tip[uid="${this.dataset.tooltipUid}"]`);
            if (tooltip){
                tooltip.remove();
            }
            this.removeAttribute("sfx");
            this.removeAttribute("tooltip");
            this.classList.add("no-anim");

            document.body.querySelectorAll("pawn-component").forEach((el:HTMLElement) => {
                el.style.zIndex = "10";
            });
            this.style.zIndex = "100";
        }
    }

    private dragPawn:EventListener = (e:MouseEvent|TouchEvent) => {
        if (this.dragging){
            const tabletop = this.parentElement as TabeltopComponent;
            let x;
            let y;
            if (window.TouchEvent && e instanceof TouchEvent){
                x = e.touches[0].clientX;
                y = e.touches[0].clientY;
            } else {
                // @ts-ignore
                x = e.clientX;
                // @ts-ignore
                y = e.clientY;
            }
            let diffX = (tabletop.x - x);
            let diffY = (tabletop.y - y);
            diffX += this.gridSize * this.getSizeMultiplier() * 0.5 * tabletop.zoom;
            diffY += this.gridSize * this.getSizeMultiplier() * 0.5 * tabletop.zoom;
            diffX /= tabletop.zoom;
            diffY /= tabletop.zoom;
            this.localX = -diffX;
            this.localY = -diffY;
            this.style.transform = `translate(${this.localX}px, ${this.localY}px)`;
        }
    }

    private getSizeMultiplier():number{
        let multi = 1;
        switch (this.model.size){
            case "tiny":
                multi = 0.5;
                break;
            case "large":
                multi = 2;
                break;
            case "huge":
                multi = 3;
                break;
            case "gargantuan":
                multi = 4;
                break;
            default:
                multi = 1;
                break;
        }
        return multi;
    }

    private setSize(){
        let multi = this.getSizeMultiplier();
        this.style.width = `${this.gridSize * multi}px`;
        this.style.height = `${this.gridSize * multi}px`;
        this.dataset.w = `${this.gridSize * multi}`;
        this.dataset.h = `${this.gridSize * multi}`;
    }

    private renderPawn():TemplateResult|string{
        let out:TemplateResult|string;
        if ("hp" in this.model && this.model.hp === 0){
            return html`
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                   <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                   <path d="M12 4c4.418 0 8 3.358 8 7.5c0 1.901 -.755 3.637 -1.999 4.96l-.001 2.54a1 1 0 0 1 -1 1h-10a1 1 0 0 1 -1 -1v-2.54c-1.245 -1.322 -2 -3.058 -2 -4.96c0 -4.142 3.582 -7.5 8 -7.5z"></path>
                   <path d="M10 17v3"></path>
                   <path d="M14 17v3"></path>
                   <circle cx="9" cy="11" r="1"></circle>
                   <circle cx="15" cy="11" r="1"></circle>
                </svg>
            `;
        }
        if (this.model.image){
            out = html`
                <img src="${this.model.image}" alt="${this.model.name} token" draggable="false">
            `;
        } else {
            out = "";
        }
        return out;
    }

    private renderRings(){
        const m = this.getSizeMultiplier();
        let x = 0;
        let y = 0;
        let w = this.gridSize * m;
        let h = this.gridSize * m;
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
        this.setSize();
        if (!this.model.hidden){
            this.style.visibility = "visible";
            this.style.opacity = "1";
            this.style.pointerEvents = "all";
            this.setAttribute("ghost", "false");
        } else if (this.model.hidden && room.isGM){
            this.style.visibility = "visible";
            this.style.opacity = "0.5";
            this.style.pointerEvents = "all";
            this.setAttribute("ghost", "true");
        } else {
            this.style.visibility = "hidden";
            this.style.opacity = "0";
            this.style.pointerEvents = "none";
            this.setAttribute("ghost", "false");
        }
        this.dataset.uid = this.model.uid;
        if (!this.dragging){
            this.setAttribute("sfx", "button");
        }
        this.resetTooltip();
        this.style.transform = `translate(${this.model.x}px, ${this.model.y}px)`;
        this.localX = this.model.x;
        this.localY = this.model.y;
        this.dataset.x = `${this.localX}`;
        this.dataset.y = `${this.localY}`;
        this.className = "pawn";
        this.dataset.name = this.model.name;
        let pawnType = "npc";
        if ("hp" in this.model && this.model.hp === 0){
            pawnType = "dead";
        } else if (this.model.type === "player"){
            pawnType = "player";
        } else if (this.model.type === "monster"){
            pawnType = "monster";
        }
        this.setAttribute("pawn", pawnType);
        const view = html`
            ${this.renderRings()}
            ${this.renderPawn()}
        `;
        render(view, this);
    }
}
env.bind("pawn-component", Pawn);
