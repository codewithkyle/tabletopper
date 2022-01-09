import SuperComponent from "@codewithkyle/supercomponent";
import { render, html } from "lit-html";
import env from "~brixi/controllers/env";
import { connected } from "~controllers/ws";

interface IHomepage{

}
export default class Homepage extends SuperComponent<IHomepage>{
    constructor(){
        super();
    }

    override async connected() {
        await env.css(["homepage"]);
        setInterval(this.render.bind(this), 1000);
    }

    override render(){
        const view = html`Homepage is ${connected}`;
        render(view, this);
    }
}