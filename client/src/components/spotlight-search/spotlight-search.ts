import SuperComponent from "@codewithkyle/supercomponent";
import {html, render} from "lit-html";
import env from "~brixi/controllers/env";
import { Spell as ISpell, Monster } from "types/app";

interface ISpotlightSearch{
    results: Array<ISpell|Monster>,
    query: string,
}
export default class SpotlightSearch extends SuperComponent<ISpotlightSearch>{
    private callback: Function|null;

    constructor(callback = null){
        super();
        this.lastQueueId = null;
        this.callback = callback;
        this.model = {
            results: [],
            query: "",
        };
    }

    async connected(){
        await env.css(["spotlight-search"]);
        this.render();
    }

    private async search(value:string){
    }
    private handleInputDebounce = this.debounce(this.search.bind(this), 300);
    private handleInput = (e) => {
        const input = e.currentTarget as HTMLInputElement;
        this.handleInputDebounce(input.value.trim());
    }

    private clickBackdrop = () => {
        this.remove();
    }

    override render(){
        const view = html`
            <div class="backdrop" @click=${this.clickBackdrop}></div>
            <div class="modal">
                <input type="search" @input=${this.handleInput} placeholder="Search for spells or monsters..." autofocus>
                ${this.renderResults()}
            </div>
        `;
        render(view, this);
        setTimeout(()=>{
            const input = this.querySelector("input") as HTMLInputElement;
            input.focus();
        }, 100);
    }
}
env.bind("spotlight-search", SpotlightSearch);
