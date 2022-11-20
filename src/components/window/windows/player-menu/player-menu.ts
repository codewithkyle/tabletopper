import db from "@codewithkyle/jsql";
import { publish, subscribe } from "@codewithkyle/pubsub";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render, TemplateResult } from "lit-html";
import Input from "~brixi/components/inputs/input/input";
import env from "~brixi/controllers/env";
import Modal from "~components/modal/modal";
import cc from "~controllers/control-center";
import { send } from "~controllers/ws";

interface IPlayerMenu {};
export default class PlayerMenu extends SuperComponent<IPlayerMenu>{
    constructor(){
        super();
        subscribe("sync", this.syncInbox.bind(this));
    }

    private syncInbox(op){
        if (op.table === "players"){
            this.render();
        }
    }

    override async connected(){
        await env.css(["player-menu", "button"]);
        this.render();
    }

    private locatePawn:EventListener = (e:Event) => {
        const target = e.currentTarget as HTMLElement;
        publish("tabletop", {
            type: "locate:pawn",
            data: target.dataset.uid,
        });
    }

    private kickPlayer:EventListener = (e:Event) => {
        const target = e.currentTarget as HTMLElement;
        send("room:player:ban", target.dataset.uid);
    }

    private handleRenameSubmit:EventListener = (e:SubmitEvent) => {
        const form = e.currentTarget as HTMLFormElement;
        const input:Input = form.querySelector(".js-input");
        const name = input.getValue();
        const op = cc.set("players", form.dataset.playerUid, "name", name);
        cc.dispatch(op);
        const modal = form.closest("modal-component");
        modal.remove();
    }

    private renamePlayer:EventListener = (e:Event) => {
        const target = e.currentTarget as HTMLElement;
        const modal = new Modal("medium");
        modal.setHeading("Rename Player");
        modal.setView(html`
            <form @submit=${this.handleRenameSubmit.bind(this)} data-player-uid="${target.dataset.uid}">
                <div class="w-full px-1 pb-1 pt-0.5">
                    ${new Input({
                        name: "name",
                        value: target.dataset.name,
                    })}
                </div>
                <div class="actions">
                    <button class="bttn mr-1" kind="solid" color="white" type="button" @click=${modal.close.bind(modal)}>cancel</button>
                    <button class="bttn" kind="solid" color="success" type="submit">update</button>
                </div>
            </form>
        `);
        document.body.append(modal);
    }

    private renderPlayer(player):TemplateResult{
        return html`
            <div flex="row nowrap items-center justify-between" class="w-full player border-1 border-solid border-grey-300 radius-0.25 pl-0.75">
                <span title="${player.name}" class="cursor-default font-sm font-medium font-grey-700">${player.name}</span>
                <div class="h-full" flex="row nowrap items-center">
                    <button data-uid="${player.uid}" @click=${this.locatePawn} sfx="button" tooltip="Go to pawn" class="bttn border-r-1 border-l-1 border-l-solid border-l-grey-300 border-r-solid border-r-grey-300" kind="text" color="grey" icon="center">
                        <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-current-location" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                            <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                            <circle cx="12" cy="12" r="3"></circle>
                            <circle cx="12" cy="12" r="8"></circle>
                            <line x1="12" y1="2" x2="12" y2="4"></line>
                            <line x1="12" y1="20" x2="12" y2="22"></line>
                            <line x1="20" y1="12" x2="22" y2="12"></line>
                            <line x1="2" y1="12" x2="4" y2="12"></line>
                        </svg>
                    </button>
                    <button data-name="${player.name}" data-uid="${player.uid}" @click=${this.renamePlayer} sfx="button" tooltip="Rename player" class="bttn border-r-1 border-r-solid border-r-grey-300" kind="text" color="grey" icon="center">
                        <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-forms" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                            <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                            <path d="M12 3a3 3 0 0 0 -3 3v12a3 3 0 0 0 3 3"></path>
                            <path d="M6 3a3 3 0 0 1 3 3v12a3 3 0 0 1 -3 3"></path>
                            <path d="M13 7h7a1 1 0 0 1 1 1v8a1 1 0 0 1 -1 1h-7"></path>
                            <path d="M5 7h-1a1 1 0 0 0 -1 1v8a1 1 0 0 0 1 1h1"></path>
                            <path d="M17 12h.01"></path>
                            <path d="M13 12h.01"></path>
                        </svg> 
                    </button>
                    <button data-uid="${player.uid}" @click=${this.kickPlayer} sfx="button" tooltip="Kick ${player.name}" class="bttn" kind="text" color="grey" icon="center">
                        <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-user-off" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                            <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                            <path d="M14.274 10.291a4 4 0 1 0 -5.554 -5.58m-.548 3.453a4.01 4.01 0 0 0 2.62 2.65"></path>
                            <path d="M6 21v-2a4 4 0 0 1 4 -4h4a4 4 0 0 1 1.147 .167m2.685 2.681a4 4 0 0 1 .168 1.152v2"></path>
                            <line x1="3" y1="3" x2="21" y2="21"></line>
                        </svg>
                    </button>
                </div>
            </div>
        `;
    }

    override async render(){
        const players = await db.query("SELECT * FROM players WHERE room = $room AND active = 1", {
            room: sessionStorage.getItem("room"),
        });
        const view = html`
            <div class="w-full block p-0.5">
                ${players.map(this.renderPlayer.bind(this))}
            </div>
        `;
        render(view, this);
    }
}
env.bind("player-menu", PlayerMenu);
