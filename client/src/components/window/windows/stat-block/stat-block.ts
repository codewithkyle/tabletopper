import SuperComponent from "@codewithkyle/supercomponent";
import {html, render} from "lit-html";
import "~brixi/components/inputs/number-input/number-input";
import "~brixi/components/inputs/input/input";
import env from "~brixi/controllers/env";
import "~brixi/components/lightswitch/lightswitch";
import { send } from "~controllers/ws";

interface IStatBlock {
    hp: number,
    fullHP: number,
    ac: number,
    hidden: boolean,
    name: string,
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
export default class StatBlock extends SuperComponent<IStatBlock>{
    private pawnId: string;
    private monsterId: string;
    private type: "player" | "npc" | "monster";

    constructor(pawnId:string, type:"player" | "npc" | "monster" = "monster", rings, hp = 0, fullHP = 0, ac = 0, hidden = true, name = "", monsterId = ""){
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
            rings: rings,
        };
    }

    override async connected(){
        await env.css(["stat-block"]);
        this.render();
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
        if (hp > this.model.fullHP) {
            this.classList.add("overhealed");
            this.classList.remove("bloody");
        } else if (hp <= this.model.fullHP && hp > this.model.fullHP / 2){
            this.classList.remove("overhealed");
            this.classList.remove("bloody");
        } else if (hp <= this.model.fullHP / 2) {
            this.classList.add("bloody");
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

    private handleRingChange:EventListener = (e:Event) => {
        const input = e.currentTarget as HTMLInputElement;
        send("room:tabletop:pawn:status", {
            pawnId: this.pawnId,
            type: input.value,
            checked: input.checked,
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

    private renderRings(){
        return html`
            ${Object.keys(this.model.rings).map(key => {
                return html`
                    <input type="checkbox" id="${this.pawnId}-ring-${key}" name="${this.pawnId}-ring-${key}" ?checked=${this.model.rings[key]} value="${key}" @change=${this.handleRingChange}>
                    <label for="${this.pawnId}-ring-${key}" color="${key}"></label>
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
            ${ this.type !== "player" ? html`
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
                        @blur=${this.updateAC.bind(this)}
                    ></number-input-component>
                </div>
            ` : "" }
            <div class="w-full rings" flex="row wrap items-center">
                ${this.renderRings()}
            </div>
        `;
        render(view, this);
    }
}
env.bind("stat-block", StatBlock);
