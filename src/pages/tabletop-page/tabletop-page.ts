import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import env from "~brixi/controllers/env";

interface ITabletopPage {}
export default class TabletopPage extends SuperComponent<ITabletopPage>{
    constructor(tokens, params){
        console.log(tokens.CODE)
        super();
    }

    override async connected(){
        await env.css(["tabletop-page"]);
        this.render();
    }

    override render(): void {
        const view = html`Tabletop`;
        render(view, this);
    }
}
env.bind("tabletop-page", TabletopPage);