import db from "@codewithkyle/jsql";
import SuperComponent from "@codewithkyle/supercomponent";
import {html, render} from "lit-html";
import NumberInput from "~brixi/components/inputs/number-input/number-input";
import Input from "~brixi/components/inputs/input/input";
import env from "~brixi/controllers/env";
import cc from "controllers/control-center";
import Lightswitch from "~brixi/components/lightswitch/lightswitch";
import Button from "~brixi/components/buttons/button/button";
import {publish} from "@codewithkyle/pubsub";
import MonsterStatBlock from "../monster-stat-block/monster-stat-block";
import Window from "components/window/window";

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
    private entityId: string|null;
    private type: "player" | "npc" | "monster";

    constructor(pawnId:string, id:string = null, type:"player" | "npc" | "monster" = "monster"){
        super();
        this.pawnId = pawnId;
        this.entityId = id;
        this.type = type;
        this.model = {
            hp: 0,
            fullHP: 0,
            ac: 0,
            hidden: true,
            name: "",
            rings: {
                blue: false,
                green: false,
                orange: false,
                pink: false,
                purple: false,
                red: false,
                white: false,
                yellow: false,
            },
        };
    }

    override async connected(){
        await env.css(["stat-block"]);
        const pawn = (await db.query("SELECT * FROM pawns WHERE uid = $uid", { uid: this.pawnId }))[0];
        this.set({
            hp: pawn.hp,
            ac: pawn.ac,
            fullHP: pawn.fullHP,
            hidden: pawn.hidden,
            name: pawn.name,
            rings: pawn.rings,
        });
        this.render();
    }

    private updateHP(value){
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
        const op = cc.set("pawns", this.pawnId, "hp", hp);
        cc.dispatch(op);
    }

    private updateAC(value){
        value = parseInt(value);
        this.set({ac: value}, true);
        const op = cc.set("pawns", this.pawnId, "ac", value);
        cc.dispatch(op);
    }

    private locate(){
        publish("tabletop", {
            type: "locate:pawn",
            data: this.pawnId,
        });
    }

    private handleRingChange:EventListener = (e:Event) => {
        const input = e.currentTarget as HTMLInputElement;
        console.log(input);
        const op = cc.set("pawns", this.pawnId, `rings.${input.value}`, input.checked);
        cc.dispatch(op);
    }

    private openMonsterManual(){
        const windowEl = document.body.querySelector(`window-component[window="${this.entityId}"]`) || new Window({
            name: this.model.name,
            view: new MonsterStatBlock(this.entityId),
            handle: this.entityId,
            width: 450,
            height: 600,
        });
        if (!windowEl.isConnected){
            document.body.append(windowEl);
        }
    }

    private renderDeleteButton(){
        if (this.type !== "player"){
            return new Button({
                icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><line x1="4" y1="7" x2="20" y2="7"></line><line x1="10" y1="11" x2="10" y2="17"></line><line x1="14" y1="11" x2="14" y2="17"></line><path d="M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12"></path><path d="M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3"></path></svg>`,
                iconPosition: "center",
                kind: "text",
                color: "danger",
                callback: ()=>{
                    const op = cc.delete("pawns", this.pawnId);
                    cc.dispatch(op);
                    this.closest("window-component").close();
                },
                tooltip: "Delete pawn",
                size: "slim",
            });
        } else {
            return "";
        }
    }

    private renderBookButton(){
        if (this.type === "monster"){
            return new Button({
                icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M19 4v16h-12a2 2 0 0 1 -2 -2v-12a2 2 0 0 1 2 -2h12z"></path><path d="M19 16h-12a2 2 0 0 0 -2 2"></path><path d="M9 8h6"></path></svg>`,
                iconPosition: "center",
                kind: "text",
                color: "grey",
                callback: this.openMonsterManual.bind(this),
                tooltip: "Open monster manual",
                size: "slim",
            });
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
            <div class="w-full mb-0.5" grid="columns 2 gap-1">
                ${new Input({
                    name: `${this.pawnId}-hp`,
                    label: "Hit Points",
                    value: this.model.hp.toString(),
                    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M19.5 12.572l-7.5 7.428l-7.5 -7.428m0 0a5 5 0 1 1 7.5 -6.566a5 5 0 1 1 7.5 6.572"></path></svg>`,
                    callbacks: {
                        onBlur: this.updateHP.bind(this),
                    },
                })}
                ${new NumberInput({
                    name: `${this.pawnId}-ac`,
                    label: "Armour Class",
                    value: this.model.ac,
                    callbacks: {
                        onBlur: this.updateAC.bind(this),
                    },
                    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M12 3a12 12 0 0 0 8.5 3a12 12 0 0 1 -8.5 15a12 12 0 0 1 -8.5 -15a12 12 0 0 0 8.5 -3"></path></svg>`,
                })}
            </div>
            <div class="w-full mb-0.5" flex="items-center justify-between row nowrap">
                ${new Lightswitch({
                    name: `${this.pawnId}-hidden`,
                    disabledLabel: `Visible`,
                    enabledLabel: "Hidden",
                    enabled: this.model.hidden === true,
                    callback: (enabled) => {
                        const op = cc.set("pawns", this.pawnId, "hidden", enabled);
                        cc.dispatch(op);
                    },
                    class: "mt-0.25",
                    value: "hidden",
                })}
                <div flex="items-center row nowrap">
                    ${this.renderBookButton()}
                    ${new Button({
                        icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><circle cx="12" cy="12" r="3"></circle><circle cx="12" cy="12" r="8"></circle><line x1="12" y1="2" x2="12" y2="4"></line><line x1="12" y1="20" x2="12" y2="22"></line><line x1="20" y1="12" x2="22" y2="12"></line><line x1="2" y1="12" x2="4" y2="12"></line></svg>`,
                        iconPosition: "center",
                        kind: "text",
                        color: "grey",
                        callback: this.locate.bind(this),
                        tooltip: "Locate pawn",
                        size: "slim",
                    })}
                    ${this.renderDeleteButton()}
                </div>
            </div>
            <div class="w-full rings" flex="row wrap items-center">
                ${this.renderRings()}
            </div>
        `;
        render(view, this);
    }
}
env.bind("stat-block", StatBlock);