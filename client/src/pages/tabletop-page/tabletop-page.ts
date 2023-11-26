import db from "@codewithkyle/jsql";
import { navigateTo } from "@codewithkyle/router";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render, TemplateResult } from "lit-html";
import Spinner from "~brixi/components/progress/spinner/spinner";
import env from "~brixi/controllers/env";
import cc from "~controllers/control-center";
import { send } from "~controllers/ws";
import TabeltopComponent from "./tabletop-component/tabletop-component";
import Toolbar from "./tool-bar/tool-bar";
import SpotlightSearch from "components/spotlight-search/spotlight-search";
import Window from "~components/window/window";
import Spell from "~components/window/windows/spell/spell";
import MonsterEditor from "~components/window/windows/monster-editor/monster-editor";
import ContextMenu from  "~brixi/components/context-menu/context-menu";
import MonsterStatBlock from "~components/window/windows/monster-stat-block/monster-stat-block";
import NPCModal from "./npc-modal/npc-modal";
import type { Monster } from "~types/app";

interface ITabletopPage {
}
export default class TabletopPage extends SuperComponent<ITabletopPage>{
    private spotlightSearchEl: HTMLElement;
    private contextEl: HTMLElement;

    constructor(tokens, params){
        super();
        this.spotlightSearchEl = null;
        this.contextEl = null;
        const lastConfirmedCode = sessionStorage.getItem("room");
        const lastConfiredSocket = sessionStorage.getItem("lastSocketId");
        if (lastConfirmedCode === tokens.CODE.toUpperCase() && lastConfiredSocket !== sessionStorage.getItem("socketId")){
            send("core:sync", {
                room: lastConfirmedCode,
                prevId: lastConfiredSocket,
            });
        } else {
            navigateTo("/");
        }
    }

    override async connected(){
        await env.css(["tabletop-page"]);
        this.setAttribute("state", "SYNCING");
        this.render();
        await db.query("RESET ledger");
        await db.query("INSERT INTO games VALUES ($game)", {
            game: {
                uid: sessionStorage.getItem("room"),
                map: null,
                loaded_maps: [],
                players: [],
                initiative: [],
            }
        });
        // @ts-expect-error
        const { SERVER_URL } = await import("/config.js");
        const url = `${SERVER_URL.trim().replace(/\/$/, "")}`;
        await db.ingest(`${url}/room/${sessionStorage.getItem("room")}`, "ledger", "NDJSON");
        cc.runHistory();
        this.setAttribute("state", "IDLING");
        window.addEventListener("keydown", this.handleKeyboard);
        this.addEventListener("contextmenu", this.handleContextMenu);
    }

    private async handleSpotlightCallback(type: "spell"|"monster", index:string = null){
        let window;
        switch(type){
            case "spell":
                let title = "Create Spell";
                if (index !== null){
                    const spell = (await db.query("SELECT name FROM spells WHERE index = $index", { index: index, }))[0];
                    title = spell["name"];
                }
                window = new Window({
                    name: title,
                    view: new Spell(index),
                    width: 600,
                    height: 350,
                });
                break;
            case "monster":
                if (index === null){
                    window = new Window({
                        name: "Create Monster",
                        view: new MonsterEditor(index),
                        width: 600,
                        height: 350,
                    });
                } else {
                    const monster = (await db.query("SELECT name FROM monsters WHERE index = $index", { index: index, }))[0];
                    window = new Window({
                        name: monster["name"],
                        width: 600,
                        height: 350,
                        view: new MonsterStatBlock(index),
                    });
                }
                break;
            default:
                break;
        }
        if (window && !window.isConnected){
            document.body.appendChild(window);
        }
    }

    private handleContextMenu = (e) => {
        e.preventDefault();
        const x = e.clientX;
        const y = e.clientY;
        const items = [
            {
                label: "Ping",
                callback: ()=>{
                    const tabletop = document.body.querySelector("tabletop-component");
                    let diffX = (x - tabletop.x) / tabletop.zoom;
                    let diffY = (y - tabletop.y) / tabletop.zoom;
                    send("room:tabletop:ping", {
                        x: Math.round(diffX) - 16,
                        y: Math.round(diffY) - 16,
                        color: sessionStorage.getItem("color"),
                    });
                }
            }
        ];
        if (sessionStorage.getItem("role") === "gm"){
            items.push({
                label: "Spawn Monster",
                callback: ()=>{
                    const el = new SpotlightSearch(true, false, async (type:"monster"|"spell", index:string)=>{
                        if (index === null){
                            const windowEl = new Window({
                                name: "Create Monster",
                                view: new MonsterEditor(),
                                width: 600,
                                height: 350,
                            });
                        } else {
                            const tabletop = document.body.querySelector("tabletop-component");
                            let diffX = (x - tabletop.x) / tabletop.zoom;
                            let diffY = (y - tabletop.y) / tabletop.zoom;
                            const monster = (await db.query<Monster>("SELECT * FROM monsters WHERE index = $index", { index: index }))[0];
                            send("room:tabletop:spawn:monster", {
                                x: Math.round(diffX) - 16,
                                y: Math.round(diffY) - 16,
                                index: index,
                                name: monster.name,
                                ac: monster.ac,
                                hp: monster.hp,
                                size: monster.size.toLowerCase(),
                            });
                        }
                    });
                    document.body.appendChild(el);
                }
            });
            items.push({
                label: "Spawn NPC",
                callback: ()=>{
                    const modal = new NPCModal(x, y);
                    document.body.appendChild(modal);
                }
            });
        }
        new ContextMenu({
            x: x,
            y: y,
            items: items,
        });
    }

    private handleKeyboard = (e:KeyboardEvent) => {
        if (e?.["KeyStatus"]?.["RepeatCount"]){
            e["KeyStatus"]["RepeatCount"]++;
        } else {
            e["KeyStatus"] = {
                RepeatCount: 1,
            };
        }
        if (e instanceof KeyboardEvent){
            const key = e.key.toLowerCase();
            switch(key){
                case " ":
                    if (e.ctrlKey && (this.spotlightSearchEl === null || !this.spotlightSearchEl.isConnected) && e?.["KeyStatus"]?.["RepeatCount"] === 1){
                        this.spotlightSearchEl = new SpotlightSearch(sessionStorage.getItem("role") === "gm", true, this.handleSpotlightCallback.bind(this));
                        document.body.appendChild(this.spotlightSearchEl);
                    }
                    break;
                default:
                    break;
            }
        }
    }

    private renderSyncing():TemplateResult{
        return html`
            <div class="absolute center spinner">
                ${new Spinner({
                    size: 32,
                    class: "mb-0.75 mx-auto block",
                })}
                <span class="block text-center font-xs font-grey-700">Synchronizing game state...</span>
            </div>
        `;
    }

    override async render(){
        const view = html`
            ${new Toolbar()}
            ${this.renderSyncing()}
            ${new TabeltopComponent()}
        `;
        render(view, this);
    }
}
env.bind("tabletop-page", TabletopPage);

