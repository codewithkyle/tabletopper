import SuperComponent from "@codewithkyle/supercomponent";
import { render, html } from "lit-html";
import { css } from "../../controllers/env";

interface IHomepage{

}
export default class Homepage extends SuperComponent<IHomepage>{
    constructor(){
        super();
        css(["homepage"]).then(() => {
            this.render();
        });
    }

    override render(){
        const view = html`Homepage`;
        render(view, this);
    }
}