import db from "@codewithkyle/jsql";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render, TemplateResult } from "lit-html";
import Input from "~brixi/components/inputs/input/input";
import env from "~brixi/controllers/env";
import notifications from "~brixi/controllers/notifications";
import Window from "~components/window/window";
import MonsterStatBlock from "../monster-stat-block/monster-stat-block";

interface IMonsterManual {
    query: string,
    limit: number,
}
export default class MonsterManual extends SuperComponent<IMonsterManual>{
    constructor(){
        super();
        this.model = {
            query: "",
            limit: 10,
        };
    }

    override async connected(){
        await env.css(["monster-manual"]);
        this.render();
    }

    private showMore:EventListener = (e:Event) => {
        this.set({
            limit: this.model.limit + 10,
        });
    }

    private search(value){
        this.set({
            query: value,
        });
    }
    private debounceInput = this.debounce(this.search.bind(this), 300);

    private deleteMonster:EventListener = async (e:Event) => {
        const target = e.currentTarget as HTMLElement;
        await db.query("DELETE FROM monsters WHERE index = $index", { index: target.dataset.index });
        this.render();
        notifications.snackbar(`${target.dataset.name} has been deleted.`);
    }

    private openMonster:EventListener = (e:Event) => {
        const target = e.currentTarget as HTMLElement;
        const window = document.body.querySelector(`window-component[window="${target.dataset.index}"]`) || new Window({
            name: target.dataset.name,
            view: new MonsterStatBlock(target.dataset.index),
            handle: target.dataset.index,
            width: 450,
            height: 600,
        });
        if (!window.isConnected){
            document.body.append(window);
        }
    }

    private spawnMonster:EventListener = (e:Event) => {
        const target = e.currentTarget as HTMLElement;
    }

    private editMonster:EventListener = (e:Event) => {
        const target = e.currentTarget as HTMLElement;
    }

    private renderMonster(monster): TemplateResult{
        return html`
            <div class="monster" flex="row nowrap items-center justify-between">
                <button sfx="button" class="bttn" color="grey" kind="text" title="${monster.name}" @click=${this.openMonster} data-name="${monster.name}" data-index="${monster.index}">${monster.name}</button>
                <div class="row nowrap items-center" style="font-size:0;">
                    <button @click=${this.spawnMonster} data-index="${monster.index}" tooltip="Spawn" class="h-full bttn" color="grey" kind="text" icon="center" sfx="button">
                        <svg aria-hidden="true" focusable="false" data-prefix="far" data-icon="street-view" class="svg-inline--fa fa-street-view fa-w-15" role="img" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512"><path fill="currentColor" d="M168 338.59V384c0 30.88 25.12 56 56 56h64c30.88 0 56-25.12 56-56v-45.41c18.91-9 32-28.3 32-50.59v-72c0-28.78-17.09-53.48-41.54-65C345.43 135.4 352 116.49 352 96c0-52.94-43.06-96-96-96s-96 43.06-96 96c0 20.49 6.57 39.4 17.55 55-24.46 11.52-41.55 36.22-41.55 65v72c0 22.3 13.09 41.59 32 50.59zM256 48c26.51 0 48 21.49 48 48s-21.49 48-48 48-48-21.49-48-48 21.49-48 48-48zm-72 168c0-13.23 10.78-24 24-24h96c13.22 0 24 10.77 24 24v72c0 4.41-3.59 8-8 8h-24v88c0 4.41-3.59 8-8 8h-64c-4.41 0-8-3.59-8-8v-88h-24c-4.41 0-8-3.59-8-8v-72zm209.61 119.14c-4.9 7.65-10.55 14.83-17.61 20.69v14.49c53.18 10.14 88 26.81 88 45.69 0 30.93-93.12 56-208 56S48 446.93 48 416c0-18.88 34.82-35.54 88-45.69v-14.49c-7.06-5.86-12.71-13.03-17.61-20.69C47.28 352.19 0 382 0 416c0 53.02 114.62 96 256 96s256-42.98 256-96c0-34-47.28-63.81-118.39-80.86z"></path></svg>
                    </button>
                    <button @click=${this.editMonster} data-index="${monster.index}" tooltip="Edit" class="h-full bttn" color="grey" kind="text" icon="center" sfx="button">
                        <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-edit" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                            <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                            <path d="M9 7h-3a2 2 0 0 0 -2 2v9a2 2 0 0 0 2 2h9a2 2 0 0 0 2 -2v-3"></path>
                            <path d="M9 15h3l8.5 -8.5a1.5 1.5 0 0 0 -3 -3l-8.5 8.5v3"></path>
                            <line x1="16" y1="5" x2="19" y2="8"></line>
                        </svg>
                    </button>
                    <button @click=${this.deleteMonster} data-name="${monster.name}" data-index="${monster.index}" tooltip="Delete" class="h-full bttn" color="grey" kind="text" icon="center" sfx="button">
                        <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-trash" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                            <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                            <line x1="4" y1="7" x2="20" y2="7"></line>
                            <line x1="10" y1="11" x2="10" y2="17"></line>
                            <line x1="14" y1="11" x2="14" y2="17"></line>
                            <path d="M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12"></path>
                            <path d="M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3"></path>
                        </svg>
                    </button>
                </div>
            </div>
        `;
    }

    override async render(){
        let monsters;
        if (this.model.query?.length){
            monsters = await db.query("SELECT * FROM monsters WHERE name LIKE $query LIMIT $limit", {
                query: this.model.query,
                limit: this.model.limit,
            });
        } else {
            monsters = await db.query("SELECT * FROM monsters LIMIT $limit", {
                limit: this.model.limit,
            });
        }
        const container = this.querySelector(".monsters");
        if (container){
            render(html`
                ${monsters.map(this.renderMonster.bind(this))}
            `, container);
        } else {
            const view = html`
                ${new Input({
                    name: "monsterSearch",
                    value: this.model.query,
                    placeholder: "Search monsters...",
                    callback: this.debounceInput.bind(this),
                    class: "mb-0.5",
                })}
                <div class="radius-0.25 border-solid border-1 border-grey-300 no-scroll">
                    <div class="monsters">
                        ${monsters.map(this.renderMonster.bind(this))}
                    </div>
                    <button @click=${this.showMore} class="w-full bttn" color="grey" kind="text" sfx="button">show more monsters</button>
                </div>
            `;
            render(view, this);
        }
    }
}
env.bind("monster-manual", MonsterManual);