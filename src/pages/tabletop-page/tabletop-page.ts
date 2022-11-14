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

interface ITabletopPage {
}
export default class TabletopPage extends SuperComponent<ITabletopPage>{
    private spotlightSearchEl: HTMLElement;

    constructor(tokens, params){
        super();
        this.spotlightSearchEl = null;
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
                map: null,
                room: sessionStorage.getItem("room"),
            }
        });
        // @ts-expect-error
        const { SERVER_URL } = await import("/config.js");
        const url = `${SERVER_URL.trim().replace(/\/$/, "")}`;
        await db.ingest(`${url}/room/${sessionStorage.getItem("room")}`, "ledger", "NDJSON");
        cc.runHistory();
        this.setAttribute("state", "IDLING");
        window.addEventListener("keydown", this.handleKeyboard);
    }

    private create(type: "spell"|"monster"){
        let window;
        switch(type){
            case "spell":
                window = new Window({
                    name: "Create Spell",
                    view: new Spell(),
                    width: 600,
                    height: 350,
                });
                break;
            case "monster":
                window = new Window({
                    name: "Create Monster",
                    view: new MonsterEditor(),
                    width: 600,
                    height: 350,
                });
                break;
            default:
                break;
        }
        if (window && !window.isConnected){
            document.body.appendChild(window);
        }
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
                        this.spotlightSearchEl = new SpotlightSearch(sessionStorage.getItem("role") === "gm", true, this.create.bind(this));
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

