import SuperComponent from "@codewithkyle/supercomponent";
import { render, html } from "lit-html";
import env from "~brixi/controllers/env";

interface IMissingPage{

}
export default class MissingPage extends SuperComponent<IMissingPage>{
    constructor(){
        super();
    }

    override async connected() {
        await env.css(["404"]);
        this.render();
    }

    override render(){
        const view = html`404 | Page not found.`;
        render(view, this);
    }
}