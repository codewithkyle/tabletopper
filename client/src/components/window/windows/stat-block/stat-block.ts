import SuperComponent from "@codewithkyle/supercomponent";
import {html, render} from "lit-html";
import "~brixi/components/inputs/number-input/number-input";
import "~brixi/components/inputs/input/input";
import env from "~brixi/controllers/env";
import "~brixi/components/lightswitch/lightswitch";
import { send } from "~controllers/ws";
import { subscribe, unsubscribe } from "@codewithkyle/pubsub";
import type { Condition } from "~types/app";
import { UUID } from "@codewithkyle/uuid";

interface IStatBlock {
    hp: number,
    fullHP: number,
    ac: number,
    hidden: boolean,
    name: string,
    conditions: {
        [uid:string]: Condition,
    },
}
export default class StatBlock extends SuperComponent<IStatBlock>{
    private pawnId: string;
    private monsterId: string;
    private type: "player" | "npc" | "monster";
    private ticket: string;

    constructor(pawnId:string, type:"player" | "npc" | "monster" = "monster", conditions = {}, hp = 0, fullHP = 0, ac = 0, hidden = true, name = "", monsterId = ""){
        super();
        this.pawnId = pawnId;
        this.monsterId = monsterId;
        this.type = type;
        this.model = {
            hp: hp,
            fullHP: fullHP,
            ac: ac,
            hidden: hidden,
            name: name,
            conditions: conditions,
        };
        this.ticket = subscribe("socket", this.inbox.bind(this));
    }

    override async connected(){
        await env.css(["stat-block"]);
        this.render();
    }

    override disconnected(): void {
        unsubscribe("socket", this.ticket);
        window.removeEventListener("add-condition", this.addCondition);
    }

    private inbox({ type, data }){
        switch(type){
            case "room:tabletop:pawn:health":
                if (data.pawnId === this.pawnId){
                    this.model.hp = data.hp;
                    this.render();
                }
                break;
            case "room:tabletop:pawn:ac":
                if (data.pawnId === this.pawnId){
                    this.model.ac = data.ac;
                    this.render();
                }
                break;
            case "room:tabletop:pawn:visibility":
                if (data.pawnId === this.pawnId){
                    this.model.hidden = data.hidden;
                    this.render();
                }
                break;
            case "room:tabletop:pawn:status:add":
                if (data.pawnId === this.pawnId){
                    this.model.conditions[data.condition.uid] = data.condition;
                    this.render();
                }
                break;
            case "room:tabletop:pawn:status:remove":
                if (data.pawnId === this.pawnId){
                    delete this.model.conditions[data.uid];
                    this.render();
                }
                break;
            default:
                break;
        }
    }

    private updateHP(e:CustomEvent){
        const { name, value } = e.detail;
        const values = value
                    .trim()
                    .replace(/[^0-9\-\+]/g, "") // only allow digets, +, and -
                    .replace(/\+{1,}/g, " + ") // force space round +
                    .replace(/\-{1,}/g, " - ") // force space around -
                    .replace(/\s+/g, " ") // convert 1+ spaces into 1 space
                    .split(" ");
        let hp = 0;
        let lastSeenOperator = "+";
        for (let i = 0; i < values.length; i++){
            switch (values[i]){
                case "+":
                    lastSeenOperator = "+";
                    break;
                case "-":
                    lastSeenOperator = "-";
                    break;
                default:
                    switch (lastSeenOperator){
                        case "+":
                            hp += +values[i];
                            break;
                        case "-":
                            hp -= +values[i];
                            break;
                    }
                    break;
            }
        }
        this.set({hp: hp});
        send("room:tabletop:pawn:health", {
            pawnId: this.pawnId,
            hp: hp,
        });
    }

    private updateAC(e:CustomEvent){
        let { name, value } = e.detail;
        value = parseInt(value);
        this.set({ac: value}, true);
        send("room:tabletop:pawn:ac", {
            pawnId: this.pawnId,
            ac: value,
        });
    }

    private changeVisibility(e:CustomEvent){
        const { name, value, enabled } = e.detail;
        send("room:tabletop:pawn:visibility", {
            pawnId: this.pawnId,
            hidden: !enabled,
        });
    }

    private openMonsterManual(){
        window.dispatchEvent(new CustomEvent("window:monsters:open", { 
            detail: {
                uid: this.monsterId,
                name: this.model.name
            } 
        }));
    }

    private addCondition:EventListener = (e:CustomEvent) => {
        window.removeEventListener("add-condition", this.addCondition);
        send("room:tabletop:pawn:status:add", {
            pawnId: this.pawnId,
            name: e.detail.name,
            duration: e.detail.duration,
            color: e.detail.color,
            uid: UUID(),
        });
    }

    private onAddConditionClick:EventListener = (e) => {
        window.dispatchEvent(new CustomEvent("show-conditions-menu"));
        window.addEventListener("add-condition", this.addCondition);
    }

    private onRemoveConditionClick:EventListener = (e) => {
        // @ts-ignore
        const uid = e.currentTarget.dataset.uid;
        send("room:tabletop:pawn:status:remove", { pawnId: this.pawnId, uid: uid });
    }

    private renderDeleteButton(){
        if (this.type !== "player"){
            return html`
                <button
                    class="bttn"
                    kind="text"
                    color="danger"
                    icon="center"
                    tooltip="Delete pawn"
                    size="slim"
                    @click=${()=>{
                        send("room:tabletop:pawn:delete", this.pawnId);
                        // @ts-ignore
                        this.closest("window-component").close();
                    }}
                >
                    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><line x1="4" y1="7" x2="20" y2="7"></line><line x1="10" y1="11" x2="10" y2="17"></line><line x1="14" y1="11" x2="14" y2="17"></line><path d="M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12"></path><path d="M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3"></path></svg>
                </button>
            `;
        } else {
            return "";
        }
    }

    private renderBookButton(){
        if (this.type === "monster"){
            return html`
                <button
                    class="bttn"
                    kind="text"
                    color="grey"
                    icon="center"
                    tooltip="Open monster manual"
                    size="slim"
                    @click=${this.openMonsterManual.bind(this)}
                >
                    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M19 4v16h-12a2 2 0 0 1 -2 -2v-12a2 2 0 0 1 2 -2h12z"></path><path d="M19 16h-12a2 2 0 0 0 -2 2"></path><path d="M9 8h6"></path></svg>
                </button>
            `;
        } else {
            return "";
        }
    }

    private renderConditions(){
        return html`
            ${Object.keys(this.model.conditions).map(key => {
                const condition = this.model.conditions[key];
                return html`
                    <condition-badge color="${condition.color}" @click=${this.onRemoveConditionClick} data-uid="${condition.uid}">
                        <span>${condition.name}</span>
                        <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                            <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                            <path d="M18 6l-12 12"></path>
                            <path d="M6 6l12 12"></path>
                        </svg>
                    </condition-badge>
                `;
            })}
        `;
    }

    override render(): void {
        const view = html`
            <div class="w-full mb-1" flex="items-center justify-between row nowrap">
                <lightswitch-component
                    data-name="${this.pawnId}-hidden"
                    data-enabled-label="Visible"
                    data-disabled-label="Hidden"
                    data-enabled="${this.model.hidden === false}"
                    data-value="hidden"
                    class="mt-0.25"
                    @change=${this.changeVisibility.bind(this)}
                ></lightswitch-component>
                <div flex="items-center row nowrap">
                    ${this.renderBookButton()}
                    ${this.renderDeleteButton()}
                </div>
            </div>
            <div class="w-full mb-1" grid="columns 2 gap-1">
                <input-component
                    data-name="${this.pawnId}-hp"
                    data-label="Hit Points"
                    data-value="${this.model.hp}"
                    data-icon='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M19.5 12.572l-7.5 7.428l-7.5 -7.428m0 0a5 5 0 1 1 7.5 -6.566a5 5 0 1 1 7.5 6.572"></path></svg>'
                    @blur=${this.updateHP.bind(this)}
                ></input-component> 
                <number-input-component
                    data-name="${this.pawnId}-ac"
                    data-label="Armour Class"
                    data-value="${this.model.ac}"
                    data-icon='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M12 3a12 12 0 0 0 8.5 3a12 12 0 0 1 -8.5 15a12 12 0 0 1 -8.5 -15a12 12 0 0 0 8.5 -3"></path></svg>'
                    @change=${this.updateAC.bind(this)}
                ></number-input-component>
            </div>
            <div class="w-full conditions" flex="row wrap items-center">
                <button
                    class="bttn mr-0.25 mb-0.5"
                    kind="text"
                    color="grey"
                    icon="center"
                    tooltip="Add condition"
                    @click=${this.onAddConditionClick}
                >
                    <svg  xmlns="http://www.w3.org/2000/svg"  width="24"  height="24"  viewBox="0 0 24 24"  fill="none"  stroke="currentColor"  stroke-width="2"  stroke-linecap="round"  stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M12 12m-9 0a9 9 0 1 0 18 0a9 9 0 1 0 -18 0" /><path d="M13 12h5" /><path d="M13 15h4" /><path d="M13 18h1" /><path d="M13 9h4" /><path d="M13 6h1" /></svg>
                </button>
                ${this.renderConditions()}
            </div>
        `;
        render(view, this);
        if (this.model.hp > this.model.fullHP) {
            this.classList.add("overhealed");
            this.classList.remove("bloody");
        } else if (this.model.hp <= this.model.fullHP && this.model.hp > this.model.fullHP / 2){
            this.classList.remove("overhealed");
            this.classList.remove("bloody");
        } else if (this.model.hp <= this.model.fullHP / 2) {
            this.classList.add("bloody");
        }
    }
}
env.bind("stat-block", StatBlock);
