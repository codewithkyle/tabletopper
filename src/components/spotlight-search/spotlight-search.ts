import SuperComponent from "@codewithkyle/supercomponent";
import {UUID} from "@codewithkyle/uuid";
import {html, render} from "lit-html";
import env from "~brixi/controllers/env";
import db from "@codewithkyle/jsql";
import { Spell as ISpell, Monster } from "types/app";
import Spell from "components/window/windows/spell/spell"
import Window from "components/window/window";
import MonsterStatBlock from "~components/window/windows/monster-stat-block/monster-stat-block";
import Button from "~brixi/components/buttons/button/button";

interface ISpotlightSearch{
    results: Array<ISpell|Monster>,
    query: string,
}
export default class SpotlightSearch extends SuperComponent<ISpotlightSearch>{
    private lastQueueId: string;
    private includeMonsters: boolean;
    private includeSpells: boolean;
    private callback: Function|null;

    constructor(includeMonsters = true, includeSpells = true, callback = null){
        super();
        this.lastQueueId = null;
        this.includeMonsters = includeMonsters;
        this.includeSpells = includeSpells;
        this.callback = callback;
        this.model = {
            results: [],
            query: "",
        };
    }

    async connected(){
        await env.css(["spotlight-search"]);
        this.render();
    }

    private async search(value:string){
        const queueId = UUID();
        this.lastQueueId = queueId;
        let sql;
        if (this.includeMonsters && this.includeSpells){
            sql = "SELECT * FROM spells WHERE name LIKE $value UNION SELECT * FROM monsters WHERE name LIKE $value";
        } else if(this.includeSpells) {
            sql = "SELECT * FROM spells WHERE name LIKE $value";
        } else {
            sql = "SELECT * FROM monsters WHERE name LIKE $value";
        }
        const results = await db.query<ISpell|Monster>(sql, {
            value: value,
        });
        this.set({
            results: results,
            query: value,
        });
    }
    private handleInputDebounce = this.debounce(this.search.bind(this), 300);
    private handleInput = (e) => {
        const input = e.currentTarget as HTMLInputElement;
        this.handleInputDebounce(input.value.trim());
    }

    private clickBackdrop = () => {
        this.remove();
    }

    private clickResult(index) {
        const window = document.body.querySelector(`window-component[window="${this.model.results[index].index}"]`) || new Window({
            name: this.model.results[index].name,
            view: ("hp" in this.model.results[index]) ? new MonsterStatBlock(this.model.results[index].index) : new Spell(this.model.results[index].index),
            width: 600,
            height: 350
        });
        if (!window.isConnected){
            document.body.append(window);
        }
        this.remove();
    }

    private renderResults(){
        if (this.model.results.length){
            return html`
                <div class="results">
                    ${this.model.results.map((res, index) => {
                        return new Button({
                            label: res.name,
                            kind: "text",
                            color: "grey",
                            class: "w-full",
                            callback: ()=>{
                                this.clickResult(index);
                            }
                        });
                    })}
                </div>
                ${this.callback !== null ? new Button({
                    label: "Create Spell",
                    kind: "outline",
                    color: "grey",
                    class: "mx-0.5 mb-0.5",
                    css: "width: calc(100% - 1rem);",
                    callback: ()=>{
                        this.callback("spell");
                        this.remove();
                    },
                }) : ""}
                ${this.callback !== null ? new Button({
                    label: "Create Monster",
                    kind: "outline",
                    color: "grey",
                    class: "mx-0.5 mb-0.5",
                    css: "width: calc(100% - 1rem);",
                    callback: ()=>{
                        this.callback("monster");
                        this.remove();
                    },
                }) : ""}
            `;
        } else if (!this.model.results.length && this.model.query.length) {
            return html`
                <p class="text-center p-1 font-sm font-grey-400 border-t-solid border-t-1 border-t-grey-300">No${sessionStorage.getItem("role") === "gm" ? " Monsters or " : " "}Spells match '${this.model.query}'.</p>
                ${this.callback !== null ? new Button({
                    label: "Create Spell",
                    kind: "outline",
                    color: "grey",
                    class: "mx-0.5 mb-0.5",
                    css: "width: calc(100% - 1rem);",
                    callback: ()=>{
                        this.callback("spell");
                        this.remove();
                    },
                }) : ""}
                ${this.callback !== null ? new Button({
                    label: "Create Monster",
                    kind: "outline",
                    color: "grey",
                    class: "mx-0.5 mb-0.5",
                    css: "width: calc(100% - 1rem);",
                    callback: ()=>{
                        this.callback("monster");
                        this.remove();
                    },
                }) : ""}
            `;
        } else {
            return "";
        }
    }

    override render(){
        const view = html`
            <div class="backdrop" @click=${this.clickBackdrop}></div>
            <div class="modal">
                <input type="search" @input=${this.handleInput} placeholder="Search for spells or monsters..." autofocus>
                ${this.renderResults()}
            </div>
        `;
        render(view, this);
        setTimeout(()=>{
            const input = this.querySelector("input") as HTMLInputElement;
            input.focus();
        }, 100);
    }
}
env.bind("spotlight-search", SpotlightSearch);
