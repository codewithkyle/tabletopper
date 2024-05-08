import { publish, subscribe } from "@codewithkyle/pubsub";
import SuperComponent from "@codewithkyle/supercomponent";
import env from "~brixi/controllers/env";
import room from "room";
import { html, render } from "lit-html";
import { send } from "~controllers/ws";
import Pawn, { IPawn } from "~components/pawn/pawn";
import { Condition } from "~types/app";
import Sortable from "sortablejs";
import ContextMenu from "~brixi/components/context-menu/context-menu";

interface IInitiativeTrackerComponent {
}
export default class InitiativeTrackerComponent extends SuperComponent<InitiativeTrackerComponent>{
    private initiative: Array<IPawn>;
    private active: number;

    constructor(){
        super();
        this.initiative = [];
        this.active = null;
        subscribe("socket", this.inbox.bind(this));
    }

    private inbox({ type, data }) {
        switch (type) {
            case "room:initiative:sync":
                this.initiative = data;
                this.render();
                break;
            case "room:initiative:active":
                this.active = data;
                this.render();
                break;
            case "room:initiative:clear":
                this.active = null;
                this.initiative = [];
                this.render();
                break;
            case "room:tabletop:pawn:status:add":
                for (const pawn of this.initiative){
                    if (data.pawnId === pawn.uid){
                        pawn.conditions[data.condition.uid] = data.condition;
                        this.render();
                        break;
                    }
                }
                break;
            case "room:tabletop:pawn:status:remove":
                for (const pawn of this.initiative){
                    if (data.pawnId === pawn.uid){
                        delete pawn.conditions[data.uid];
                        this.render();
                        break;
                    }
                }
                break;
            default:
                break;
        }
    }

    override async connected() {
        await env.css(["initiative-tracker"]);
        window.addEventListener("initiative:sync", this.sync.bind(this));
        window.addEventListener("initiative:clear", this.clear.bind(this));
        this.render();
    }

    public clear() {
        this.initiative = [];
        this.active = null;
        send("room:initiative:clear");
    }

    public sync() {
        const pawns: Pawn[] = Array.from(document.body.querySelectorAll("pawn-component"));
        const claimedNames = [];
        if (this.initiative.length){
            for (let i = 0; i < this.initiative.length; i++){
                claimedNames.push(this.initiative[i].name);
            }
        }
        for (let i = 0; i < pawns.length; i++){
            const pawnType = pawns[i].getAttribute("pawn");
            if (pawnType !== "dead" && !claimedNames.includes(pawns[i].dataset.name)){
                claimedNames.push(pawns[i].dataset.name);
                this.initiative.push({
                    ...pawns[i].model,
                });
            }
        }
        send("room:initiative:sync", this.initiative);
    }

    private onCardClick:EventListener = (e:Event) => {
        if (!room.isGM) return;
        const target = e.currentTarget as HTMLElement;
        this.active = +target.dataset.index;
        send("room:initiative:active", this.active);
        this.render();
    }

    private onContextMenu:EventListener = (e:MouseEvent) => {
        e.preventDefault();
        const x = e.clientX;
        const y = e.clientY;
        document.body.querySelectorAll("context-menu").forEach((el) => {
            el.remove();
        });
        if (!room.isGM) return;
        const target = e.currentTarget as HTMLElement;
        const index = +target.dataset.index;
        const name = target.dataset.name;
        new ContextMenu({
            x: x,
            y: y,
            items: [
                {
                    label: `Remove ${name}`,
                    callback: () => {
                        if (index === this.active) {
                            send("room:initiative:active", null);
                        }
                        this.initiative.splice(index, 1);
                        send("room:initiative:sync", this.initiative);
                        this.render();
                    }
                }
            ],
        });
    }

    private renderConditions(conditions){
        return html`
            <initiative-card-conditions>
                ${Object.keys(conditions).map(key => {
                    const condition = conditions[key] as Condition;
                    return html`
                        <initiative-card-condition>
                            <div class="ring" color="${condition.color}"></div>
                            <span>${condition.name}</span>
                        </initiative-card-condition>
                    `;
                })}
            </initiative-card-conditions>
        `;
    }

    private renderCard(pawn:IPawn, index:number, container:HTMLElement) {
        let renderConditions = false;
        if (index === this.active){
            if (pawn.type !== "monster"){
                renderConditions = true;
            } else if (document.body.querySelectorAll(`pawn-component[data-name="${pawn.name}"]`).length === 1) {
                renderConditions = true;
            }
        }
        const view = html`
            <span class="static-name">${pawn.name}</span>
            ${pawn.image?.length ? html`
                <img src="https://tabletopper.nyc3.cdn.digitaloceanspaces.com/${pawn.image}" alt="${pawn.name} token" draggable="false">
            ` : ""}
            <span class="name">${pawn.name}</span>
            ${renderConditions ? this.renderConditions(pawn.conditions) : ""}
        `;
        render(view, container);
    }

    override render(): void {
        if (!this.initiative.length) {
            this.style.display = "none";
            return;
        } else {
            this.style.display = "block";
        }
        this.innerHTML = "";
        this.initiative.map((pawn, index) => {
            const container:HTMLElement = this.querySelector(`[data-pawn-id="${pawn.uid}"]`) || document.createElement("initiative-card");
            if (!container.isConnected){
                container.dataset.pawnId = pawn.uid;
                container.dataset.name = pawn.name;
                container.setAttribute("type", pawn.type);
                container.setAttribute("tooltip", pawn.name);
                container.setAttribute("tabindex", "0");
                container.addEventListener("click", this.onCardClick);
                container.addEventListener("contextmenu", this.onContextMenu);
                this.appendChild(container);
            }
            container.setAttribute("active", `${this.active === index}`);
            container.dataset.index = index.toString();
            this.renderCard(pawn, index, container);
        });
        if (room.isGM) {
            new Sortable(this, {
                sort: true,
                animation: 150,
                onEnd: (e) => {
                    if (e.oldIndex === e.newIndex) return;
                    const temp = this.initiative.splice(e.oldIndex, 1)[0];
                    if (temp === null) return;
                    this.initiative.splice(e.newIndex, 0, temp);
                    send("room:initiative:sync", this.initiative);
                }
            });
        }
    }
}
env.bind("initiative-tracker", InitiativeTrackerComponent);
