import SuperComponent from "@codewithkyle/supercomponent";
import { render, html } from "lit-html";
import env from "~brixi/controllers/env";

interface IHomepage{

}
export default class Homepage extends SuperComponent<IHomepage>{
    constructor(){
        super();
    }

    override async connected() {
        await env.css(["homepage"]);
        this.render();
    }

    override render(){
        const view = html`Homepage`;
        render(view, this);
    }
}