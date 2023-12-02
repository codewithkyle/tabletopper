import SuperComponent from "@codewithkyle/supercomponent";
import {html, render} from "lit-html";
import env from "~brixi/controllers/env";
import {subscribe, unsubscribe} from "@codewithkyle/pubsub";
import {send} from "~controllers/ws";
import Sortable from "sortablejs";
import room from "room";

interface IInitiative {
    initiative: Array<any>,
    active_initiative: string|null,
}
export default class Initiative extends SuperComponent<IInitiative>{
    private ticket: string;
    public override model: IInitiative;

    constructor(initiative: Array<any>){
        super();
        this.model = {
            initiative: initiative,
            active_initiative: room.activeInitiative,
        };
        this.ticket = subscribe("socket", this.inbox.bind(this));
    }

    private inbox({ type, data }){
        switch (type){
            case "room:initiative:active":
                this.set({
                    active_initiative: data,
                });
                break;
            case "room:initiative:sync":
                this.set({
                    initiative: data,
                });
                break;
            default:
                break;
        }
    }

    override async connected() {
        await env.css(["initiative"]);
        this.render();
    }

    override disconnected(): void {
        unsubscribe(this.ticket);
    }

    public ping(currentPawn, nextIndex){
        if (nextIndex >= this.model.initiative.length){
                nextIndex = 0;
            }
        send("room:announce:initiative", {
            current: currentPawn,
            next: this.model.initiative[nextIndex],
        });
    }

    public drop(uid:string){
        const updated = this.get();
        for (let i = 0; i < updated.initiative.length; i++){
            if (updated.initiative[i].uid === uid){
                updated.initiative.splice(i, 1);
                break;
            }
        }
        send("room:initiative:sync", updated.initiative);
    }

    override render(): void{
        this.innerHTML = "";
        this.model.initiative.map((pawn, index) => {
            const el = new InitiativeItem(pawn, this.model.active_initiative, this.ping.bind(this), this.drop.bind(this));
            this.appendChild(el);
        });
        if (room.isGM){
            new Sortable(this, {
                sort: true,
                animation: 150,
                onEnd: (e) => {
                    if (e.oldIndex === e.newIndex) return;
                    const updated = this.get();
                    const temp = updated.initiative.splice(e.oldIndex, 1)[0];
                    updated.initiative.splice(e.newIndex, 0, temp);
                    for (let i = 0; i < updated.initiative.length; i++){
                        updated.initiative[i].index = i;
                    }
                    send("room:initiative:sync", updated.initiative);
                }
            });
        }
    }
}
env.bind("initiative-menu", Initiative);

interface IInitiativeItem{}
class InitiativeItem extends SuperComponent<IInitiativeItem>{
    private pawn: any;
    private active_initiative: string;
    private ping: Function;
    private drop: Function;

    constructor(pawn, active_initiative, ping, drop){
        super();
        this.pawn = pawn;
        this.active_initiative = active_initiative;
        this.ping = ping;
        this.drop = drop;
        this.render();
    }
    private renderButtons(){
        if (room.isGM){
            return html`
                <button
                    class="ml-0.5 bttn"
                    tooltip="Start turn"
                    kind="text"
                    icon="center"
                    color="grey"
                    @click=${()=>{
                        let nextIndex = this.pawn.index + 1;
                        this.ping(this.pawn, nextIndex);
                    }}
                >
                    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M15 11l4 4l-4 4m4 -4h-11a4 4 0 0 1 0 -8h1"></path></svg>
                </button>
                <button
                    class="bttn"
                    tooltip="Remove"
                    kind="text"
                    icon="center"
                    color="danger"
                    @click=${()=>{
                        this.drop(this.pawn.uid)
                    }}
                >
                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M4 7l16 0"></path><path d="M10 11l0 6"></path><path d="M14 11l0 6"></path><path d="M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12"></path><path d="M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3"></path></svg>
                </button>
            `;
        } else {
            return "";
        }
    }
    override render(): void {
        this.dataset.uid = this.pawn.uid;
        if (this.active_initiative === this.pawn.uid){
            this.classList.add("is-active");
        }
        const view = html`
            <span>${this.pawn.name}</span>
            ${this.renderButtons()}
        `;
        render(view, this);
    }
}
env.bind("initiative-item", InitiativeItem);
