import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import env from "~brixi/controllers/env";

interface IPlayerTokenPicker {}
export default class PlayerTokenPicker extends SuperComponent<IPlayerTokenPicker>{
    constructor(){
        super();
    }

    override async connected(){
        await env.css(["player-token-picker"]);
        this.render();
    }

    override render(): void {
        const view = html``;
        render(view, this);
    }
}
env.bind("player-token-picker", PlayerTokenPicker);