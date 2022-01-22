import db from "@codewithkyle/jsql";
import { publish } from "@codewithkyle/pubsub";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render, TemplateResult } from "lit-html";
import env from "~brixi/controllers/env";

interface IPlayerMenu {};
export default class PlayerMenu extends SuperComponent<IPlayerMenu>{
    constructor(){
        super();
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

    private renderPlayer(player):TemplateResult{
        return html`
            <div flex="row nowrap items-center justify-between" class="w-full player border-b-1 border-b-solid border-b-grey-300 pl-0.5">
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
                    <button sfx="button" tooltip="Rename player" class="bttn border-r-1 border-r-solid border-r-grey-300" kind="text" color="grey" icon="center">
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
                    <button sfx="button" tooltip="Ban ${player.name}" class="bttn" kind="text" color="grey" icon="center">
                        <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-ban" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                            <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                            <circle cx="12" cy="12" r="9"></circle>
                            <line x1="5.7" y1="5.7" x2="18.3" y2="18.3"></line>
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
            ${players.map(this.renderPlayer.bind(this))}
        `;
        render(view, this);
    }
}
env.bind("player-menu", PlayerMenu);