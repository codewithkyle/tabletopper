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

interface ITabletopPage {
}
export default class TabletopPage extends SuperComponent<ITabletopPage>{
    constructor(tokens, params){
        super();
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
        const lookupRequest = await fetch(`${url}/room/${sessionStorage.getItem("room")}`, {
            method: "HEAD"
        });
        if (lookupRequest.status === 200){
            await db.ingest(`${url}/room/${sessionStorage.getItem("room")}`, "ledger", "NDJSON");
            cc.runHistory();
        }
        this.setAttribute("state", "IDLING");
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