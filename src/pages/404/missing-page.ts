import SuperComponent from "@codewithkyle/supercomponent";
import { render, html } from "lit-html";
import { css } from "../../controllers/env";

interface IMissingPage{

}
export default class MissingPage extends SuperComponent<IMissingPage>{
    constructor(){
        super();
        css(["404"]).then(() => {
            this.render();
        });
    }

    override render(){
        const view = html`404 | Page not found.`;
        render(view, this);
    }
}