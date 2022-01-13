import db from "@codewithkyle/jsql";
import { subscribe } from "@codewithkyle/pubsub";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render, TemplateResult } from "lit-html";
import Spinner from "~brixi/components/progress/spinner/spinner";
import env from "~brixi/controllers/env";
import { setValueFromKeypath } from "~utils/object";
import Toolbar from "./tool-bar/tool-bar";

interface ITabletopPage {
    map: string,
}
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
        this.model = {
            map: null,
        };
        subscribe("sync", this.syncInbox.bind(this));
    }

    private syncInbox(op){
        let updatedModel = this.get();
        switch (op.op){
            case "SET":
                if(op.table === "games"){
                    setValueFromKeypath(updatedModel, op.keypath, op.value);
                    this.set(updatedModel);
                }
                break;
            default:
                break;
        }
    }

    override async connected(){
        await env.css(["tabletop-page"]);
        this.render();
        await db.query("DELETE FROM games WHERE room = $room", { room: sessionStorage.getItem("room") });
        await db.query("INSERT INTO games VALUES ($game)", {
            game: {
                map: null,
                room: sessionStorage.getItem("room"),
            }
        });
        await db.query("RESET ledger");
        // TODO: get room data from server
        this.trigger("DONE");
    }

    private async renderIdling():Promise<TemplateResult>{
        let image = null;
        if (this.model.map){
            image = (await db.query("SELECT * FROM images WHERE uid = $uid", { uid : this.model.map }))[0];
        }
        return html`
            <div class="anchor">
                ${image ? html`<img class="center absolute" src="${image.data}" alt="${image.name}" draggable="false">` : ""}
            </div>
        `;
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

    override async render(){
        let content;
        switch (this.state){
            case "SYNCING":
                content = this.renderSyncing();
                break;
            case "IDLING":
                content = await this.renderIdling();
                break;
            default:
                break;
        }
        const view = html`
            ${new Toolbar()}
            ${content}
        `;
        render(view, this);
        setTimeout(()=>{
            const anchor = this.querySelector(".anchor") as HTMLElement;
            if (anchor){
                anchor.style.transform = `translate(${window.innerWidth * .5}px, ${(window.innerHeight - 28) * .5}px)`;
            }
        }, 150);
    }
}
env.bind("tabletop-page", TabletopPage);