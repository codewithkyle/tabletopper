import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import env from "~brixi/controllers/env";

interface IPlayerMenu {};
export default class PlayerMenu extends SuperComponent<IPlayerMenu>{
    constructor(){
        super();
    }

    override async connected(){
        await env.css(["player-menu"]);
        this.render();
    }

    override render(): void {
        const view = html`Players go here`;
        render(view, this);
    }
}
env.bind("player-menu", PlayerMenu);