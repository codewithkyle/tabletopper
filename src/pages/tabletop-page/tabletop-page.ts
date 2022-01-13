import db from "@codewithkyle/jsql";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render, TemplateResult } from "lit-html";
import Spinner from "~brixi/components/progress/spinner/spinner";
import env from "~brixi/controllers/env";
import Toolbar from "./tool-bar/tool-bar";

interface ITabletopPage {}
export default class TabletopPage extends SuperComponent<ITabletopPage>{
    constructor(tokens, params){
        super();
        sessionStorage.setItem("room", tokens.CODE);
        this.state = "SYNCING";
        this.stateMachine = {
            SYNCING: {
                DONE: "IDLING",
            },
        };
    }

    override async connected(){
        await env.css(["tabletop-page"]);
        this.render();
        await db.query("RESET ledger");
        // TODO: get room data from server
        this.trigger("DONE");
    }

    private renderIdling():TemplateResult{
        return html``;
    }

    private renderSyncing():TemplateResult{
        return html`
            <div class="absolute center">
                ${new Spinner({
                    size: 32,
                    class: "mb-0.75 mx-auto block",
                })}
                <span class="block text-center font-xs font-grey-700">Synchronizing game state...</span>
            </div>
        `;
    }

    override render(): void {
        let content;
        switch (this.state){
            case "SYNCING":
                content = this.renderSyncing();
                break;
            case "IDLING":
                content = this.renderIdling();
                break;
            default:
                break;
        }
        const view = html`
            ${new Toolbar()}
            ${content}
        `;
        render(view, this);
    }
}
env.bind("tabletop-page", TabletopPage);