import SuperComponent from "@codewithkyle/supercomponent";
import {UUID} from "@codewithkyle/uuid";
import {html, render} from "lit-html";
import env from "~brixi/controllers/env";
import db from "@codewithkyle/jsql";
import { Spell as ISpell, Monster } from "types/app";
import Spell from "components/window/windows/spell/spell"
import Window from "components/window/window";
import MonsterStatBlock from "~components/window/windows/monster-stat-block/monster-stat-block";

interface ISpotlightSearch{
    results: Array<ISpell|Monster>,
    query: string,
}
export default class SpotlightSearch extends SuperComponent<ISpotlightSearch>{
    private lastQueueId: string;

    constructor(){
        super();
        this.lastQueueId = null;
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
        const results = await db.query<ISpell|Monster>("SELECT * FROM spells WHERE name LIKE $value UNION SELECT * FROM monsters WHERE name LIKE $value", {
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

    private clickResult = (e) => {
        const target = e.currentTarget as HTMLButtonElement;
        const index = parseInt(target.dataset.index);
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
                        return html`
                            <button data-index="${index}" @click=${this.clickResult}>${res.name}</button>
                        `;
                    })}
                </div>
            `;
        } else if (!this.model.results.length && this.model.query.length) {
            return html`
                <p class="text-center p-1 font-sm font-grey-400">No Monsters or Spells match '${this.model.query}'.</p>
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
